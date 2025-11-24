package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// WorkerSupervisor manages the lifecycle of channel-based workers
// Uses pure channel communication without shared mutable state
type WorkerSupervisor struct {
	// Configuration (immutable after creation)
	maxWorkers     int
	pipeline       *AnalysisPipeline
	healthInterval time.Duration

	// Communication channels
	requests     chan *AnalysisRequest  // Incoming work requests
	workerCmds   chan SupervisorCommand // Commands to supervisor
	workerEvents chan WorkerEvent       // Events from workers
	shutdownChan chan struct{}          // Shutdown signal

	// Worker management (owned by supervisor goroutine)
	workers       map[int]*ChannelAnalysisWorker
	workerCounter atomic.Int32

	// Metrics (atomic operations only)
	totalWorkers  atomic.Int32
	activeWorkers atomic.Int32
	failedWorkers atomic.Int64
	processedReqs atomic.Int64
}

// SupervisorCommand represents commands sent to the supervisor
type SupervisorCommand struct {
	Type SupervisorCommandType
	Data any
	Done chan error
}

// WorkerEvent represents events from workers to supervisor
type WorkerEvent struct {
	Type     WorkerEventType
	WorkerID int
	Data     any
	Error    error
}

// WorkerEventType defines types of worker events
type WorkerEventType int

const (
	WorkerStarted WorkerEventType = iota
	WorkerCompleted
	WorkerFailed
	WorkerUnresponsive
	WorkerShutdownEvent
)

// SupervisorCommandType defines supervisor command types
type SupervisorCommandType int

const (
	AddWorker SupervisorCommandType = iota
	RemoveWorker
	GetWorkerStatus
	ScaleWorkers
	ShutdownSupervisor
)

// WorkerPool represents the current state of the worker pool
type WorkerPool struct {
	TotalWorkers  int            `json:"total_workers"`
	ActiveWorkers int            `json:"active_workers"`
	FailedWorkers int64          `json:"failed_workers"`
	ProcessedReqs int64          `json:"processed_requests"`
	Workers       []WorkerStatus `json:"workers"`
	Health        PoolHealth     `json:"health"`
}

// PoolHealth represents the overall health of the worker pool
type PoolHealth string

const (
	PoolHealthy      PoolHealth = "healthy"
	PoolDegraded     PoolHealth = "degraded"
	PoolUnhealthy    PoolHealth = "unhealthy"
	PoolShuttingDown PoolHealth = "shutting_down"
)

// NewWorkerSupervisor creates a new worker supervisor
func NewWorkerSupervisor(maxWorkers int, pipeline *AnalysisPipeline) *WorkerSupervisor {
	return &WorkerSupervisor{
		maxWorkers:     maxWorkers,
		pipeline:       pipeline,
		healthInterval: 30 * time.Second,
		requests:       make(chan *AnalysisRequest, 100),
		workerCmds:     make(chan SupervisorCommand, 20),
		workerEvents:   make(chan WorkerEvent, 100),
		shutdownChan:   make(chan struct{}),
		workers:        make(map[int]*ChannelAnalysisWorker),
	}
}

// Start begins the supervisor's main management loop
func (s *WorkerSupervisor) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Worker supervisor starting", "max_workers", s.maxWorkers)

	// Start initial worker pool
	for i := 0; i < s.maxWorkers; i++ {
		if err := s.createWorker(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to create initial worker", "error", err)
			return err
		}
	}

	// Start health monitoring
	go s.healthMonitor(ctx)

	// Start request distribution
	go s.requestDistributor(ctx)

	// Main supervisor loop
	go s.supervisorLoop(ctx)

	return nil
}

// supervisorLoop is the main event processing loop
func (s *WorkerSupervisor) supervisorLoop(ctx context.Context) {
	slog.InfoContext(ctx, "Supervisor main loop started")

	for {
		select {
		case event := <-s.workerEvents:
			s.handleWorkerEvent(ctx, event)

		case cmd := <-s.workerCmds:
			s.handleSupervisorCommand(ctx, cmd)

		case <-s.shutdownChan:
			slog.InfoContext(ctx, "Supervisor shutting down")
			s.shutdownAllWorkers(ctx)
			return

		case <-ctx.Done():
			slog.InfoContext(ctx, "Supervisor context cancelled")
			s.shutdownAllWorkers(ctx)
			return
		}
	}
}

// createWorker creates and starts a new worker
func (s *WorkerSupervisor) createWorker(ctx context.Context) error {
	workerID := int(s.workerCounter.Add(1))

	worker := NewChannelAnalysisWorker(workerID, s.pipeline)
	s.workers[workerID] = worker

	// Start worker in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "Worker panicked",
					"worker_id", workerID,
					"panic", r)
				s.handleWorkerFailure(ctx, workerID, fmt.Errorf("worker panic: %v", r))
			}
		}()

		worker.Start(ctx)
	}()

	// Subscribe to worker results
	go s.monitorWorkerResults(ctx, worker)

	s.totalWorkers.Add(1)
	s.activeWorkers.Add(1)

	slog.InfoContext(ctx, "Worker created and started", "worker_id", workerID)

	// Notify about worker creation
	s.workerEvents <- WorkerEvent{
		Type:     WorkerStarted,
		WorkerID: workerID,
	}

	return nil
}

// SubmitRequest submits a request for processing
func (s *WorkerSupervisor) SubmitRequest(ctx context.Context, request *AnalysisRequest) error {
	select {
	case s.requests <- request:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrSupervisorTimeout
	}
}

// Shutdown gracefully shuts down the supervisor
func (s *WorkerSupervisor) Shutdown(ctx context.Context) error {
	select {
	case s.shutdownChan <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return ErrSupervisorTimeout
	}
}

// requestDistributor distributes incoming requests to available workers
func (s *WorkerSupervisor) requestDistributor(ctx context.Context) {
	slog.InfoContext(ctx, "Request distributor started")

	for {
		select {
		case request := <-s.requests:
			s.distributeRequest(ctx, request)

		case <-s.shutdownChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// distributeRequest finds an available worker and assigns the request
func (s *WorkerSupervisor) distributeRequest(ctx context.Context, request *AnalysisRequest) {
	// Find available worker using round-robin
	workerID := s.selectWorker()
	if workerID == -1 {
		slog.WarnContext(ctx, "No available workers for request", "request_id", request.RequestID)
		return
	}

	worker, exists := s.workers[workerID]
	if !exists {
		slog.ErrorContext(ctx, "Selected worker does not exist", "worker_id", workerID)
		return
	}

	// Send request to worker
	cmd := WorkerRequestCommand{
		Type:    ProcessAnalysisRequest,
		Request: request,
		Context: ctx,
		Done:    make(chan error, 1),
	}

	err := worker.SendCommand(ctx, cmd)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to send request to worker",
			"worker_id", workerID,
			"request_id", request.RequestID,
			"error", err)

		// Try to create replacement worker
		go s.handleWorkerFailure(ctx, workerID, err)
		return
	}

	s.processedReqs.Add(1)
}

// selectWorker selects an available worker using round-robin
func (s *WorkerSupervisor) selectWorker() int {
	counter := int(s.workerCounter.Add(1))

	for id := range s.workers {
		if (counter % len(s.workers)) == (id % len(s.workers)) {
			return id
		}
	}

	return -1 // No workers available
}

// monitorWorkerResults monitors results from a specific worker
func (s *WorkerSupervisor) monitorWorkerResults(_ context.Context, worker *ChannelAnalysisWorker) {
	results := worker.GetResultChannel()

	for result := range results {
		switch result.Status {
		case ResultSuccess:
			s.workerEvents <- WorkerEvent{
				Type:     WorkerCompleted,
				WorkerID: result.WorkerID,
				Data:     result,
			}

		case ResultError, ResultTimeout:
			s.workerEvents <- WorkerEvent{
				Type:     WorkerFailed,
				WorkerID: result.WorkerID,
				Data:     result,
				Error:    result.Error,
			}
		}
	}

	// Worker result channel closed - worker has shutdown
	s.workerEvents <- WorkerEvent{
		Type:     WorkerShutdownEvent,
		WorkerID: worker.ID,
	}
}

// handleWorkerEvent processes events from workers
func (s *WorkerSupervisor) handleWorkerEvent(ctx context.Context, event WorkerEvent) {
	switch event.Type {
	case WorkerStarted:
		slog.InfoContext(ctx, "Worker started event", "worker_id", event.WorkerID)

	case WorkerCompleted:
		slog.DebugContext(ctx, "Worker completed request", "worker_id", event.WorkerID)

	case WorkerFailed:
		slog.WarnContext(ctx, "Worker failed",
			"worker_id", event.WorkerID,
			"error", event.Error)
		s.failedWorkers.Add(1)

	case WorkerShutdownEvent:
		slog.InfoContext(ctx, "Worker shutdown event", "worker_id", event.WorkerID)
		s.cleanupWorker(event.WorkerID)
	}
}

// handleWorkerFailure handles worker failures and replacement
func (s *WorkerSupervisor) handleWorkerFailure(ctx context.Context, workerID int, err error) {
	slog.WarnContext(ctx, "Handling worker failure",
		"worker_id", workerID,
		"error", err)

	// Remove failed worker
	if worker, exists := s.workers[workerID]; exists {
		worker.Shutdown(ctx)
		delete(s.workers, workerID)
		s.activeWorkers.Add(-1)
	}

	// Create replacement worker if under capacity
	if len(s.workers) < s.maxWorkers {
		if err := s.createWorker(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to create replacement worker", "error", err)
		}
	}
}

// cleanupWorker removes a worker from tracking
func (s *WorkerSupervisor) cleanupWorker(workerID int) {
	delete(s.workers, workerID)
	s.activeWorkers.Add(-1)
	slog.Debug("Worker cleaned up", "worker_id", workerID)
}

// healthMonitor periodically checks worker health
func (s *WorkerSupervisor) healthMonitor(ctx context.Context) {
	ticker := time.NewTicker(s.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performHealthCheck(ctx)

		case <-s.shutdownChan:
			return

		case <-ctx.Done():
			return
		}
	}
}

// performHealthCheck checks health of all workers
func (s *WorkerSupervisor) performHealthCheck(ctx context.Context) {
	for workerID, worker := range s.workers {
		go func(id int, w *ChannelAnalysisWorker) {
			status, err := w.GetStatus(ctx)
			if err != nil {
				slog.WarnContext(ctx, "Health check failed",
					"worker_id", id,
					"error", err)

				s.workerEvents <- WorkerEvent{
					Type:     WorkerUnresponsive,
					WorkerID: id,
					Error:    err,
				}
			}

			// Check if worker is unhealthy
			if status.Health == "failed" || status.Health == "unresponsive" {
				s.workerEvents <- WorkerEvent{
					Type:     WorkerUnresponsive,
					WorkerID: id,
					Error:    fmt.Errorf("worker health: %s", status.Health),
				}
			}
		}(workerID, worker)
	}
}

// GetWorkerPool returns current worker pool status
func (s *WorkerSupervisor) GetWorkerPool(ctx context.Context) (WorkerPool, error) {
	// Collect worker statuses
	workerStatuses := make([]WorkerStatus, 0, len(s.workers))

	for _, worker := range s.workers {
		status, err := worker.GetStatus(ctx)
		if err != nil {
			continue // Skip unresponsive workers
		}
		workerStatuses = append(workerStatuses, status)
	}

	// Determine pool health
	health := s.calculatePoolHealth()

	return WorkerPool{
		TotalWorkers:  int(s.totalWorkers.Load()),
		ActiveWorkers: int(s.activeWorkers.Load()),
		FailedWorkers: s.failedWorkers.Load(),
		ProcessedReqs: s.processedReqs.Load(),
		Workers:       workerStatuses,
		Health:        health,
	}, nil
}

// calculatePoolHealth determines overall pool health
func (s *WorkerSupervisor) calculatePoolHealth() PoolHealth {
	total := int(s.totalWorkers.Load())
	active := int(s.activeWorkers.Load())

	if total == 0 {
		return PoolUnhealthy
	}

	ratio := float64(active) / float64(total)

	switch {
	case ratio >= 0.8:
		return PoolHealthy
	case ratio >= 0.5:
		return PoolDegraded
	default:
		return PoolUnhealthy
	}
}

// shutdownAllWorkers gracefully shuts down all workers
func (s *WorkerSupervisor) shutdownAllWorkers(ctx context.Context) {
	slog.Info("Shutting down all workers")

	// Close request channel to stop accepting new work
	close(s.requests)

	// Shutdown all workers
	for workerID, worker := range s.workers {
		go func(id int, w *ChannelAnalysisWorker) {
			if err := w.Shutdown(ctx); err != nil {
				slog.ErrorContext(ctx, "Error shutting down worker",
					"worker_id", id,
					"error", err)
			}
		}(workerID, worker)
	}

	slog.Info("All workers shut down successfully")
}

// handleSupervisorCommand processes supervisor commands (placeholder)
func (s *WorkerSupervisor) handleSupervisorCommand(ctx context.Context, cmd SupervisorCommand) {
	// Implementation for supervisor commands would go here
	slog.DebugContext(ctx, "Supervisor command received", "type", cmd.Type)
}

// Supervisor-related errors
var (
	ErrSupervisorTimeout  = fmt.Errorf("supervisor operation timeout")
	ErrNoWorkersAvailable = fmt.Errorf("no workers available")
)
