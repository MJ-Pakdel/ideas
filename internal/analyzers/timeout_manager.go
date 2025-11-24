package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// TimeoutManager handles comprehensive timeout configuration and enforcement
// for all system operations following graceful cancellation principles
type TimeoutManager struct {
	// Operation timeouts
	ExtractionTimeout time.Duration
	StorageTimeout    time.Duration
	MetricsTimeout    time.Duration
	NetworkTimeout    time.Duration
	ShutdownTimeout   time.Duration

	// Component timeouts
	WorkerTimeout      time.Duration
	PipelineTimeout    time.Duration
	HealthCheckTimeout time.Duration

	// Graceful timeouts
	GracefulShutdown  time.Duration
	ComponentShutdown time.Duration
	GoroutineCleanup  time.Duration

	// Cancellation coordinator for system-wide coordination
	coordinator *CancellationCoordinator
}

// TimeoutConfig holds timeout configuration for different operations
type TimeoutConfig struct {
	// Primary operation timeouts
	ExtractionTimeout time.Duration `yaml:"extraction_timeout"`
	StorageTimeout    time.Duration `yaml:"storage_timeout"`
	MetricsTimeout    time.Duration `yaml:"metrics_timeout"`
	NetworkTimeout    time.Duration `yaml:"network_timeout"`

	// Component lifecycle timeouts
	WorkerTimeout      time.Duration `yaml:"worker_timeout"`
	PipelineTimeout    time.Duration `yaml:"pipeline_timeout"`
	HealthCheckTimeout time.Duration `yaml:"health_check_timeout"`

	// Shutdown timeouts
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	GracefulShutdown  time.Duration `yaml:"graceful_shutdown"`
	ComponentShutdown time.Duration `yaml:"component_shutdown"`
	GoroutineCleanup  time.Duration `yaml:"goroutine_cleanup"`
}

// DefaultTimeoutConfig returns sensible default timeout values
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		// Primary operations
		ExtractionTimeout: 30 * time.Second,
		StorageTimeout:    10 * time.Second,
		MetricsTimeout:    5 * time.Second,
		NetworkTimeout:    15 * time.Second,

		// Component lifecycle
		WorkerTimeout:      60 * time.Second,
		PipelineTimeout:    120 * time.Second,
		HealthCheckTimeout: 5 * time.Second,

		// Shutdown coordination
		ShutdownTimeout:   30 * time.Second,
		GracefulShutdown:  45 * time.Second,
		ComponentShutdown: 20 * time.Second,
		GoroutineCleanup:  15 * time.Second,
	}
}

// NewTimeoutManager creates a new timeout manager with the given configuration
func NewTimeoutManager(config *TimeoutConfig, coordinator *CancellationCoordinator) *TimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	return &TimeoutManager{
		ExtractionTimeout:  config.ExtractionTimeout,
		StorageTimeout:     config.StorageTimeout,
		MetricsTimeout:     config.MetricsTimeout,
		NetworkTimeout:     config.NetworkTimeout,
		WorkerTimeout:      config.WorkerTimeout,
		PipelineTimeout:    config.PipelineTimeout,
		HealthCheckTimeout: config.HealthCheckTimeout,
		ShutdownTimeout:    config.ShutdownTimeout,
		GracefulShutdown:   config.GracefulShutdown,
		ComponentShutdown:  config.ComponentShutdown,
		GoroutineCleanup:   config.GoroutineCleanup,
		coordinator:        coordinator,
	}
}

// CreateExtractionContext creates a context for extraction operations with proper timeout
func (tm *TimeoutManager) CreateExtractionContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		// Use coordinator's context as base to respect system-wide cancellation
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.ExtractionTimeout)
	}
	return context.WithTimeout(parentCtx, tm.ExtractionTimeout)
}

// CreateStorageContext creates a context for storage operations with proper timeout
func (tm *TimeoutManager) CreateStorageContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.StorageTimeout)
	}
	return context.WithTimeout(parentCtx, tm.StorageTimeout)
}

// CreateMetricsContext creates a context for metrics operations with proper timeout
func (tm *TimeoutManager) CreateMetricsContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.MetricsTimeout)
	}
	return context.WithTimeout(parentCtx, tm.MetricsTimeout)
}

// CreateNetworkContext creates a context for network operations with proper timeout
func (tm *TimeoutManager) CreateNetworkContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.NetworkTimeout)
	}
	return context.WithTimeout(parentCtx, tm.NetworkTimeout)
}

// CreateWorkerContext creates a context for worker operations with proper timeout
func (tm *TimeoutManager) CreateWorkerContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.WorkerTimeout)
	}
	return context.WithTimeout(parentCtx, tm.WorkerTimeout)
}

// CreatePipelineContext creates a context for pipeline operations with proper timeout
func (tm *TimeoutManager) CreatePipelineContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.PipelineTimeout)
	}
	return context.WithTimeout(parentCtx, tm.PipelineTimeout)
}

// CreateHealthCheckContext creates a context for health check operations
func (tm *TimeoutManager) CreateHealthCheckContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	if tm.coordinator != nil {
		baseCtx := tm.coordinator.GetContext()
		if parentCtx.Err() != nil {
			baseCtx = parentCtx
		}
		return context.WithTimeout(baseCtx, tm.HealthCheckTimeout)
	}
	return context.WithTimeout(parentCtx, tm.HealthCheckTimeout)
}

// WithTimeoutOperation wraps an operation with timeout and proper error handling
func (tm *TimeoutManager) WithTimeoutOperation(ctx context.Context, timeout time.Duration, operation func(ctx context.Context) error) error {
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use coordinator context if available for system-wide cancellation
	if tm.coordinator != nil {
		coordinatorCtx := tm.coordinator.GetContext()
		// Create a context that respects both timeout and coordinator cancellation
		combinedCtx, combinedCancel := context.WithTimeout(coordinatorCtx, timeout)
		defer combinedCancel()
		timeoutCtx = combinedCtx
	}

	// Execute operation with proper error handling
	operationErr := make(chan error, 1)
	go func() {
		operationErr <- operation(timeoutCtx)
	}()

	select {
	case err := <-operationErr:
		return err
	case <-timeoutCtx.Done():
		return fmt.Errorf("operation timeout after %v: %w", timeout, timeoutCtx.Err())
	}
}

// WithExtractionTimeout wraps an extraction operation with proper timeout
func (tm *TimeoutManager) WithExtractionTimeout(ctx context.Context, operation func(ctx context.Context) error) error {
	return tm.WithTimeoutOperation(ctx, tm.ExtractionTimeout, operation)
}

// WithStorageTimeout wraps a storage operation with proper timeout
func (tm *TimeoutManager) WithStorageTimeout(ctx context.Context, operation func(ctx context.Context) error) error {
	return tm.WithTimeoutOperation(ctx, tm.StorageTimeout, operation)
}

// WithMetricsTimeout wraps a metrics operation with proper timeout
func (tm *TimeoutManager) WithMetricsTimeout(ctx context.Context, operation func(ctx context.Context) error) error {
	return tm.WithTimeoutOperation(ctx, tm.MetricsTimeout, operation)
}

// WithNetworkTimeout wraps a network operation with proper timeout
func (tm *TimeoutManager) WithNetworkTimeout(ctx context.Context, operation func(ctx context.Context) error) error {
	return tm.WithTimeoutOperation(ctx, tm.NetworkTimeout, operation)
}

// MonitorOperation monitors a long-running operation with periodic health checks
func (tm *TimeoutManager) MonitorOperation(ctx context.Context, name string, operation func(ctx context.Context) error) error {
	// Start monitoring
	startTime := time.Now()
	slog.Info("starting monitored operation", slog.String("operation", name))

	// Track with coordinator if available
	if tm.coordinator != nil {
		tm.coordinator.TrackGoroutine(fmt.Sprintf("monitored-%s", name))
		defer tm.coordinator.UntrackGoroutine(fmt.Sprintf("monitored-%s", name))
	}

	// Execute operation
	err := operation(ctx)
	duration := time.Since(startTime)

	if err != nil {
		slog.Error("monitored operation failed",
			slog.String("operation", name),
			slog.Duration("duration", duration),
			slog.Any("error", err))
		return fmt.Errorf("monitored operation %s failed after %v: %w", name, duration, err)
	}

	slog.Info("monitored operation completed",
		slog.String("operation", name),
		slog.Duration("duration", duration))
	return nil
}

// HandleTimeoutError provides standardized timeout error handling
func (tm *TimeoutManager) HandleTimeoutError(err error, operation string, duration time.Duration) error {
	if err == nil {
		return nil
	}

	// Check if it's a context timeout error
	if err == context.DeadlineExceeded {
		slog.Warn("operation timeout",
			slog.String("operation", operation),
			slog.Duration("timeout", duration))
		return fmt.Errorf("%s operation timeout after %v", operation, duration)
	}

	// Check if it's a context cancellation
	if err == context.Canceled {
		slog.Info("operation cancelled",
			slog.String("operation", operation),
			slog.Duration("duration", duration))
		return fmt.Errorf("%s operation cancelled after %v", operation, duration)
	}

	// Other errors
	slog.Error("operation error",
		slog.String("operation", operation),
		slog.Duration("duration", duration),
		slog.Any("error", err))
	return fmt.Errorf("%s operation error: %w", operation, err)
}

// CreateDerivedTimeout creates a derived timeout that respects both parent and operation limits
func (tm *TimeoutManager) CreateDerivedTimeout(parentCtx context.Context, operationTimeout time.Duration) (context.Context, context.CancelFunc) {
	// Get parent deadline if any
	parentDeadline, hasParentDeadline := parentCtx.Deadline()

	// Calculate effective timeout
	now := time.Now()
	effectiveTimeout := operationTimeout

	if hasParentDeadline {
		parentRemaining := parentDeadline.Sub(now)
		if parentRemaining < operationTimeout {
			effectiveTimeout = parentRemaining
		}
	}

	// Use coordinator context if available
	baseCtx := parentCtx
	if tm.coordinator != nil {
		coordinatorCtx := tm.coordinator.GetContext()
		if coordinatorCtx.Err() == nil {
			baseCtx = coordinatorCtx
		}
	}

	return context.WithTimeout(baseCtx, effectiveTimeout)
}

// GetTimeoutConfig returns the current timeout configuration
func (tm *TimeoutManager) GetTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ExtractionTimeout:  tm.ExtractionTimeout,
		StorageTimeout:     tm.StorageTimeout,
		MetricsTimeout:     tm.MetricsTimeout,
		NetworkTimeout:     tm.NetworkTimeout,
		WorkerTimeout:      tm.WorkerTimeout,
		PipelineTimeout:    tm.PipelineTimeout,
		HealthCheckTimeout: tm.HealthCheckTimeout,
		ShutdownTimeout:    tm.ShutdownTimeout,
		GracefulShutdown:   tm.GracefulShutdown,
		ComponentShutdown:  tm.ComponentShutdown,
		GoroutineCleanup:   tm.GoroutineCleanup,
	}
}

// UpdateTimeouts allows dynamic timeout updates
func (tm *TimeoutManager) UpdateTimeouts(config *TimeoutConfig) {
	if config.ExtractionTimeout > 0 {
		tm.ExtractionTimeout = config.ExtractionTimeout
	}
	if config.StorageTimeout > 0 {
		tm.StorageTimeout = config.StorageTimeout
	}
	if config.MetricsTimeout > 0 {
		tm.MetricsTimeout = config.MetricsTimeout
	}
	if config.NetworkTimeout > 0 {
		tm.NetworkTimeout = config.NetworkTimeout
	}
	if config.WorkerTimeout > 0 {
		tm.WorkerTimeout = config.WorkerTimeout
	}
	if config.PipelineTimeout > 0 {
		tm.PipelineTimeout = config.PipelineTimeout
	}
	if config.HealthCheckTimeout > 0 {
		tm.HealthCheckTimeout = config.HealthCheckTimeout
	}

	slog.Info("timeout configuration updated")
}
