package analyzers

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

// TestBasicChannelCoordination tests basic channel coordination without complex orchestration
func TestBasicChannelCoordination(t *testing.T) {
	ctx := t.Context()
	config := DefaultOrchestratorConfig()
	config.MaxWorkers = 1
	config.QueueSize = 2
	config.MetricsInterval = 0     // Disable
	config.HealthCheckInterval = 0 // Disable

	orchestrator := NewAnalysisOrchestrator(config)

	// Create a minimal mock pipeline
	mockPipeline := &AnalysisPipeline{
		config:  DefaultAnalysisConfig(),
		metrics: &PipelineMetrics{},
	}

	// Test 1: Add pipeline (should work even without coordination running)
	orchestrator.AddPipeline(mockPipeline)

	// Test 2: Start orchestrator
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := orchestrator.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Give minimal time for startup
	time.Sleep(50 * time.Millisecond)

	// Test 3: Get metrics (basic functionality)
	metrics := orchestrator.GetMetrics()
	if metrics == nil {
		t.Error("Expected metrics, got nil")
	}

	// Test 4: Stop orchestrator
	stopCtx, stopCancel := context.WithTimeout(ctx, 1*time.Second)
	defer stopCancel()

	err = orchestrator.Stop(stopCtx)
	if err != nil {
		t.Errorf("Failed to stop orchestrator: %v", err)
	}

	t.Log("Basic channel coordination test passed")
}

// TestAnalysisRequestProcessing tests basic request/response patterns
func TestAnalysisRequestProcessing(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "analysis request creation and validation",
			testFunc: func(t *testing.T) {
				// Create test document
				testDoc := &types.Document{
					ID:      "test-doc-1",
					Name:    "test.txt",
					Content: "This is a test document for processing.",
				}

				// Create analysis request
				request := &AnalysisRequest{
					Document:  testDoc,
					RequestID: "req-001",
					Priority:  1,
				}

				// Validate request fields
				if request.Document != testDoc {
					t.Error("Request document not set correctly")
				}

				if request.RequestID != "req-001" {
					t.Errorf("RequestID = %v, want req-001", request.RequestID)
				}

				if request.Priority != 1 {
					t.Errorf("Priority = %v, want 1", request.Priority)
				}
			},
		},
		{
			name: "analysis response creation",
			testFunc: func(t *testing.T) {
				testDoc := &types.Document{
					ID:      "test-doc-1",
					Name:    "test.txt",
					Content: "Test content",
				}

				testResult := &types.AnalysisResult{
					DocumentID:     testDoc.ID,
					ProcessingTime: 100 * time.Millisecond,
					Confidence:     0.95,
				}

				response := &AnalysisResponse{
					RequestID:      "req-001",
					Document:       testDoc,
					Result:         testResult,
					ProcessingTime: 150 * time.Millisecond,
					Timestamp:      time.Now(),
				}

				// Validate response fields
				if response.RequestID != "req-001" {
					t.Errorf("RequestID = %v, want req-001", response.RequestID)
				}

				if response.Document != testDoc {
					t.Error("Response document not set correctly")
				}

				if response.Result != testResult {
					t.Error("Response result not set correctly")
				}

				if response.ProcessingTime != 150*time.Millisecond {
					t.Errorf("ProcessingTime = %v, want 150ms", response.ProcessingTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestOrchestratorConfigPattern tests orchestrator configuration patterns
func TestOrchestratorConfigPattern(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "default orchestrator configuration",
			testFunc: func(t *testing.T) {
				config := DefaultOrchestratorConfig()

				// Validate default values
				if config.MaxWorkers <= 0 {
					t.Errorf("MaxWorkers = %v, should be positive", config.MaxWorkers)
				}

				if config.QueueSize <= 0 {
					t.Errorf("QueueSize = %v, should be positive", config.QueueSize)
				}

				if config.ProcessingTimeout <= 0 {
					t.Errorf("ProcessingTimeout = %v, should be positive", config.ProcessingTimeout)
				}

				if config.WorkerIdleTimeout <= 0 {
					t.Errorf("WorkerIdleTimeout = %v, should be positive", config.WorkerIdleTimeout)
				}
			},
		},
		{
			name: "custom orchestrator configuration",
			testFunc: func(t *testing.T) {
				config := &OrchestratorConfig{
					MaxWorkers:        5,
					QueueSize:         200,
					ProcessingTimeout: 30 * time.Second,
					WorkerIdleTimeout: 5 * time.Minute,
					MaxRetries:        3,
					RetryDelay:        1 * time.Second,
					EnablePriority:    true,
					LoadBalancing:     "round_robin",
					MetricsInterval:   10 * time.Second,
				}

				// Validate custom values
				if config.MaxWorkers != 5 {
					t.Errorf("MaxWorkers = %v, want 5", config.MaxWorkers)
				}

				if config.QueueSize != 200 {
					t.Errorf("QueueSize = %v, want 200", config.QueueSize)
				}

				if config.LoadBalancing != "round_robin" {
					t.Errorf("LoadBalancing = %v, want round_robin", config.LoadBalancing)
				}

				if !config.EnablePriority {
					t.Error("EnablePriority should be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestWorkerStatusPattern tests worker status communication patterns
func TestWorkerStatusPattern(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "worker status creation and validation",
			testFunc: func(t *testing.T) {
				now := time.Now()
				uptime := 5 * time.Minute

				status := &WorkerStatus{
					ID:              1,
					IsActive:        true,
					CurrentRequest:  "req-001",
					RequestsHandled: 10,
					Uptime:          uptime,
					LastActivity:    now,
					Health:          "healthy",
				}

				// Validate status fields
				if status.ID != 1 {
					t.Errorf("ID = %v, want 1", status.ID)
				}

				if !status.IsActive {
					t.Error("Worker should be active")
				}

				if status.CurrentRequest != "req-001" {
					t.Errorf("CurrentRequest = %v, want req-001", status.CurrentRequest)
				}

				if status.RequestsHandled != 10 {
					t.Errorf("RequestsHandled = %v, want 10", status.RequestsHandled)
				}

				if status.Uptime != uptime {
					t.Errorf("Uptime = %v, want %v", status.Uptime, uptime)
				}

				if status.Health != "healthy" {
					t.Errorf("Health = %v, want healthy", status.Health)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestMetricsEventPattern tests metrics event creation and broadcasting
func TestMetricsEventPattern(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "metrics event creation",
			testFunc: func(t *testing.T) {
				now := time.Now()

				event := &MetricsEvent{
					Type:      MetricsRequestSubmitted,
					Timestamp: now,
					WorkerID:  1,
					Data:      "test data",
				}

				// Validate event fields
				if event.Type != MetricsRequestSubmitted {
					t.Errorf("Type = %v, want %v", event.Type, MetricsRequestSubmitted)
				}

				if event.WorkerID != 1 {
					t.Errorf("WorkerID = %v, want 1", event.WorkerID)
				}

				if event.Data != "test data" {
					t.Errorf("Data = %v, want 'test data'", event.Data)
				}

				if event.Timestamp != now {
					t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
				}
			},
		},
		{
			name: "metrics event types validation",
			testFunc: func(t *testing.T) {
				// Test different event types
				eventTypes := []MetricsEventType{
					MetricsRequestSubmitted,
					MetricsRequestCompleted,
					MetricsRequestFailed,
					MetricsWorkerUtilization,
					MetricsQueueDepth,
				}

				for _, eventType := range eventTypes {
					event := &MetricsEvent{
						Type:      eventType,
						Timestamp: time.Now(),
						WorkerID:  1,
						Data:      "test",
					}

					if event.Type != eventType {
						t.Errorf("Event type = %v, want %v", event.Type, eventType)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// BenchmarkPhase2_ChannelOrchestrator benchmarks the channel-based orchestrator
func BenchmarkPhase2_ChannelOrchestrator(b *testing.B) {
	concurrencyLevels := []int{1, 10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			// Create channel-based orchestrator configuration with shorter timeouts
			config := &OrchestratorConfig{
				MaxWorkers:          concurrency,
				QueueSize:           1000,
				ProcessingTimeout:   1 * time.Second,
				WorkerIdleTimeout:   2 * time.Second,
				MetricsInterval:     5 * time.Second,
				HealthCheckInterval: 10 * time.Second,
			}

			orchestrator := NewChannelBasedOrchestrator(config)

			// Create context with timeout for the benchmark
			ctx, cancel := context.WithTimeout(b.Context(), 5*time.Second)
			defer cancel()

			// Create dummy pipeline for Start method
			pipeline := &AnalysisPipeline{}

			// Start orchestrator in background with proper cancellation
			done := make(chan error, 1)
			go func() {
				done <- orchestrator.Start(ctx, []*AnalysisPipeline{pipeline})
			}()

			// Wait a bit for orchestrator to start up
			time.Sleep(100 * time.Millisecond)

			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ResetTimer()

			// Benchmark request submission and response handling
			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup

				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func(reqIndex int) {
						defer wg.Done()

						request := &AnalysisRequest{
							RequestID: fmt.Sprintf("bench-%d-%d", i, reqIndex),
							Document: &types.Document{
								ID:      fmt.Sprintf("doc-%d", reqIndex),
								Content: "Test document content for benchmarking performance",
							},
							Priority: 1,
						}

						// Test request submission with short timeout
						submitCtx, submitCancel := context.WithTimeout(ctx, 100*time.Millisecond)
						_ = orchestrator.SubmitRequest(submitCtx, request)
						submitCancel()
					}(j)
				}

				wg.Wait()
			}

			b.StopTimer()

			// Measure memory usage
			runtime.GC()
			runtime.ReadMemStats(&memAfter)

			allocDiff := memAfter.TotalAlloc - memBefore.TotalAlloc
			b.ReportMetric(float64(allocDiff)/float64(b.N), "bytes/op")
			b.ReportMetric(float64(concurrency), "concurrency")

			// Ensure orchestrator is properly stopped
			cancel()
			select {
			case <-done:
				// Orchestrator stopped cleanly
			case <-time.After(2 * time.Second):
				// Force timeout
				b.Log("Warning: Orchestrator did not stop cleanly within timeout")
			}
		})
	}
}
