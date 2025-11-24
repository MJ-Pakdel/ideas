package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ChannelBasedOrchestrator implements high-performance channel-based request processing
// Following Go principle: "Do not communicate by sharing memory; instead, share memory by communicating"
type ChannelBasedOrchestrator struct {
	// Configuration (immutable after creation)
	config *OrchestratorConfig

	// Channel-based communication (no mutexes!)
	requestChan  chan *AnalysisRequest
	responseChan chan *AnalysisResponse

	// Worker management channels
	workerStatusChan chan WorkerStatusUpdate
	workerCmdChan    chan WorkerCommand

	// Metrics event streaming
	metricsEventChan chan MetricsEvent

	// Graceful shutdown coordination
	shutdownChan chan struct{}
	doneChan     chan struct{}

	// Worker management (no mutex needed - managed via channels)
	workerSupervisor *WorkerSupervisor

	// Metrics broadcaster (no mutex - event driven)
	metricsBroadcaster *MetricsBroadcaster
}

// WorkerStatusUpdate represents worker status changes
type WorkerStatusUpdate struct {
	WorkerID    int
	Status      WorkerState
	RequestID   string
	ProcessTime time.Duration
	Error       error
}

// WorkerCommand represents commands sent to workers
type WorkerCommand struct {
	Type     CommandType
	WorkerID int
	Data     any
}

// WorkerState represents possible worker states
type WorkerState int

const (
	WorkerIdle WorkerState = iota
	WorkerBusy
	WorkerShutdown
	WorkerError
)

// CommandType represents worker command types
type CommandType int

const (
	CmdShutdown CommandType = iota
	CmdHealthCheck
	CmdRestart
)

// MetricsEvent represents metrics events for broadcasting
type MetricsEvent struct {
	Type      MetricsEventType
	Timestamp time.Time
	WorkerID  int
	Data      any
}

// MetricsEventType represents types of metrics events
type MetricsEventType int

const (
	MetricsRequestSubmitted MetricsEventType = iota
	MetricsRequestCompleted
	MetricsRequestFailed
	MetricsWorkerUtilization
	MetricsQueueDepth
)

// NewChannelBasedOrchestrator creates a new high-performance orchestrator
func NewChannelBasedOrchestrator(config *OrchestratorConfig) *ChannelBasedOrchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	return &ChannelBasedOrchestrator{
		config:           config,
		requestChan:      make(chan *AnalysisRequest, config.QueueSize),
		responseChan:     make(chan *AnalysisResponse, config.QueueSize),
		workerStatusChan: make(chan WorkerStatusUpdate, config.MaxWorkers*2), // Buffer for worker events
		workerCmdChan:    make(chan WorkerCommand, config.MaxWorkers),
		metricsEventChan: make(chan MetricsEvent, 1000), // High-throughput metrics
		shutdownChan:     make(chan struct{}),
		doneChan:         make(chan struct{}),
	}
}

// Start initializes the orchestrator with full channel-based coordination
func (o *ChannelBasedOrchestrator) Start(ctx context.Context, pipelines []*AnalysisPipeline) error {
	if len(pipelines) == 0 {
		return fmt.Errorf("no analysis pipelines provided")
	}

	slog.InfoContext(ctx, "Starting channel-based orchestrator",
		"max_workers", o.config.MaxWorkers,
		"queue_size", o.config.QueueSize,
		"pipelines", len(pipelines))

	// Start metrics broadcaster (Phase 3 - will be implemented next)
	o.metricsBroadcaster = NewMetricsBroadcaster(o.metricsEventChan)
	go o.metricsBroadcaster.Serve(ctx)

	// Start worker supervisor with first pipeline (simplified for now)
	if len(pipelines) > 0 {
		o.workerSupervisor = NewWorkerSupervisor(o.config.MaxWorkers, pipelines[0])
		if err := o.workerSupervisor.Start(ctx); err != nil {
			return fmt.Errorf("failed to start worker supervisor: %w", err)
		}
	}

	// Start metrics collection goroutine
	go o.collectMetrics(ctx)

	// Start health monitoring
	go o.monitorHealth(ctx)

	slog.InfoContext(ctx, "Channel-based orchestrator started successfully")
	return nil
}

// Stop gracefully shuts down the orchestrator using channel coordination
func (o *ChannelBasedOrchestrator) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Stopping channel-based orchestrator")

	// Shutdown worker supervisor if it exists
	if o.workerSupervisor != nil {
		if err := o.workerSupervisor.Shutdown(ctx); err != nil {
			slog.WarnContext(ctx, "Worker supervisor shutdown error", "error", err)
		}
	}

	// Signal shutdown to all components
	close(o.shutdownChan)

	// Wait for graceful shutdown with timeout
	select {
	case <-o.doneChan:
		slog.InfoContext(ctx, "Orchestrator stopped gracefully")
	case <-ctx.Done():
		slog.WarnContext(ctx, "Orchestrator shutdown timeout")
		return ctx.Err()
	}

	return nil
}

// SubmitRequest submits analysis requests via channels (no mutex!)
func (o *ChannelBasedOrchestrator) SubmitRequest(ctx context.Context, request *AnalysisRequest) error {
	// Add request ID if not present
	if request.RequestID == "" {
		request.RequestID = fmt.Sprintf("req_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
	}

	// Send metrics event
	o.metricsEventChan <- MetricsEvent{
		Type:      MetricsRequestSubmitted,
		Timestamp: time.Now(),
		Data:      request.RequestID,
	}

	// Submit request via worker supervisor if available
	if o.workerSupervisor != nil {
		return o.workerSupervisor.SubmitRequest(ctx, request)
	}

	// Fallback to direct channel submission
	select {
	case o.requestChan <- request:
		slog.DebugContext(ctx, "Request submitted successfully", "request_id", request.RequestID)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("request queue is full")
	}
}

// GetResponse retrieves responses via channels (no mutex!)
func (o *ChannelBasedOrchestrator) GetResponse(ctx context.Context) (*AnalysisResponse, error) {
	select {
	case response := <-o.responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetResponseChannel returns the response channel for direct access
func (o *ChannelBasedOrchestrator) GetResponseChannel() <-chan *AnalysisResponse {
	return o.responseChan
}

// SubscribeToMetrics returns a channel for receiving metrics events
func (o *ChannelBasedOrchestrator) SubscribeToMetrics() <-chan MetricsEvent {
	return o.metricsBroadcaster.Subscribe()
}

// UnsubscribeFromMetrics cancels a metrics subscription
func (o *ChannelBasedOrchestrator) UnsubscribeFromMetrics(subscription <-chan MetricsEvent) {
	o.metricsBroadcaster.Unsubscribe(subscription)
}

// collectMetrics aggregates metrics from worker status updates
func (o *ChannelBasedOrchestrator) collectMetrics(ctx context.Context) {
	defer close(o.doneChan) // Signal completion when this goroutine exits

	ticker := time.NewTicker(o.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case <-ticker.C:
			// Collect queue depth metrics
			o.metricsEventChan <- MetricsEvent{
				Type:      MetricsQueueDepth,
				Timestamp: time.Now(),
				Data:      len(o.requestChan),
			}
		case statusUpdate := <-o.workerStatusChan:
			// Forward worker status to metrics
			o.handleWorkerStatusUpdate(statusUpdate)
		}
	}
}

// handleWorkerStatusUpdate processes worker status changes and emits metrics
func (o *ChannelBasedOrchestrator) handleWorkerStatusUpdate(update WorkerStatusUpdate) {
	// Emit completion metrics
	if update.Status == WorkerIdle && update.RequestID != "" {
		eventType := MetricsRequestCompleted
		if update.Error != nil {
			eventType = MetricsRequestFailed
		}

		o.metricsEventChan <- MetricsEvent{
			Type:      eventType,
			Timestamp: time.Now(),
			WorkerID:  update.WorkerID,
			Data: map[string]any{
				"request_id":   update.RequestID,
				"process_time": update.ProcessTime,
				"error":        update.Error,
			},
		}
	}
}

// monitorHealth performs health checks using channel communication
func (o *ChannelBasedOrchestrator) monitorHealth(ctx context.Context) {
	if o.config.HealthCheckInterval <= 0 {
		return // Health monitoring disabled
	}

	ticker := time.NewTicker(o.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-o.shutdownChan:
			return
		case <-ticker.C:
			// Send health check commands to all workers
			for i := 0; i < o.config.MaxWorkers; i++ {
				select {
				case o.workerCmdChan <- WorkerCommand{
					Type:     CmdHealthCheck,
					WorkerID: i,
				}:
				default:
					// Non-blocking - if command channel is full, skip this check
				}
			}
		}
	}
}
