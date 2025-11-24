package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AnalysisOrchestrator manages multiple analysis pipelines and queues using channel-based coordination
type AnalysisOrchestrator struct {
	pipelines     []*AnalysisPipeline // Protected by coordinationChan - only accessed in coordination goroutine
	requestQueue  chan *AnalysisRequest
	responseQueue chan *AnalysisResponse
	workers       []*AnalysisWorker
	config        *OrchestratorConfig
	metrics       *OrchestratorMetrics

	// Load balancing state
	roundRobinCounter int64 // Separate counter for round-robin pipeline selection

	// Channel-based coordination (Phase 1 pattern)
	coordinationChan   chan coordinationMsg
	pipelineSelectChan chan pipelineSelectRequest // New: for race-free pipeline selection
	statusChan         chan workerStatus
	shutdownChan       chan struct{}
	doneChan           chan struct{}

	wg sync.WaitGroup
}

// pipelineSelectRequest represents a request for pipeline selection
type pipelineSelectRequest struct {
	responseChan chan *AnalysisPipeline
}

// coordinationMsg represents coordination messages between orchestrator and workers
type coordinationMsg struct {
	msgType   coordinationMsgType
	workerID  int
	requestID string
	payload   any
	respChan  chan any // For synchronous responses
}

type coordinationMsgType int

const (
	msgWorkerStarted coordinationMsgType = iota
	msgWorkerStopped
	msgRequestStarted
	msgRequestCompleted
	msgRequestFailed
	msgGetMetrics
	msgGetWorkerStatus
	msgShutdownRequest
)

// workerStatus represents worker status updates sent via channels
type workerStatus struct {
	workerID        int
	isActive        bool
	currentRequest  string
	requestsHandled int64
	startTime       time.Time
}

// OrchestratorConfig configures the analysis orchestrator
type OrchestratorConfig struct {
	// Worker configuration
	MaxWorkers        int           `json:"max_workers" yaml:"max_workers"`
	WorkerIdleTimeout time.Duration `json:"worker_idle_timeout" yaml:"worker_idle_timeout"`

	// Queue configuration
	QueueSize         int           `json:"queue_size" yaml:"queue_size"`
	ProcessingTimeout time.Duration `json:"processing_timeout" yaml:"processing_timeout"`
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// Priority handling
	EnablePriority    bool `json:"enable_priority" yaml:"enable_priority"`
	HighPriorityQueue bool `json:"high_priority_queue" yaml:"high_priority_queue"`

	// Load balancing
	LoadBalancing string `json:"load_balancing" yaml:"load_balancing"` // "round_robin", "least_busy", "random"

	// Monitoring
	MetricsInterval     time.Duration `json:"metrics_interval" yaml:"metrics_interval"`
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
}

// OrchestratorMetrics tracks orchestrator performance
type OrchestratorMetrics struct {
	TotalRequests         int64         `json:"total_requests"`
	ActiveRequests        int64         `json:"active_requests"`
	CompletedRequests     int64         `json:"completed_requests"`
	FailedRequests        int64         `json:"failed_requests"`
	QueueDepth            int           `json:"queue_depth"`
	AverageQueueTime      time.Duration `json:"average_queue_time"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	WorkerUtilization     float64       `json:"worker_utilization"`
	ThroughputPerSecond   float64       `json:"throughput_per_second"`
	ErrorRate             float64       `json:"error_rate"`
	mu                    sync.RWMutex
}

// AnalysisWorker handles analysis requests from the queue using channel coordination
type AnalysisWorker struct {
	ID              int
	pipeline        *AnalysisPipeline
	orchestrator    *AnalysisOrchestrator
	startTime       time.Time
	requestsHandled int64
	// Removed mutex - using channel coordination instead
}

// WorkerStatus represents the current status of a worker
type WorkerStatus struct {
	ID              int           `json:"id"`
	IsActive        bool          `json:"is_active"`
	CurrentRequest  string        `json:"current_request,omitempty"`
	RequestsHandled int64         `json:"requests_handled"`
	Uptime          time.Duration `json:"uptime"`
	LastActivity    time.Time     `json:"last_activity,omitempty"`
	Health          string        `json:"health,omitempty"`
}

// NewAnalysisOrchestrator creates a new orchestrator with proper channel initialization
func NewAnalysisOrchestrator(config *OrchestratorConfig) *AnalysisOrchestrator {
	if config == nil {
		config = &OrchestratorConfig{
			MaxWorkers:          4,
			QueueSize:           100,
			MaxRetries:          3,
			RetryDelay:          time.Second,
			WorkerIdleTimeout:   10 * time.Second,
			ProcessingTimeout:   5 * time.Second,
			LoadBalancing:       "round_robin",
			MetricsInterval:     time.Minute,
			HealthCheckInterval: 30 * time.Second,
		}
	}

	return &AnalysisOrchestrator{
		pipelines:     make([]*AnalysisPipeline, 0),
		requestQueue:  make(chan *AnalysisRequest, config.QueueSize),
		responseQueue: make(chan *AnalysisResponse, config.QueueSize),
		workers:       make([]*AnalysisWorker, 0, config.MaxWorkers),
		config:        config,
		metrics:       &OrchestratorMetrics{},

		coordinationChan:   make(chan coordinationMsg, 10),
		pipelineSelectChan: make(chan pipelineSelectRequest, 10), // Buffer for pipeline selection requests
		statusChan:         make(chan workerStatus, config.MaxWorkers),
		shutdownChan:       make(chan struct{}),
		doneChan:           make(chan struct{}),
	}
}

// DefaultOrchestratorConfig returns default orchestrator configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxWorkers:          10,
		WorkerIdleTimeout:   30 * time.Second,
		QueueSize:           1000,
		ProcessingTimeout:   10 * time.Minute,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		EnablePriority:      true,
		HighPriorityQueue:   false,
		LoadBalancing:       "least_busy",
		MetricsInterval:     30 * time.Second,
		HealthCheckInterval: 60 * time.Second,
	}
}

// NewOrchestratorMetrics creates new orchestrator metrics
func NewOrchestratorMetrics() *OrchestratorMetrics {
	return &OrchestratorMetrics{}
}

// AddPipeline adds an analysis pipeline to the orchestrator using channel coordination
func (o *AnalysisOrchestrator) AddPipeline(pipeline *AnalysisPipeline) {
	// Try channel coordination first, fallback to direct addition
	respChan := make(chan any)
	select {
	case o.coordinationChan <- coordinationMsg{
		msgType:  msgWorkerStarted, // Reuse for pipeline addition
		payload:  pipeline,
		respChan: respChan,
	}:
		// Wait for confirmation with timeout
		select {
		case <-respChan:
			slog.Info("added analysis pipeline via coordination", slog.Int("total_pipelines", len(o.pipelines)))
		case <-time.After(100 * time.Millisecond):
			// Timeout, fallback to direct addition
			o.pipelines = append(o.pipelines, pipeline)
			slog.Info("added analysis pipeline (coordination timeout)", slog.Int("total_pipelines", len(o.pipelines)))
		}
	default:
		// Coordination not available, add directly
		o.pipelines = append(o.pipelines, pipeline)
		slog.Info("added analysis pipeline (direct)", slog.Int("total_pipelines", len(o.pipelines)))
	}
}

// Start starts the orchestrator and its workers using channel-based coordination
func (o *AnalysisOrchestrator) Start(ctx context.Context) error {
	if len(o.pipelines) == 0 {
		return fmt.Errorf("no analysis pipelines configured")
	}

	// Start coordination manager first (Phase 1 pattern)
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.runCoordination(ctx)
	}()

	// Give coordination time to start
	time.Sleep(10 * time.Millisecond)

	// Start workers
	for i := 0; i < o.config.MaxWorkers; i++ {
		worker := o.createWorker(i)
		o.workers = append(o.workers, worker)

		o.wg.Add(1)
		go func(w *AnalysisWorker) {
			defer o.wg.Done()
			w.run(ctx)
		}(worker)
	}

	// DISABLED: Start response handler - This conflicts with analysis system GetResponse()
	// The analysis system needs to consume directly from responseQueue
	// o.wg.Add(1)
	// go func() {
	// 	defer o.wg.Done()
	// 	o.handleResponses(ctx)
	// }()

	// Start metrics collection
	if o.config.MetricsInterval > 0 {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			o.collectMetrics(ctx)
		}()
	}

	// Start health checks
	if o.config.HealthCheckInterval > 0 {
		o.wg.Add(1)
		go func() {
			defer o.wg.Done()
			o.performHealthChecks(ctx)
		}()
	}

	// Signal that we're running via coordination
	select {
	case o.coordinationChan <- coordinationMsg{
		msgType: msgWorkerStarted,
		payload: true, // isRunning = true
	}:
	default:
		// Coordination might be busy, continue anyway
	}

	slog.Info("Analysis orchestrator started",
		slog.Int("total_workers", len(o.workers)),
		slog.Int("total_pipelines", len(o.pipelines)),
		slog.Int("queue_size", o.config.QueueSize))

	return nil
}

// runCoordination manages orchestrator state using channel-based coordination (Phase 1 pattern)
func (o *AnalysisOrchestrator) runCoordination(ctx context.Context) {
	isRunning := false
	workerStatuses := make(map[int]workerStatus)

	defer func() {
		// Close channels when coordination ends
		close(o.doneChan)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case msg := <-o.coordinationChan:
			switch msg.msgType {
			case msgWorkerStarted:
				if pipeline, ok := msg.payload.(*AnalysisPipeline); ok {
					// Handle pipeline addition - check for duplicates to prevent double-addition
					pipelineExists := false
					for _, existingPipeline := range o.pipelines {
						if existingPipeline == pipeline {
							pipelineExists = true
							break
						}
					}

					if !pipelineExists {
						o.pipelines = append(o.pipelines, pipeline)
						slog.Info("added analysis pipeline via coordination (deferred)", slog.Int("total_pipelines", len(o.pipelines)))
					} else {
						slog.Debug("pipeline already exists, skipping duplicate addition", slog.Int("total_pipelines", len(o.pipelines)))
					}

					if msg.respChan != nil {
						select {
						case msg.respChan <- true:
						default:
						}
					}
				} else if running, ok := msg.payload.(bool); ok {
					// Handle running state change
					isRunning = running
				}

			case msgGetMetrics:
				if msg.respChan != nil {
					select {
					case msg.respChan <- isRunning:
					default:
					}
				}

			case msgGetWorkerStatus:
				if msg.respChan != nil {
					statuses := make([]WorkerStatus, 0, len(workerStatuses))
					for _, status := range workerStatuses {
						statuses = append(statuses, WorkerStatus{
							ID:              status.workerID,
							IsActive:        status.isActive,
							CurrentRequest:  status.currentRequest,
							RequestsHandled: status.requestsHandled,
							Uptime:          time.Since(status.startTime),
						})
					}
					select {
					case msg.respChan <- statuses:
					default:
					}
				}

			case msgRequestStarted:
				o.updateMetricsOnSubmit()

			case msgRequestCompleted:
				o.updateMetricsOnComplete()

			case msgRequestFailed:
				o.metrics.mu.Lock()
				o.metrics.FailedRequests++
				if o.metrics.ActiveRequests > 0 {
					o.metrics.ActiveRequests--
				}
				o.metrics.mu.Unlock()
			}
		case request := <-o.pipelineSelectChan:
			// Handle pipeline selection request (race-free)
			var selectedPipeline *AnalysisPipeline
			if len(o.pipelines) > 0 {
				switch o.config.LoadBalancing {
				case "least_busy":
					// Select least busy pipeline - for now, just return first
					selectedPipeline = o.pipelines[0]
				case "round_robin":
					// Round-robin selection with dedicated counter
					idx := int(o.roundRobinCounter) % len(o.pipelines)
					o.roundRobinCounter++ // Increment for next selection
					selectedPipeline = o.pipelines[idx]
				default:
					selectedPipeline = o.pipelines[0]
				}
			}
			// Send response back
			select {
			case request.responseChan <- selectedPipeline:
			default:
			}
		case status := <-o.statusChan:
			// Update worker status
			workerStatuses[status.workerID] = status
		}
	}
}

// Stop gracefully stops the orchestrator using channel-based coordination
func (o *AnalysisOrchestrator) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "stopping analysis orchestrator")

	// Signal shutdown
	close(o.shutdownChan)

	// Close request queue to stop accepting new requests
	close(o.requestQueue)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.InfoContext(ctx, "all workers stopped gracefully")
	case <-ctx.Done():
		slog.WarnContext(ctx, "orchestrator shutdown timeout")
		return ctx.Err()
	}

	// Close response queue
	close(o.responseQueue)

	return nil
}

// SubmitRequest submits an analysis request to the orchestrator using channel coordination
func (o *AnalysisOrchestrator) SubmitRequest(request *AnalysisRequest) error {
	// Check if running via coordination channel with timeout
	respChan := make(chan any)
	select {
	case o.coordinationChan <- coordinationMsg{
		msgType:  msgGetMetrics,
		respChan: respChan,
	}:
		select {
		case resp := <-respChan:
			if isRunning, ok := resp.(bool); !ok || !isRunning {
				return fmt.Errorf("orchestrator is not running")
			}
		case <-time.After(50 * time.Millisecond):
			// Timeout, assume not running
			return fmt.Errorf("orchestrator coordination not available")
		}
	default:
		return fmt.Errorf("orchestrator coordination not available")
	}

	// Add request ID if not present
	if request.RequestID == "" {
		request.RequestID = fmt.Sprintf("req_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
	}

	select {
	case o.requestQueue <- request:
		// Signal request started via coordination (non-blocking)
		select {
		case o.coordinationChan <- coordinationMsg{
			msgType:   msgRequestStarted,
			requestID: request.RequestID,
		}:
		default:
			// Coordination busy, continue anyway
		}
		slog.Debug("Request submitted to queue", slog.String("request_id", request.RequestID))
		return nil
	default:
		return fmt.Errorf("request queue is full")
	}
}

// GetResponse gets a response from the response queue
func (o *AnalysisOrchestrator) GetResponse() (*AnalysisResponse, bool) {
	slog.Debug("GetResponse: Waiting for response from response queue")
	response, ok := <-o.responseQueue
	if ok && response != nil {
		slog.Info("GetResponse: Received response from response queue", "request_id", response.RequestID, "has_error", response.Error != nil)
	} else {
		slog.Warn("GetResponse: Response queue closed or received nil response", "ok", ok, "response_nil", response == nil)
	}
	return response, ok
}

// GetMetrics returns current orchestrator metrics
func (o *AnalysisOrchestrator) GetMetrics() *OrchestratorMetrics {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	// Create a copy
	return &OrchestratorMetrics{
		TotalRequests:         o.metrics.TotalRequests,
		ActiveRequests:        o.metrics.ActiveRequests,
		CompletedRequests:     o.metrics.CompletedRequests,
		FailedRequests:        o.metrics.FailedRequests,
		QueueDepth:            len(o.requestQueue),
		AverageQueueTime:      o.metrics.AverageQueueTime,
		AverageProcessingTime: o.metrics.AverageProcessingTime,
		WorkerUtilization:     o.calculateWorkerUtilization(),
		ThroughputPerSecond:   o.metrics.ThroughputPerSecond,
		ErrorRate:             o.metrics.ErrorRate,
	}
}

// GetWorkerStatus returns the status of all workers using channel coordination
func (o *AnalysisOrchestrator) GetWorkerStatus() []WorkerStatus {
	respChan := make(chan any)
	select {
	case o.coordinationChan <- coordinationMsg{
		msgType:  msgGetWorkerStatus,
		respChan: respChan,
	}:
		select {
		case resp := <-respChan:
			if statuses, ok := resp.([]WorkerStatus); ok {
				return statuses
			}
		case <-time.After(100 * time.Millisecond):
			// Timeout, return empty status
		}
	default:
		// Coordination not available
	}

	return []WorkerStatus{}
}

// createWorker creates a new analysis worker
func (o *AnalysisOrchestrator) createWorker(id int) *AnalysisWorker {
	// Select pipeline using load balancing strategy
	pipeline := o.selectPipeline()

	slog.Info("Created worker with pipeline assignment",
		"worker_id", id,
		"pipeline_ptr", fmt.Sprintf("%p", pipeline),
		"total_pipelines", len(o.pipelines))

	return &AnalysisWorker{
		ID:           id,
		pipeline:     pipeline,
		orchestrator: o,
		startTime:    time.Now(),
	}
}

// selectPipeline selects a pipeline based on the load balancing strategy
func (o *AnalysisOrchestrator) selectPipeline() *AnalysisPipeline {
	// Create a request for pipeline selection
	responseChan := make(chan *AnalysisPipeline, 1)
	request := pipelineSelectRequest{
		responseChan: responseChan,
	}

	// Send request to coordination goroutine
	select {
	case o.pipelineSelectChan <- request:
		// Wait for response
		select {
		case pipeline := <-responseChan:
			return pipeline
		case <-o.shutdownChan:
			return nil
		}
	case <-o.shutdownChan:
		return nil
	}
}

// run starts the worker's main processing loop
func (w *AnalysisWorker) run(ctx context.Context) {
	slog.InfoContext(ctx, "worker started")
	defer slog.InfoContext(ctx, "worker stopeed")

	// Report initial status as idle (non-blocking)
	select {
	case w.orchestrator.statusChan <- workerStatus{
		workerID:        w.ID,
		isActive:        false,
		currentRequest:  "",
		requestsHandled: w.requestsHandled,
		startTime:       w.startTime,
	}:
	default:
		// Status channel busy, continue anyway
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.orchestrator.shutdownChan:
			return
		case request, ok := <-w.orchestrator.requestQueue:
			if !ok {
				return // Queue closed
			}
			w.processRequest(ctx, request)
		}
	}
}

// processRequest processes a single analysis request using channel coordination
func (w *AnalysisWorker) processRequest(ctx context.Context, request *AnalysisRequest) {
	// Update status via channel coordination (non-blocking)
	select {
	case w.orchestrator.statusChan <- workerStatus{
		workerID:        w.ID,
		isActive:        true,
		currentRequest:  request.RequestID,
		requestsHandled: w.requestsHandled,
		startTime:       w.startTime,
	}:
	default:
		// Status channel busy, continue anyway
	}

	defer func() {
		w.requestsHandled++
		// Update status to inactive via channel coordination (non-blocking)
		select {
		case w.orchestrator.statusChan <- workerStatus{
			workerID:        w.ID,
			isActive:        false,
			currentRequest:  "",
			requestsHandled: w.requestsHandled,
			startTime:       w.startTime,
		}:
		default:
			// Status channel busy, continue anyway
		}
		// Signal request completed via coordination (non-blocking)
		select {
		case w.orchestrator.coordinationChan <- coordinationMsg{
			msgType:   msgRequestCompleted,
			workerID:  w.ID,
			requestID: request.RequestID,
		}:
		default:
			// Coordination busy, continue anyway
		}
	}()

	slog.DebugContext(ctx, "Processing request", "request_id", request.RequestID)

	slog.InfoContext(ctx, "Worker calling pipeline AnalyzeDocument",
		"request_id", request.RequestID,
		"worker_id", w.ID,
		"pipeline_ptr", fmt.Sprintf("%p", w.pipeline))

	// Process with timeout
	requestCtx, cancel := context.WithTimeout(ctx, w.orchestrator.config.ProcessingTimeout)
	defer cancel()

	response := w.pipeline.AnalyzeDocument(requestCtx, request)
	slog.InfoContext(ctx, "Worker received response from pipeline", "request_id", request.RequestID, "has_error", response.Error != nil, "processing_time", response.ProcessingTime)

	// Check for errors and signal appropriately (non-blocking)
	if response.Error != nil {
		slog.ErrorContext(ctx, "Worker got error response", "request_id", request.RequestID, "error", response.Error)
		select {
		case w.orchestrator.coordinationChan <- coordinationMsg{
			msgType:   msgRequestFailed,
			workerID:  w.ID,
			requestID: request.RequestID,
		}:
		default:
			// Coordination busy, continue anyway
		}
	}

	// Send response
	slog.InfoContext(ctx, "Worker attempting to send response to queue", "request_id", request.RequestID, "queue_buffer", len(w.orchestrator.responseQueue), "queue_capacity", cap(w.orchestrator.responseQueue))
	select {
	case w.orchestrator.responseQueue <- response:
		slog.InfoContext(ctx, "Worker successfully sent response to queue", "request_id", request.RequestID, "queue_buffer_after", len(w.orchestrator.responseQueue))
	case <-ctx.Done():
		slog.WarnContext(ctx, "Worker context cancelled while sending response", "request_id", request.RequestID)
		return
	default:
		slog.WarnContext(ctx, "Response queue full, dropping response", "request_id", request.RequestID)
	}

	// Call response callback if provided
	if request.ResultCallback != nil {
		request.ResultCallback(response)
	}
}

// Note: getStatus method removed - status now handled via channel coordination

// handleResponses processes responses from workers
func (o *AnalysisOrchestrator) handleResponses(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case response, ok := <-o.responseQueue:
			if !ok {
				return // Queue closed
			}

			// Log response metrics
			slog.DebugContext(ctx, "Response processed", "request_id", response.RequestID, "processing_time", response.ProcessingTime, "success", response.Error == nil)
		}
	}
}

// collectMetrics periodically collects and updates metrics
func (o *AnalysisOrchestrator) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(o.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case <-ticker.C:
			o.updateMetrics()
		}
	}
}

// performHealthChecks periodically checks the health of pipelines
func (o *AnalysisOrchestrator) performHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(o.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case <-ticker.C:
			o.checkHealth(ctx)
		}
	}
}

// updateMetricsOnSubmit updates metrics when a request is submitted
func (o *AnalysisOrchestrator) updateMetricsOnSubmit() {
	o.metrics.mu.Lock()
	defer o.metrics.mu.Unlock()

	o.metrics.TotalRequests++
	o.metrics.ActiveRequests++
}

// updateMetricsOnComplete updates metrics when a request is completed
func (o *AnalysisOrchestrator) updateMetricsOnComplete() {
	o.metrics.mu.Lock()
	defer o.metrics.mu.Unlock()

	if o.metrics.ActiveRequests > 0 {
		o.metrics.ActiveRequests--
	}
	o.metrics.CompletedRequests++
}

// updateMetrics updates all metrics
func (o *AnalysisOrchestrator) updateMetrics() {
	o.metrics.mu.Lock()
	defer o.metrics.mu.Unlock()

	// Update queue depth
	o.metrics.QueueDepth = len(o.requestQueue)

	// Calculate error rate
	if o.metrics.TotalRequests > 0 {
		o.metrics.ErrorRate = float64(o.metrics.FailedRequests) / float64(o.metrics.TotalRequests)
	}
}

// calculateWorkerUtilization calculates the percentage of workers that are active
// Note: This now queries status via channel coordination instead of direct access
func (o *AnalysisOrchestrator) calculateWorkerUtilization() float64 {
	statuses := o.GetWorkerStatus()
	if len(statuses) == 0 {
		return 0
	}

	activeWorkers := 0
	for _, status := range statuses {
		if status.IsActive {
			activeWorkers++
		}
	}

	return float64(activeWorkers) / float64(len(statuses)) * 100
}

// checkHealth checks the health of all pipelines
func (o *AnalysisOrchestrator) checkHealth(ctx context.Context) {
	for i, pipeline := range o.pipelines {
		if pipeline.storageManager != nil {
			if err := pipeline.storageManager.Health(ctx); err != nil {
				slog.ErrorContext(ctx, "Pipeline storage health check failed", "pipeline_index", i, "error", err)
			}
		}

		if pipeline.llmClient != nil {
			if err := pipeline.llmClient.Health(ctx); err != nil {
				slog.ErrorContext(ctx, "Pipeline LLM health check failed", "pipeline_index", i, "error", err)
			}
		}
	}
}
