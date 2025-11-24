package analyzers

import (
	"testing"
	"time"
)

func TestPipelineSelection(t *testing.T) {
	// Test pipeline selection logic with multiple pipelines
	ctx := t.Context()
	// Create orchestrator config
	cfg := &OrchestratorConfig{
		MaxWorkers:    2,
		LoadBalancing: "round_robin",
		QueueSize:     10,
	}

	orchestrator := NewAnalysisOrchestrator(cfg)
	defer orchestrator.Stop(ctx)

	// Mock add pipelines to test selection
	mockPipeline1 := &AnalysisPipeline{}
	mockPipeline2 := &AnalysisPipeline{}
	orchestrator.pipelines = []*AnalysisPipeline{mockPipeline1, mockPipeline2}

	// Start the orchestrator to enable coordination goroutine
	err := orchestrator.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Track which pipeline instances are selected
	selectedPipelines := make(map[*AnalysisPipeline]int)

	// Test multiple selections to see round-robin behavior
	for i := 0; i < 6; i++ {
		selectedPipeline := orchestrator.selectPipeline()
		if selectedPipeline != nil {
			selectedPipelines[selectedPipeline]++
			t.Logf("Selection %d: pipeline instance %p", i, selectedPipeline)
		}
	}

	t.Logf("Selected pipeline distribution: %v", selectedPipelines)

	// With round-robin, we should see both pipelines selected
	if len(selectedPipelines) != 2 {
		t.Errorf("Expected 2 different pipelines selected, got %d", len(selectedPipelines))
	}
}

func TestSinglePipelineConsistency(t *testing.T) {
	// Test with only one pipeline to verify consistency
	ctx := t.Context()
	cfg := &OrchestratorConfig{
		MaxWorkers:    1,
		LoadBalancing: "round_robin",
		QueueSize:     10,
	}

	orchestrator := NewAnalysisOrchestrator(cfg)
	defer orchestrator.Stop(ctx)

	// Add single mock pipeline
	mockPipeline := &AnalysisPipeline{}
	orchestrator.pipelines = []*AnalysisPipeline{mockPipeline}

	// Start the orchestrator to enable coordination goroutine
	err := orchestrator.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Multiple selections should return the same instance
	pipeline1 := orchestrator.selectPipeline()
	pipeline2 := orchestrator.selectPipeline()
	pipeline3 := orchestrator.selectPipeline()

	if pipeline1 != pipeline2 || pipeline2 != pipeline3 {
		t.Errorf("Single pipeline test failed: got different instances %p, %p, %p", pipeline1, pipeline2, pipeline3)
	} else {
		t.Logf("Single pipeline test passed: all selections returned %p", pipeline1)
	}
}

func TestPipelineSelectionTimeout(t *testing.T) {
	// Test that pipeline selection doesn't hang when orchestrator is shutting down
	ctx := t.Context()
	cfg := &OrchestratorConfig{
		MaxWorkers:    1,
		LoadBalancing: "round_robin",
		QueueSize:     10,
	}

	orchestrator := NewAnalysisOrchestrator(cfg)

	// Add mock pipeline
	mockPipeline := &AnalysisPipeline{}
	orchestrator.pipelines = []*AnalysisPipeline{mockPipeline}

	// Start shutdown before selecting pipeline
	go func() {
		time.Sleep(100 * time.Millisecond)
		orchestrator.Stop(ctx)
	}()

	// This should return nil due to shutdown
	selectedPipeline := orchestrator.selectPipeline()
	if selectedPipeline != nil {
		t.Errorf("Expected nil pipeline after shutdown, got %p", selectedPipeline)
	} else {
		t.Log("Pipeline selection correctly returned nil after shutdown")
	}
}
