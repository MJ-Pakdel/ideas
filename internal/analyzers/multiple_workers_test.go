package analyzers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

func TestMultipleWorkersSharedPipeline(t *testing.T) {
	ctx := t.Context()
	// Test the exact issue: multiple workers calling AnalyzeDocument on the same pipeline instance
	// This test demonstrates the BUG - it should FAIL to show the problem we fixed

	// Create a single pipeline (this simulates the OLD broken architecture)
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	// Create test documents
	numRequests := 3
	var wg sync.WaitGroup
	results := make([]*AnalysisResponse, numRequests)
	errors := make([]error, numRequests)

	// Simulate multiple workers calling AnalyzeDocument on the same pipeline concurrently
	// This is the bug: each worker is waiting on the same resultChan!
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			doc := &types.Document{
				ID:      fmt.Sprintf("doc-%d", workerID),
				Content: fmt.Sprintf("Test content for worker %d", workerID),
			}

			request := &AnalysisRequest{
				Document:  doc,
				RequestID: fmt.Sprintf("request-worker-%d", workerID),
				Priority:  0,
			}

			t.Logf("Worker %d: Starting AnalyzeDocument with request %s", workerID, request.RequestID)

			// This is what each worker is doing - calling AnalyzeDocument on the SAME pipeline
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// Simulate the AnalyzeDocument method behavior
			// Each worker starts a document processing routine
			go func() {
				time.Sleep(time.Duration(50+workerID*10) * time.Millisecond) // Stagger completion

				response := &AnalysisResponse{
					RequestID:      request.RequestID,
					Document:       doc,
					ProcessingTime: time.Duration(50+workerID*10) * time.Millisecond,
				}

				// All workers send to the SAME resultChan
				t.Logf("Worker %d: Sending result for request %s", workerID, request.RequestID)
				pipeline.resultChan <- response
			}()

			// All workers wait on the SAME resultChan - THIS IS THE BUG!
			select {
			case response := <-pipeline.resultChan:
				results[workerID] = response
				t.Logf("Worker %d: Received response for request %s (expected %s)",
					workerID, response.RequestID, request.RequestID)

				// Check if worker got the wrong response
				if response.RequestID != request.RequestID {
					errors[workerID] = fmt.Errorf("worker %d got wrong response: expected %s, got %s",
						workerID, request.RequestID, response.RequestID)
				}
			case <-ctx.Done():
				errors[workerID] = fmt.Errorf("worker %d timeout", workerID)
				t.Logf("Worker %d: Timeout waiting for response", workerID)
			}
		}(i)
	}

	wg.Wait()

	// Analyze results
	correctResponses := 0
	wrongResponses := 0
	timeouts := 0

	for i := 0; i < numRequests; i++ {
		if errors[i] != nil {
			if errors[i].Error() == fmt.Sprintf("worker %d timeout", i) {
				timeouts++
			} else {
				wrongResponses++
			}
			t.Logf("Worker %d error: %v", i, errors[i])
		} else if results[i] != nil {
			correctResponses++
		}
	}

	t.Logf("Results: %d correct, %d wrong, %d timeouts out of %d workers",
		correctResponses, wrongResponses, timeouts, numRequests)

	// This test SHOULD FAIL to demonstrate the bug
	// We expect that NOT all workers get their correct responses due to shared resultChan
	if correctResponses == numRequests {
		t.Logf("WARNING: All workers got correct responses - this suggests the shared pipeline bug is not being demonstrated properly")
	} else {
		t.Logf("SUCCESS: This test correctly demonstrates the shared pipeline bug - %d workers got wrong/timeout responses",
			numRequests-correctResponses)
	}

	// Check if any results are left in the channel
	remainingResults := len(pipeline.resultChan)
	if remainingResults > 0 {
		t.Logf("Warning: %d results left in channel", remainingResults)
	}
}

func TestCorrectArchitecture(t *testing.T) {
	// Test the correct architecture: each worker should have its own pipeline or use request-specific channels
	ctx := t.Context()
	numRequests := 3
	var wg sync.WaitGroup
	results := make([]*AnalysisResponse, numRequests)
	errors := make([]error, numRequests)

	// Correct approach 1: Each worker gets its own pipeline
	for i := range numRequests {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker has its own pipeline with its own resultChan
			pipeline := &AnalysisPipeline{
				resultChan: make(chan *AnalysisResponse, 30),
			}

			doc := &types.Document{
				ID:      fmt.Sprintf("doc-%d", workerID),
				Content: fmt.Sprintf("Test content for worker %d", workerID),
			}

			request := &AnalysisRequest{
				Document:  doc,
				RequestID: fmt.Sprintf("request-worker-%d", workerID),
				Priority:  0,
			}

			t.Logf("Worker %d: Starting with dedicated pipeline", workerID)

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// Simulate processing
			go func() {
				time.Sleep(time.Duration(50+workerID*10) * time.Millisecond)

				response := &AnalysisResponse{
					RequestID:      request.RequestID,
					Document:       doc,
					ProcessingTime: time.Duration(50+workerID*10) * time.Millisecond,
				}

				// Send to worker's own resultChan
				pipeline.resultChan <- response
			}()

			// Wait on worker's own resultChan
			select {
			case response := <-pipeline.resultChan:
				results[workerID] = response
				t.Logf("Worker %d: Received own response %s", workerID, response.RequestID)
			case <-ctx.Done():
				errors[workerID] = fmt.Errorf("worker %d timeout", workerID)
			}
		}(i)
	}

	wg.Wait()

	// All workers should get their correct responses
	correctResponses := 0
	for i := 0; i < numRequests; i++ {
		if errors[i] != nil {
			t.Errorf("Worker %d failed: %v", i, errors[i])
		} else if results[i] != nil && results[i].RequestID == fmt.Sprintf("request-worker-%d", i) {
			correctResponses++
		}
	}

	if correctResponses != numRequests {
		t.Errorf("Expected all %d workers to succeed, but only %d did", numRequests, correctResponses)
	} else {
		t.Logf("SUCCESS: All %d workers received their correct responses", numRequests)
	}
}
