package analyzers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

func TestPipelineResultChannelSync(t *testing.T) {
	ctx := t.Context()
	// Test the specific synchronization issue: pipeline sends result but AnalyzeDocument doesn't receive it

	// Create a minimal pipeline setup
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30), // Same buffer size as production
	}

	// Test document and request
	doc := &types.Document{
		ID:      "test-sync-doc",
		Content: "Test content for sync testing",
	}

	request := &AnalysisRequest{
		Document:  doc,
		RequestID: "test-sync-request",
		Priority:  0,
	}

	// Track what happens
	var wg sync.WaitGroup
	var senderComplete, receiverComplete bool
	var senderErr, receiverErr error
	var receivedResponse *AnalysisResponse

	// Simulate the sender (storage stage)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { senderComplete = true }()

		// Simulate some processing delay
		time.Sleep(100 * time.Millisecond)

		response := &AnalysisResponse{
			RequestID:      request.RequestID,
			Document:       doc,
			ProcessingTime: 100 * time.Millisecond,
		}

		select {
		case pipeline.resultChan <- response:
			t.Logf("Sender: Successfully sent response to resultChan")
		case <-time.After(1 * time.Second):
			senderErr = fmt.Errorf("sender timeout")
		}
	}()

	// Simulate the receiver (AnalyzeDocument method)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { receiverComplete = true }()

		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		select {
		case response := <-pipeline.resultChan:
			receivedResponse = response
			t.Logf("Receiver: Successfully received response from resultChan")
		case <-ctx.Done():
			receiverErr = ctx.Err()
			t.Logf("Receiver: Context timeout - %v", receiverErr)
		case <-time.After(3 * time.Second):
			receiverErr = fmt.Errorf("receiver timeout")
		}
	}()

	// Wait for completion
	wg.Wait()

	// Verify results
	if senderErr != nil {
		t.Errorf("Sender failed: %v", senderErr)
	}
	if receiverErr != nil {
		t.Errorf("Receiver failed: %v", receiverErr)
	}
	if !senderComplete {
		t.Error("Sender did not complete")
	}
	if !receiverComplete {
		t.Error("Receiver did not complete")
	}
	if receivedResponse == nil {
		t.Error("No response received")
	} else if receivedResponse.RequestID != request.RequestID {
		t.Errorf("Wrong response received: got %s, expected %s", receivedResponse.RequestID, request.RequestID)
	}

	// Check channel state
	t.Logf("Final channel state: len=%d, cap=%d", len(pipeline.resultChan), cap(pipeline.resultChan))
}

func TestPipelineRaceCondition(t *testing.T) {
	// Test for race conditions with multiple concurrent operations
	ctx := t.Context()
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	numRequests := 10
	var wg sync.WaitGroup
	results := make([]bool, numRequests)

	// Start multiple AnalyzeDocument-like receivers
	for i := range numRequests {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()

			expectedID := fmt.Sprintf("request-%d", requestID)

			select {
			case response := <-pipeline.resultChan:
				if response.RequestID == expectedID {
					results[requestID] = true
					t.Logf("Request %d: Received correct response", requestID)
				} else {
					t.Logf("Request %d: Received wrong response %s", requestID, response.RequestID)
				}
			case <-ctx.Done():
				t.Logf("Request %d: Timeout", requestID)
			}
		}(i)
	}

	// Start multiple senders
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			// Stagger the sends
			time.Sleep(time.Duration(requestID*10) * time.Millisecond)

			response := &AnalysisResponse{
				RequestID: fmt.Sprintf("request-%d", requestID),
			}

			select {
			case pipeline.resultChan <- response:
				t.Logf("Sender %d: Sent response", requestID)
			case <-time.After(1 * time.Second):
				t.Errorf("Sender %d: Send timeout", requestID)
			}
		}(i)
	}

	wg.Wait()

	// Count successful matches
	successful := 0
	for i, success := range results {
		if success {
			successful++
		} else {
			t.Logf("Request %d failed to receive correct response", i)
		}
	}

	t.Logf("Successful request-response matches: %d/%d", successful, numRequests)
}

func TestPipelineContextCancellation(t *testing.T) {
	// Test if context cancellation is interfering with channel operations
	ctx := t.Context()
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	request := &AnalysisRequest{
		RequestID: "context-test-request",
	}

	// Test with short context that expires before result is sent
	t.Run("context_expires_before_result", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		var wg sync.WaitGroup
		var receivedResult bool

		// Start receiver with short context
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-pipeline.resultChan:
				receivedResult = true
			case <-ctx.Done():
				t.Logf("Context expired as expected")
			}
		}()

		// Send result after context should expire
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond) // Longer than context timeout

			response := &AnalysisResponse{
				RequestID: request.RequestID,
			}

			select {
			case pipeline.resultChan <- response:
				t.Logf("Result sent after context expiry")
			case <-time.After(1 * time.Second):
				t.Error("Failed to send result")
			}
		}()

		wg.Wait()

		if receivedResult {
			t.Error("Should not have received result after context expiry")
		}

		// Channel should still have the result
		if len(pipeline.resultChan) != 1 {
			t.Errorf("Expected 1 item in channel, got %d", len(pipeline.resultChan))
		}
	})

	// Test with context that doesn't expire
	t.Run("context_stays_valid", func(t *testing.T) {
		// Clear the channel first
		select {
		case <-pipeline.resultChan:
		default:
		}

		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		var wg sync.WaitGroup
		var receivedResult bool
		var resultRequestID string

		// Start receiver
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case response := <-pipeline.resultChan:
				receivedResult = true
				resultRequestID = response.RequestID
				t.Logf("Received result with valid context")
			case <-ctx.Done():
				t.Logf("Context expired unexpectedly: %v", ctx.Err())
			}
		}()

		// Send result quickly
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond) // Well within context timeout

			response := &AnalysisResponse{
				RequestID: request.RequestID,
			}

			select {
			case pipeline.resultChan <- response:
				t.Logf("Result sent with valid context")
			case <-time.After(1 * time.Second):
				t.Error("Failed to send result")
			}
		}()

		wg.Wait()

		if !receivedResult {
			t.Error("Should have received result with valid context")
		}
		if resultRequestID != request.RequestID {
			t.Errorf("Wrong request ID: got %s, expected %s", resultRequestID, request.RequestID)
		}
	})
}

func TestPipelineChannelOrdering(t *testing.T) {
	// Test if channel operations maintain proper ordering

	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	numMessages := 5
	var wg sync.WaitGroup

	// Send multiple messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			response := &AnalysisResponse{
				RequestID: fmt.Sprintf("ordered-request-%d", id),
			}
			pipeline.resultChan <- response
		}(i)
	}

	wg.Wait()

	// Receive all messages and check we get all of them
	received := make(map[string]bool)
	for i := 0; i < numMessages; i++ {
		select {
		case response := <-pipeline.resultChan:
			received[response.RequestID] = true
			t.Logf("Received: %s", response.RequestID)
		case <-time.After(1 * time.Second):
			t.Errorf("Timeout receiving message %d", i)
		}
	}

	// Verify all messages were received
	for i := 0; i < numMessages; i++ {
		expectedID := fmt.Sprintf("ordered-request-%d", i)
		if !received[expectedID] {
			t.Errorf("Missing message: %s", expectedID)
		}
	}

	// Channel should be empty
	if len(pipeline.resultChan) != 0 {
		t.Errorf("Channel should be empty, has %d items", len(pipeline.resultChan))
	}
}
