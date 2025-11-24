package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CancellationCoordinator implements the done channel broadcasting pattern
// from GO_CONCURRENCY_PATTERNS.md for system-wide graceful cancellation
type CancellationCoordinator struct {
	// Primary cancellation signal
	rootCtx    context.Context
	rootCancel context.CancelFunc

	// Done channel broadcasting to all subsystems
	done chan struct{}

	// Component registration for coordinated shutdown
	components     map[string]CancellableComponent
	componentMutex sync.RWMutex

	// Shutdown coordination
	shutdownOnce sync.Once
	shutdownDone chan struct{}

	// Timeout configuration
	shutdownTimeout  time.Duration
	componentTimeout time.Duration
	gracefulTimeout  time.Duration

	// Goroutine tracking for leak prevention
	activeGoroutines sync.WaitGroup
	goroutineTracker map[string]int
	trackerMutex     sync.RWMutex
}

// CancellableComponent defines the interface for components that can be gracefully cancelled
type CancellableComponent interface {
	// Shutdown gracefully shuts down the component
	Shutdown(ctx context.Context) error

	// Name returns the component name for logging
	Name() string

	// IsHealthy returns true if the component is functioning properly
	IsHealthy(ctx context.Context) bool
}

// GoroutineInfo tracks information about active goroutines
type GoroutineInfo struct {
	Name      string
	StartTime time.Time
	Context   context.Context
	Cancel    context.CancelFunc
}

// NewCancellationCoordinator creates a new cancellation coordinator
func NewCancellationCoordinator(ctx context.Context, shutdownTimeout, componentTimeout, gracefulTimeout time.Duration) *CancellationCoordinator {
	ctx, cancel := context.WithCancel(ctx)

	return &CancellationCoordinator{
		rootCtx:          ctx,
		rootCancel:       cancel,
		done:             make(chan struct{}),
		components:       make(map[string]CancellableComponent),
		shutdownDone:     make(chan struct{}),
		shutdownTimeout:  shutdownTimeout,
		componentTimeout: componentTimeout,
		gracefulTimeout:  gracefulTimeout,
		goroutineTracker: make(map[string]int),
	}
}

// RegisterComponent registers a component for coordinated shutdown
func (cc *CancellationCoordinator) RegisterComponent(name string, component CancellableComponent) error {
	cc.componentMutex.Lock()
	defer cc.componentMutex.Unlock()

	if _, exists := cc.components[name]; exists {
		return fmt.Errorf("component %s already registered", name)
	}

	cc.components[name] = component
	slog.Info("registered component for cancellation coordination", slog.String("component", name))
	return nil
}

// UnregisterComponent removes a component from coordination
func (cc *CancellationCoordinator) UnregisterComponent(name string) {
	cc.componentMutex.Lock()
	defer cc.componentMutex.Unlock()

	delete(cc.components, name)
	slog.Info("unregistered component from cancellation coordination", slog.String("component", name))
}

// GetContext returns a context that will be cancelled when shutdown is initiated
func (cc *CancellationCoordinator) GetContext() context.Context {
	return cc.rootCtx
}

// GetDoneChannel returns the done channel for broadcasting shutdown signals
func (cc *CancellationCoordinator) GetDoneChannel() <-chan struct{} {
	return cc.done
}

// CreateDerivedContext creates a context with timeout that respects the cancellation coordinator
func (cc *CancellationCoordinator) CreateDerivedContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(cc.rootCtx, timeout)
	}
	return context.WithCancel(cc.rootCtx)
}

// TrackGoroutine tracks a goroutine for leak prevention
func (cc *CancellationCoordinator) TrackGoroutine(name string) {
	cc.activeGoroutines.Add(1)

	cc.trackerMutex.Lock()
	cc.goroutineTracker[name]++
	count := cc.goroutineTracker[name]
	cc.trackerMutex.Unlock()

	slog.Debug("tracking goroutine", slog.String("name", name), slog.Int("count", count))
}

// UntrackGoroutine marks a goroutine as completed
func (cc *CancellationCoordinator) UntrackGoroutine(name string) {
	cc.trackerMutex.Lock()
	if count, exists := cc.goroutineTracker[name]; exists && count > 0 {
		cc.goroutineTracker[name]--
		if cc.goroutineTracker[name] == 0 {
			delete(cc.goroutineTracker, name)
		}
	}
	cc.trackerMutex.Unlock()

	cc.activeGoroutines.Done()
	slog.Debug("untracked goroutine", slog.String("name", name))
}

// StartGoroutine starts a goroutine with proper tracking and cancellation
func (cc *CancellationCoordinator) StartGoroutine(name string, fn func(ctx context.Context)) {
	cc.TrackGoroutine(name)

	go func() {
		defer cc.UntrackGoroutine(name)

		// Create derived context for this goroutine
		ctx, cancel := context.WithCancel(cc.rootCtx)
		defer cancel()

		// Run the goroutine function
		fn(ctx)
	}()
}

// StartGoroutineWithTimeout starts a goroutine with timeout and proper tracking
func (cc *CancellationCoordinator) StartGoroutineWithTimeout(name string, timeout time.Duration, fn func(ctx context.Context)) {
	cc.TrackGoroutine(name)

	go func() {
		defer cc.UntrackGoroutine(name)

		// Create derived context with timeout
		ctx, cancel := cc.CreateDerivedContext(timeout)
		defer cancel()

		// Run the goroutine function
		fn(ctx)
	}()
}

// InitiateShutdown begins the graceful shutdown process
func (cc *CancellationCoordinator) InitiateShutdown(ctx context.Context) error {
	var shutdownErr error

	cc.shutdownOnce.Do(func() {
		slog.Info("initiating system-wide graceful shutdown")

		// Signal all components via done channel broadcasting
		close(cc.done)

		// Cancel the root context to propagate cancellation
		cc.rootCancel()

		// Start coordinated component shutdown
		go func() {
			defer close(cc.shutdownDone)
			shutdownErr = cc.coordinateComponentShutdown(ctx)
		}()
	})

	return shutdownErr
}

// coordinateComponentShutdown handles the shutdown of all registered components
func (cc *CancellationCoordinator) coordinateComponentShutdown(ctx context.Context) error {
	cc.componentMutex.RLock()
	componentCount := len(cc.components)
	components := make(map[string]CancellableComponent, componentCount)
	for name, comp := range cc.components {
		components[name] = comp
	}
	cc.componentMutex.RUnlock()

	if componentCount == 0 {
		slog.Info("no components to shutdown")
		return nil
	}

	slog.Info("shutting down components", slog.Int("count", componentCount))

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, cc.shutdownTimeout)
	defer cancel()

	// Channel to collect shutdown results
	resultsChan := make(chan componentShutdownResult, componentCount)

	// Start shutdown for all components in parallel
	for name, component := range components {
		go cc.shutdownComponent(shutdownCtx, name, component, resultsChan)
	}

	// Collect results
	var errors []error
	for range componentCount {
		select {
		case result := <-resultsChan:
			if result.err != nil {
				slog.Error("component shutdown failed",
					slog.String("component", result.name),
					slog.Any("error", result.err),
					slog.Duration("duration", result.duration))
				errors = append(errors, fmt.Errorf("component %s: %w", result.name, result.err))
			} else {
				slog.Info("component shutdown completed",
					slog.String("component", result.name),
					slog.Duration("duration", result.duration))
			}
		case <-shutdownCtx.Done():
			slog.Error("component shutdown timeout exceeded")
			errors = append(errors, fmt.Errorf("shutdown timeout exceeded %w", shutdownCtx.Err()))
			return fmt.Errorf("shutdown timeout exceeded %v", errors)
		}
	}

	// Wait for all goroutines to complete with timeout
	goroutinesDone := make(chan struct{})
	go func() {
		cc.activeGoroutines.Wait()
		close(goroutinesDone)
	}()

	gracefulCtx, gracefulCancel := context.WithTimeout(ctx, cc.gracefulTimeout)
	defer gracefulCancel()

	select {
	case <-goroutinesDone:
		slog.Info("all goroutines completed gracefully")
	case <-gracefulCtx.Done():
		cc.trackerMutex.RLock()
		activeCount := len(cc.goroutineTracker)
		activeNames := make([]string, 0, len(cc.goroutineTracker))
		for name := range cc.goroutineTracker {
			activeNames = append(activeNames, name)
		}
		cc.trackerMutex.RUnlock()

		slog.Warn("goroutine shutdown timeout - potential leaks",
			slog.Int("active_goroutines", activeCount),
			slog.Any("active_names", activeNames))
		errors = append(errors, fmt.Errorf("goroutine shutdown timeout: %d goroutines still active", activeCount))
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with errors: %v", errors)
	}

	slog.Info("graceful shutdown completed successfully")
	return nil
}

// componentShutdownResult holds the result of a component shutdown
type componentShutdownResult struct {
	name     string
	err      error
	duration time.Duration
}

// shutdownComponent shuts down a single component with timeout
func (cc *CancellationCoordinator) shutdownComponent(ctx context.Context, name string, component CancellableComponent, resultsChan chan<- componentShutdownResult) {
	startTime := time.Now()

	// Create component-specific context with timeout
	componentCtx, cancel := context.WithTimeout(ctx, cc.componentTimeout)
	defer cancel()

	err := component.Shutdown(componentCtx)
	duration := time.Since(startTime)

	resultsChan <- componentShutdownResult{
		name:     name,
		err:      err,
		duration: duration,
	}
}

// WaitForShutdown waits for the shutdown process to complete
func (cc *CancellationCoordinator) WaitForShutdown(ctx context.Context) error {
	select {
	case <-cc.shutdownDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown wait timeout: %w", ctx.Err())
	}
}

// GetActiveGoroutines returns information about currently active goroutines
func (cc *CancellationCoordinator) GetActiveGoroutines() map[string]int {
	cc.trackerMutex.RLock()
	defer cc.trackerMutex.RUnlock()

	result := make(map[string]int, len(cc.goroutineTracker))
	for name, count := range cc.goroutineTracker {
		result[name] = count
	}
	return result
}

// IsShutdownInitiated returns true if shutdown has been initiated
func (cc *CancellationCoordinator) IsShutdownInitiated() bool {
	select {
	case <-cc.done:
		return true
	default:
		return false
	}
}

// Health returns the overall health status of the cancellation coordinator
func (cc *CancellationCoordinator) Health(ctx context.Context) bool {
	cc.componentMutex.RLock()
	defer cc.componentMutex.RUnlock()

	for _, component := range cc.components {
		if !component.IsHealthy(ctx) {
			return false
		}
	}
	return true
}
