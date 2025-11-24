package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/idaes/internal/interfaces"
)

// SystemCancellationManager coordinates graceful shutdown across all IDAES components
// This integrates with existing components to provide system-wide cancellation
type SystemCancellationManager struct {
	// Core cancellation coordination
	coordinator *CancellationCoordinator
	timeout     *TimeoutManager

	// Component references for integration
	orchestrator       CancellableComponent
	pipelineStages     CancellableComponent
	metricsBroadcaster CancellableComponent
	workerSupervisor   CancellableComponent
	storageManager     interfaces.StorageManager

	// Integration status
	integrationMutex sync.RWMutex
	components       map[string]CancellableComponent
}

// NewSystemCancellationManager creates a new system-wide cancellation manager
func NewSystemCancellationManager(ctx context.Context) *SystemCancellationManager {
	// Create centralized cancellation coordinator
	timeoutConfig := DefaultTimeoutConfig()
	coordinator := NewCancellationCoordinator(
		ctx,
		timeoutConfig.ShutdownTimeout,
		timeoutConfig.ComponentShutdown,
		timeoutConfig.GracefulShutdown,
	)

	timeoutManager := NewTimeoutManager(timeoutConfig, coordinator)

	return &SystemCancellationManager{
		coordinator: coordinator,
		timeout:     timeoutManager,
		components:  make(map[string]CancellableComponent),
	}
}

// IntegrateOrchestrator integrates the analysis orchestrator with cancellation system
func (scm *SystemCancellationManager) IntegrateOrchestrator(orchestrator CancellableComponent) error {
	scm.integrationMutex.Lock()
	defer scm.integrationMutex.Unlock()

	scm.orchestrator = orchestrator
	scm.components["orchestrator"] = orchestrator

	return scm.coordinator.RegisterComponent("analysis_orchestrator", orchestrator)
}

// IntegratePipelineStages integrates the pipeline stages analyzer with cancellation system
func (scm *SystemCancellationManager) IntegratePipelineStages(pipeline CancellableComponent) error {
	scm.integrationMutex.Lock()
	defer scm.integrationMutex.Unlock()

	scm.pipelineStages = pipeline
	scm.components["pipeline_stages"] = pipeline

	return scm.coordinator.RegisterComponent("pipeline_stages", pipeline)
}

// IntegrateMetricsBroadcaster integrates the metrics broadcaster with cancellation system
func (scm *SystemCancellationManager) IntegrateMetricsBroadcaster(broadcaster CancellableComponent) error {
	scm.integrationMutex.Lock()
	defer scm.integrationMutex.Unlock()

	scm.metricsBroadcaster = broadcaster
	scm.components["metrics_broadcaster"] = broadcaster

	return scm.coordinator.RegisterComponent("metrics_broadcaster", broadcaster)
}

// IntegrateWorkerSupervisor integrates the worker supervisor with cancellation system
func (scm *SystemCancellationManager) IntegrateWorkerSupervisor(supervisor CancellableComponent) error {
	scm.integrationMutex.Lock()
	defer scm.integrationMutex.Unlock()

	scm.workerSupervisor = supervisor
	scm.components["worker_supervisor"] = supervisor

	return scm.coordinator.RegisterComponent("worker_supervisor", supervisor)
}

// IntegrateStorageManager integrates the storage manager with cancellation system
func (scm *SystemCancellationManager) IntegrateStorageManager(storage interfaces.StorageManager) error {
	scm.integrationMutex.Lock()
	defer scm.integrationMutex.Unlock()

	scm.storageManager = storage

	// Wrap storage manager to implement CancellableComponent interface
	storageWrapper := &StorageManagerWrapper{
		storage: storage,
		name:    "storage_manager",
	}
	scm.components["storage_manager"] = storageWrapper

	return scm.coordinator.RegisterComponent("storage_manager", storageWrapper)
}

// StorageManagerWrapper wraps a StorageManager to implement CancellableComponent
type StorageManagerWrapper struct {
	storage interfaces.StorageManager
	name    string
}

func (smw *StorageManagerWrapper) Shutdown(ctx context.Context) error {
	// Storage managers typically don't have explicit shutdown methods
	// This is a graceful no-op shutdown
	slog.Info("storage manager shutdown completed", slog.String("name", smw.name))
	return nil
}

func (smw *StorageManagerWrapper) Name() string {
	return smw.name
}

func (smw *StorageManagerWrapper) IsHealthy(ctx context.Context) bool {
	// Check storage health if available
	if healthChecker, ok := smw.storage.(interface{ Health(context.Context) error }); ok {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return healthChecker.Health(ctx) == nil
	}
	return true // Assume healthy if no health check available
}

// GetSystemContext returns the system-wide cancellation context
func (scm *SystemCancellationManager) GetSystemContext() context.Context {
	return scm.coordinator.GetContext()
}

// GetDoneChannel returns the system-wide done channel
func (scm *SystemCancellationManager) GetDoneChannel() <-chan struct{} {
	return scm.coordinator.GetDoneChannel()
}

// CreateManagedGoroutine starts a goroutine with proper lifecycle management
func (scm *SystemCancellationManager) CreateManagedGoroutine(name string, fn func(ctx context.Context)) {
	scm.coordinator.StartGoroutine(name, fn)
}

// CreateManagedGoroutineWithTimeout starts a goroutine with timeout and lifecycle management
func (scm *SystemCancellationManager) CreateManagedGoroutineWithTimeout(name string, timeout time.Duration, fn func(ctx context.Context)) {
	scm.coordinator.StartGoroutineWithTimeout(name, timeout, fn)
}

// CreateOperationContext creates a context for a specific operation type
func (scm *SystemCancellationManager) CreateOperationContext(ctx context.Context, operationType string) (context.Context, context.CancelFunc) {
	switch operationType {
	case "extraction":
		return scm.timeout.CreateExtractionContext(ctx)
	case "storage":
		return scm.timeout.CreateStorageContext(ctx)
	case "metrics":
		return scm.timeout.CreateMetricsContext(ctx)
	case "network":
		return scm.timeout.CreateNetworkContext(ctx)
	case "worker":
		return scm.timeout.CreateWorkerContext(ctx)
	case "pipeline":
		return scm.timeout.CreatePipelineContext(ctx)
	case "health":
		return scm.timeout.CreateHealthCheckContext(ctx)
	default:
		// Default to system context with no additional timeout
		return context.WithCancel(scm.coordinator.GetContext())
	}
}

// InitiateGracefulShutdown begins system-wide graceful shutdown
func (scm *SystemCancellationManager) InitiateGracefulShutdown(ctx context.Context) error {
	slog.Info("initiating system-wide graceful shutdown")

	// Initiate coordinated shutdown
	if err := scm.coordinator.InitiateShutdown(ctx); err != nil {
		slog.Error("error during shutdown initiation", slog.Any("error", err))
		return fmt.Errorf("shutdown initiation failed: %w", err)
	}

	return nil
}

// WaitForShutdownCompletion waits for all components to complete shutdown
func (scm *SystemCancellationManager) WaitForShutdownCompletion(ctx context.Context) error {
	return scm.coordinator.WaitForShutdown(ctx)
}

// GetSystemHealth returns the overall system health status
func (scm *SystemCancellationManager) GetSystemHealth(ctx context.Context) SystemHealthStatus {
	scm.integrationMutex.RLock()
	defer scm.integrationMutex.RUnlock()

	health := SystemHealthStatus{
		Overall:           true,
		Components:        make(map[string]bool),
		ActiveGoroutines:  scm.coordinator.GetActiveGoroutines(),
		ShutdownInitiated: scm.coordinator.IsShutdownInitiated(),
	}

	// Check health of all components
	for name, component := range scm.components {
		componentHealthy := component.IsHealthy(ctx)
		health.Components[name] = componentHealthy
		if !componentHealthy {
			health.Overall = false
		}
	}

	return health
}

// SystemHealthStatus represents the health of the entire system
type SystemHealthStatus struct {
	Overall           bool            `json:"overall"`
	Components        map[string]bool `json:"components"`
	ActiveGoroutines  map[string]int  `json:"active_goroutines"`
	ShutdownInitiated bool            `json:"shutdown_initiated"`
}

// GetActiveGoroutineCount returns the total number of active goroutines
func (scm *SystemCancellationManager) GetActiveGoroutineCount() int {
	activeGoroutines := scm.coordinator.GetActiveGoroutines()
	total := 0
	for _, count := range activeGoroutines {
		total += count
	}
	return total
}

// IsShutdownInitiated returns true if system shutdown has been initiated
func (scm *SystemCancellationManager) IsShutdownInitiated() bool {
	return scm.coordinator.IsShutdownInitiated()
}

// GetTimeoutManager returns the timeout manager for advanced timeout operations
func (scm *SystemCancellationManager) GetTimeoutManager() *TimeoutManager {
	return scm.timeout
}

// GetCancellationCoordinator returns the cancellation coordinator for advanced operations
func (scm *SystemCancellationManager) GetCancellationCoordinator() *CancellationCoordinator {
	return scm.coordinator
}

// UpdateTimeoutConfiguration updates system-wide timeout configuration
func (scm *SystemCancellationManager) UpdateTimeoutConfiguration(config *TimeoutConfig) error {
	scm.timeout.UpdateTimeouts(config)
	slog.Info("system timeout configuration updated")
	return nil
}

// ForceShutdown performs immediate shutdown without waiting for graceful completion
func (scm *SystemCancellationManager) ForceShutdown(ctx context.Context) error {
	slog.Warn("initiating force shutdown - may cause resource leaks")

	// Cancel the root context immediately
	return scm.coordinator.InitiateShutdown(ctx)
}

// GetIntegratedComponents returns a list of all integrated components
func (scm *SystemCancellationManager) GetIntegratedComponents() []string {
	scm.integrationMutex.RLock()
	defer scm.integrationMutex.RUnlock()

	components := make([]string, 0, len(scm.components))
	for name := range scm.components {
		components = append(components, name)
	}
	return components
}
