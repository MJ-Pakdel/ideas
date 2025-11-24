package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// ChannelAnalysisWorker implements worker functionality using pure channel communication
// Eliminates all mutex usage by using single goroutine ownership pattern
type ChannelAnalysisWorker struct {
	// Immutable fields (no synchronization needed)
	ID        int
	pipeline  *AnalysisPipeline
	startTime time.Time

	// Channel-based communication (no shared state)
	commands chan WorkerRequestCommand // Control commands (buffered)
	status   chan WorkerStatusQuery    // Status queries (buffered)
	results  chan WorkerResult         // Completion notifications
	shutdown chan struct{}             // Graceful shutdown signal

	// Worker metrics (atomic operations only)
	requestsHandled atomic.Int64
}

// WorkerRequestCommand represents a command sent to the worker for processing
type WorkerRequestCommand struct {
	Type    RequestCommandType
	Request *AnalysisRequest
	Context context.Context
	Done    chan error // Response channel for command completion
}

// RequestCommandType defines the types of commands workers can receive
type RequestCommandType int

const (
	ProcessAnalysisRequest RequestCommandType = iota
	HealthCheckRequest
	GracefulShutdownRequest
)

// WorkerStatusQuery represents a request for worker status
type WorkerStatusQuery struct {
	ResultChan chan WorkerStatus
	Timestamp  time.Time
}

// WorkerHealth represents worker health status
type WorkerHealth int

const (
	HealthyWorker WorkerHealth = iota
	BusyWorker
	UnresponsiveWorker
	FailedWorker
)

// WorkerResult represents the result of a worker operation
type WorkerResult struct {
	WorkerID  int           `json:"worker_id"`
	RequestID string        `json:"request_id"`
	Status    ResultStatus  `json:"status"`
	Error     error         `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// ResultStatus represents the status of a worker result
type ResultStatus int

const (
	ResultSuccess ResultStatus = iota
	ResultError
	ResultTimeout
	ResultCancelled
)

// workerState holds the internal state owned by the worker goroutine
// This struct is only accessed by the single worker goroutine - no synchronization needed
type workerState struct {
	isActive       bool
	currentRequest *AnalysisRequest
	lastActivity   time.Time
	health         WorkerHealth
}

// NewChannelAnalysisWorker creates a new channel-based analysis worker
func NewChannelAnalysisWorker(id int, pipeline *AnalysisPipeline) *ChannelAnalysisWorker {
	return &ChannelAnalysisWorker{
		ID:        id,
		pipeline:  pipeline,
		startTime: time.Now(),
		commands:  make(chan WorkerRequestCommand, 10), // Buffered for responsiveness
		status:    make(chan WorkerStatusQuery, 5),     // Buffered for concurrent queries
		results:   make(chan WorkerResult, 20),         // Buffered for result bursts
		shutdown:  make(chan struct{}),
	}
}

// Start begins the worker's main processing loop
// This is the single goroutine that owns all worker state
func (w *ChannelAnalysisWorker) Start(ctx context.Context) {
	slog.InfoContext(ctx, "Channel worker starting", "worker_id", w.ID)

	// Initialize worker state (owned by this goroutine only)
	state := &workerState{
		isActive:     false,
		lastActivity: time.Now(),
		health:       HealthyWorker,
	}

	// Main event loop - single goroutine owns all state
	for {
		select {
		case cmd := <-w.commands:
			w.handleCommand(ctx, cmd, state)

		case query := <-w.status:
			w.handleStatusQuery(query, state)

		case <-w.shutdown:
			slog.InfoContext(ctx, "Worker shutting down gracefully", "worker_id", w.ID)
			w.cleanup(state)
			return

		case <-ctx.Done():
			slog.InfoContext(ctx, "Worker context cancelled", "worker_id", w.ID)
			w.cleanup(state)
			return
		}
	}
}

// SendCommand sends a command to the worker with timeout
func (w *ChannelAnalysisWorker) SendCommand(ctx context.Context, cmd WorkerRequestCommand) error {
	select {
	case w.commands <- cmd:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrWorkerTimeout
	}
}

// GetStatus queries the current worker status (non-blocking)
func (w *ChannelAnalysisWorker) GetStatus(ctx context.Context) (WorkerStatus, error) {
	query := WorkerStatusQuery{
		ResultChan: make(chan WorkerStatus, 1),
		Timestamp:  time.Now(),
	}

	select {
	case w.status <- query:
		// Wait for response
		select {
		case status := <-query.ResultChan:
			return status, nil
		case <-ctx.Done():
			return WorkerStatus{}, ctx.Err()
		case <-time.After(1 * time.Second):
			return WorkerStatus{}, ErrWorkerTimeout
		}
	case <-ctx.Done():
		return WorkerStatus{}, ctx.Err()
	case <-time.After(1 * time.Second):
		return WorkerStatus{}, ErrWorkerTimeout
	}
}

// GetResultChannel returns the result channel for subscribing to worker results
func (w *ChannelAnalysisWorker) GetResultChannel() <-chan WorkerResult {
	return w.results
}

// Shutdown initiates graceful worker shutdown
func (w *ChannelAnalysisWorker) Shutdown(ctx context.Context) error {
	select {
	case w.shutdown <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrWorkerTimeout
	}
}

// handleCommand processes a command (called only by the worker goroutine)
func (w *ChannelAnalysisWorker) handleCommand(ctx context.Context, cmd WorkerRequestCommand, state *workerState) {
	switch cmd.Type {
	case ProcessAnalysisRequest:
		w.processRequest(ctx, cmd, state)
	case HealthCheckRequest:
		w.processHealthCheck(cmd, state)
	case GracefulShutdownRequest:
		w.processGracefulShutdown(cmd, state)
	default:
		slog.WarnContext(ctx, "Unknown command type", "worker_id", w.ID, "command_type", cmd.Type)
		if cmd.Done != nil {
			cmd.Done <- ErrInvalidCommand
		}
	}
}

// processRequest handles a processing request (no mutex needed - single goroutine)
func (w *ChannelAnalysisWorker) processRequest(ctx context.Context, cmd WorkerRequestCommand, state *workerState) {
	start := time.Now()

	// Update state (no mutex needed - single goroutine ownership)
	state.isActive = true
	state.currentRequest = cmd.Request
	state.lastActivity = start
	state.health = BusyWorker

	defer func() {
		// Update state on completion (no mutex needed)
		state.isActive = false
		state.currentRequest = nil
		state.lastActivity = time.Now()
		state.health = HealthyWorker
		w.requestsHandled.Add(1)
	}()

	slog.DebugContext(ctx, "Processing request",
		"worker_id", w.ID,
		"request_id", cmd.Request.RequestID)

	// Process with timeout
	requestCtx := cmd.Context
	if requestCtx == nil {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	// Process the request
	response := w.pipeline.AnalyzeDocument(requestCtx, cmd.Request)

	// Determine result status
	var status ResultStatus
	var err error

	if response.Error != nil {
		status = ResultError
		err = response.Error
	} else if requestCtx.Err() == context.DeadlineExceeded {
		status = ResultTimeout
		err = requestCtx.Err()
	} else if requestCtx.Err() == context.Canceled {
		status = ResultCancelled
		err = requestCtx.Err()
	} else {
		status = ResultSuccess
	}

	// Send result notification (non-blocking)
	result := WorkerResult{
		WorkerID:  w.ID,
		RequestID: cmd.Request.RequestID,
		Status:    status,
		Error:     err,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	select {
	case w.results <- result:
		// Result sent successfully
	default:
		// Result channel full - log warning but don't block
		slog.WarnContext(ctx, "Result channel full, dropping result notification",
			"worker_id", w.ID,
			"request_id", cmd.Request.RequestID)
	}

	// Call response callback if provided
	if cmd.Request.ResultCallback != nil {
		cmd.Request.ResultCallback(response)
	}

	// Signal command completion
	if cmd.Done != nil {
		select {
		case cmd.Done <- err:
		default:
			// Don't block if caller isn't waiting
		}
	}

	slog.DebugContext(ctx, "Request processing completed",
		"worker_id", w.ID,
		"request_id", cmd.Request.RequestID,
		"status", status,
		"duration", time.Since(start))
}

// processHealthCheck handles a health check command
func (w *ChannelAnalysisWorker) processHealthCheck(cmd WorkerRequestCommand, state *workerState) {
	// Update health based on activity
	now := time.Now()
	if now.Sub(state.lastActivity) > 5*time.Minute {
		state.health = UnresponsiveWorker
	} else if state.isActive {
		state.health = BusyWorker
	} else {
		state.health = HealthyWorker
	}

	state.lastActivity = now

	// Respond to health check
	if cmd.Done != nil {
		select {
		case cmd.Done <- nil:
		default:
		}
	}
}

// processGracefulShutdown handles graceful shutdown command
func (w *ChannelAnalysisWorker) processGracefulShutdown(cmd WorkerRequestCommand, state *workerState) {
	slog.Info("Worker received graceful shutdown command", "worker_id", w.ID)

	// Mark as shutting down
	state.health = FailedWorker

	// Signal completion
	if cmd.Done != nil {
		select {
		case cmd.Done <- nil:
		default:
		}
	}
}

// handleStatusQuery processes a status query (called only by worker goroutine)
func (w *ChannelAnalysisWorker) handleStatusQuery(query WorkerStatusQuery, state *workerState) {
	// Create immutable status snapshot (no mutex needed - single goroutine)
	status := WorkerStatus{
		ID:              w.ID,
		IsActive:        state.isActive,
		RequestsHandled: w.requestsHandled.Load(),
		Uptime:          time.Since(w.startTime),
		LastActivity:    state.lastActivity,
		Health:          w.healthToString(state.health),
	}

	if state.currentRequest != nil {
		status.CurrentRequest = state.currentRequest.RequestID
	}

	// Send response (non-blocking)
	select {
	case query.ResultChan <- status:
		// Status sent successfully
	default:
		// Caller stopped waiting - no problem, just continue
	}
}

// cleanup performs worker cleanup operations
func (w *ChannelAnalysisWorker) cleanup(state *workerState) {
	// Close result channel to signal shutdown
	close(w.results)

	// Clear any remaining state
	state.isActive = false
	state.currentRequest = nil
	state.health = FailedWorker

	slog.Info("Worker cleanup completed", "worker_id", w.ID)
}

// healthToString converts WorkerHealth enum to string
func (w *ChannelAnalysisWorker) healthToString(health WorkerHealth) string {
	switch health {
	case HealthyWorker:
		return "healthy"
	case BusyWorker:
		return "busy"
	case UnresponsiveWorker:
		return "unresponsive"
	case FailedWorker:
		return "failed"
	default:
		return "unknown"
	}
}

// Worker-related errors
var (
	ErrWorkerTimeout  = fmt.Errorf("worker operation timeout")
	ErrInvalidCommand = fmt.Errorf("invalid worker command")
	ErrWorkerShutdown = fmt.Errorf("worker is shutting down")
)
