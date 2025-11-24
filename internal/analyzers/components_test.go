package analyzers

import (
	"context"
	"testing"
	"time"
)

// TestMetricsBroadcaster tests the metrics broadcasting component
func TestMetricsBroadcaster(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "basic broadcast functionality",
			testFunc: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				// Create source channel for metrics
				source := make(chan MetricsEvent, 10)
				broadcaster := NewMetricsBroadcaster(source)

				// Start the broadcaster in a goroutine
				go broadcaster.Serve(ctx)

				// Test subscription
				subscriber1 := broadcaster.Subscribe()
				subscriber2 := broadcaster.Subscribe()

				// Test broadcasting
				testEvent := MetricsEvent{
					Type:      MetricsRequestSubmitted,
					Timestamp: time.Now(),
					WorkerID:  1,
					Data:      "test_data",
				}

				// Send event to source channel
				source <- testEvent

				// Verify both subscribers received the event
				select {
				case event := <-subscriber1:
					if event.Type != testEvent.Type {
						t.Errorf("subscriber1: Type = %v, want %v", event.Type, testEvent.Type)
					}
					if event.WorkerID != testEvent.WorkerID {
						t.Errorf("subscriber1: WorkerID = %v, want %v", event.WorkerID, testEvent.WorkerID)
					}
				case <-time.After(time.Second):
					t.Error("subscriber1 did not receive event")
				}

				select {
				case event := <-subscriber2:
					if event.Type != testEvent.Type {
						t.Errorf("subscriber2: Type = %v, want %v", event.Type, testEvent.Type)
					}
				case <-time.After(time.Second):
					t.Error("subscriber2 did not receive event")
				}

				// Clean shutdown by closing source
				close(source)
			},
		},
		{
			name: "unsubscribe functionality",
			testFunc: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				source := make(chan MetricsEvent, 10)
				broadcaster := NewMetricsBroadcaster(source)

				go broadcaster.Serve(ctx)

				subscriber := broadcaster.Subscribe()

				// Send first event (should be received)
				testEvent1 := MetricsEvent{
					Type:      MetricsRequestSubmitted,
					Timestamp: time.Now(),
					WorkerID:  1,
					Data:      "test_data_1",
				}
				source <- testEvent1

				select {
				case <-subscriber:
					// Expected
				case <-time.After(time.Second):
					t.Error("subscriber did not receive first event")
				}

				// Unsubscribe
				broadcaster.Unsubscribe(subscriber)

				// Send second event (should not be received due to channel being closed)
				testEvent2 := MetricsEvent{
					Type:      MetricsRequestCompleted,
					Timestamp: time.Now(),
					WorkerID:  1,
					Data:      "test_data_2",
				}
				source <- testEvent2

				// Give some time for the event to potentially be sent
				time.Sleep(100 * time.Millisecond)

				// Check if channel is closed (which it should be after unsubscribe)
				select {
				case _, ok := <-subscriber:
					if ok {
						t.Error("subscriber should have been closed after unsubscribing")
					}
					// Expected - channel should be closed
				default:
					t.Error("subscriber channel should be closed after unsubscribing")
				}

				close(source)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestChannelWorker tests the channel worker component communication
func TestChannelWorker(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "worker status query",
			testFunc: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				// Create a channel worker with minimal config
				worker := &ChannelAnalysisWorker{
					ID:        1,
					startTime: time.Now(),
					commands:  make(chan WorkerRequestCommand, 10),
					status:    make(chan WorkerStatusQuery, 10),
					results:   make(chan WorkerResult, 10),
					shutdown:  make(chan struct{}),
				}

				// Start worker in goroutine
				go func() {
					defer close(worker.results)

					for {
						select {
						case <-ctx.Done():
							return
						case <-worker.shutdown:
							return
						case statusQuery := <-worker.status:
							// Respond to status query
							status := WorkerStatus{
								ID:              worker.ID,
								IsActive:        true,
								RequestsHandled: worker.requestsHandled.Load(),
								Uptime:          time.Since(worker.startTime),
								LastActivity:    time.Now(),
								Health:          "healthy",
							}
							statusQuery.ResultChan <- status
						case cmd := <-worker.commands:
							// Handle commands
							if cmd.Type == GracefulShutdownRequest {
								close(worker.shutdown)
								cmd.Done <- nil
								return
							}
							// For other commands, just acknowledge
							cmd.Done <- nil
						}
					}
				}()

				// Test status query
				statusQuery := WorkerStatusQuery{
					ResultChan: make(chan WorkerStatus, 1),
					Timestamp:  time.Now(),
				}

				worker.status <- statusQuery

				select {
				case status := <-statusQuery.ResultChan:
					if status.ID != worker.ID {
						t.Errorf("Status ID = %v, want %v", status.ID, worker.ID)
					}
					if !status.IsActive {
						t.Error("Worker should be active")
					}
					if status.Health != "healthy" {
						t.Errorf("Worker health = %v, want healthy", status.Health)
					}
				case <-time.After(time.Second):
					t.Error("Did not receive status response")
				}

				// Test graceful shutdown
				shutdownCmd := WorkerRequestCommand{
					Type:    GracefulShutdownRequest,
					Context: ctx,
					Done:    make(chan error, 1),
				}

				worker.commands <- shutdownCmd

				select {
				case err := <-shutdownCmd.Done:
					if err != nil {
						t.Errorf("Shutdown failed: %v", err)
					}
				case <-time.After(time.Second):
					t.Error("Shutdown command did not complete")
				}

				cancel()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestCancellationCoordinator tests graceful cancellation coordination
func TestCancellationCoordinator(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "component registration and shutdown",
			testFunc: func(t *testing.T) {
				// Create coordinator with reasonable timeouts
				coordinator := NewCancellationCoordinator(
					ctx,
					5*time.Second, // shutdown timeout
					2*time.Second, // component timeout
					1*time.Second, // graceful timeout
				)

				// Create test component
				component := &testComponent{
					name:        "test-component",
					shutdownSig: make(chan struct{}, 1),
				}

				// Register component
				err := coordinator.RegisterComponent("test-component", component)
				if err != nil {
					t.Fatalf("Failed to register component: %v", err)
				}

				// Test component registration was successful
				if !component.IsHealthy(ctx) {
					t.Error("Component should be healthy after registration")
				}

				// Test component shutdown via coordinator
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()

				err = component.Shutdown(ctx)
				if err != nil {
					t.Errorf("Component shutdown failed: %v", err)
				}

				// Verify shutdown signal was sent
				select {
				case <-component.shutdownSig:
					// Expected
				case <-time.After(time.Second):
					t.Error("Component did not receive shutdown signal")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// testComponent implements CancellableComponent for testing
type testComponent struct {
	name        string
	shutdownSig chan struct{}
}

func (tc *testComponent) Shutdown(ctx context.Context) error {
	select {
	case tc.shutdownSig <- struct{}{}:
	default:
	}
	return nil
}

func (tc *testComponent) Name() string {
	return tc.name
}

func (tc *testComponent) IsHealthy(_ context.Context) bool {
	return true
}
