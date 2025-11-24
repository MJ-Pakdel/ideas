package analyzers

import (
	"context"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

func TestPipelineSelectDeadlock(t *testing.T) {
	// Test if there's a deadlock or timing issue in the AnalyzeDocument select statement
	ctx := t.Context()
	// Create pipeline exactly like in production
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	doc := &types.Document{
		ID:      "test-deadlock-doc",
		Content: "Test content for deadlock investigation",
	}

	request := &AnalysisRequest{
		Document:  doc,
		RequestID: "test-deadlock-request",
		Priority:  0,
	}

	// Simulate exactly what happens in production
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Track what happens
	var selectResult string
	var responseReceived *AnalysisResponse
	resultSent := false

	// Simulate sending the result (like pipeline stages do)
	go func() {
		time.Sleep(100 * time.Millisecond) // Small delay like in production

		response := &AnalysisResponse{
			RequestID:      request.RequestID,
			Document:       doc,
			ProcessingTime: 100 * time.Millisecond,
		}

		t.Logf("Sending result to resultChan")
		pipeline.resultChan <- response
		resultSent = true
		t.Logf("Result sent successfully")
	}()

	// Simulate the AnalyzeDocument select statement
	select {
	case response := <-pipeline.resultChan:
		selectResult = "received_response"
		responseReceived = response
		t.Logf("Successfully received response from resultChan")
	case <-ctx.Done():
		selectResult = "context_timeout"
		t.Logf("Context timeout: %v", ctx.Err())
	case <-time.After(5 * time.Second):
		selectResult = "test_timeout"
		t.Logf("Test timeout after 5 seconds")
	}

	// Verify results
	if !resultSent {
		t.Error("Result was not sent")
	}

	if selectResult != "received_response" {
		t.Errorf("Expected to receive response, but got: %s", selectResult)
	}

	if responseReceived == nil {
		t.Error("No response received")
	} else if responseReceived.RequestID != request.RequestID {
		t.Errorf("Wrong response received: got %s, expected %s", responseReceived.RequestID, request.RequestID)
	}

	// Check final channel state
	t.Logf("Final channel state: len=%d, cap=%d", len(pipeline.resultChan), cap(pipeline.resultChan))
}

func TestContextCancellationTiming(t *testing.T) {
	// Test if context cancellation timing is interfering with channel operations
	ctx := t.Context()
	pipeline := &AnalysisPipeline{
		resultChan: make(chan *AnalysisResponse, 30),
	}

	request := &AnalysisRequest{
		RequestID: "context-timing-test",
	}

	// Test with context that should NOT expire
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Send result quickly, well before context expires
	go func() {
		time.Sleep(50 * time.Millisecond)
		response := &AnalysisResponse{
			RequestID: request.RequestID,
		}
		t.Logf("Sending result to channel")
		pipeline.resultChan <- response
		t.Logf("Result sent")
	}()

	// This should work without context interference
	select {
	case response := <-pipeline.resultChan:
		if response.RequestID != request.RequestID {
			t.Errorf("Wrong response: got %s, expected %s", response.RequestID, request.RequestID)
		} else {
			t.Logf("SUCCESS: Received correct response")
		}
	case <-ctx.Done():
		t.Errorf("Unexpected context cancellation: %v", ctx.Err())
	}
}
