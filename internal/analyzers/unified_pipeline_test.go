package analyzers

import (
	"context"
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// MockUnifiedExtractor for testing unified extraction
type MockUnifiedExtractor struct {
	shouldFail bool
	result     *interfaces.UnifiedExtractionResult
}

func (m *MockUnifiedExtractor) ExtractAll(ctx context.Context, document *types.Document, config *interfaces.UnifiedExtractionConfig) (*interfaces.UnifiedExtractionResult, error) {
	if m.shouldFail {
		return nil, &testError{message: "mock unified extraction failed"}
	}

	if m.result != nil {
		return m.result, nil
	}

	// Default mock result
	return &interfaces.UnifiedExtractionResult{
		Entities: []*types.Entity{
			{
				ID:          "unified-entity-1",
				Text:        "Unified Entity",
				Type:        types.EntityTypePerson,
				Confidence:  0.9,
				StartOffset: 0,
				EndOffset:   14,
				DocumentID:  document.ID,
				Method:      types.ExtractorMethodLLM,
			},
		},
		Citations: []*types.Citation{
			{
				ID:          "unified-citation-1",
				Text:        "Unified Citation (2023)",
				Format:      types.CitationFormatAPA,
				Confidence:  0.85,
				StartOffset: 15,
				EndOffset:   40,
				DocumentID:  document.ID,
				Method:      types.ExtractorMethodLLM,
			},
		},
		Topics: []*types.Topic{
			{
				ID:         "unified-topic-1",
				Name:       "Unified Topic",
				Confidence: 0.8,
				Weight:     0.75,
				DocumentID: document.ID,
				Method:     string(types.ExtractorMethodLLM),
			},
		},
		QualityMetrics: &interfaces.QualityMetrics{
			ConfidenceScore:   0.85,
			CompletenessScore: 0.9,
			ReliabilityScore:  0.8,
		},
		ExtractionMetadata: interfaces.ExtractionMetadata{
			ProcessingTime: time.Millisecond * 100,
			Method:         types.ExtractorMethodLLM,
			Confidence:     0.85,
			Metadata: map[string]any{
				"unified": true,
			},
		},
	}, nil
}

func (m *MockUnifiedExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes:     []types.EntityType{types.EntityTypePerson, types.EntityTypeOrganization},
		SupportedCitationFormats: []types.CitationFormat{types.CitationFormatAPA, types.CitationFormatMLA},
		SupportedDocumentTypes:   []types.DocumentType{types.DocumentTypeResearch, types.DocumentTypeArticle},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		RequiresExternalService:  true,
	}
}

func (m *MockUnifiedExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return interfaces.ExtractorMetrics{
		ProcessingTime:    time.Millisecond * 100,
		AverageConfidence: 0.85,
	}
}

func (m *MockUnifiedExtractor) Close() error {
	return nil
}

// testError for testing error scenarios
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

func TestUnifiedExtractionPipeline_Success(t *testing.T) {
	ctx := t.Context()

	// Create pipeline with unified extraction enabled
	config := &AnalysisConfig{
		UseUnifiedExtraction:     true,  // Enable unified extraction for this test
		UnifiedFallbackEnabled:   false, // Don't fallback to individual extractors
		EnableEntityExtraction:   true,
		EnableCitationExtraction: true,
		EnableTopicExtraction:    true,
		StoreResults:             false, // Disable storage for test
		MinConfidenceScore:       0.6,
		MaxConcurrentAnalyses:    6, // Ensure enough workers for all stages
		AnalysisTimeout:          time.Second * 5,
	}

	pipeline := NewAnalysisPipeline(ctx, nil, nil, config)
	defer func() {
		_ = pipeline.Shutdown(ctx)
	}()

	// Register mock unified extractor
	mockExtractor := &MockUnifiedExtractor{shouldFail: false}
	pipeline.RegisterUnifiedExtractor(mockExtractor)

	// Create test document
	document := types.NewDocument(
		"/test/unified.txt",
		"Unified Test Document",
		"This is a test document for unified extraction. It contains entities and citations.",
		"text/plain",
	)

	// Create analysis request
	request := &AnalysisRequest{
		RequestID: "test-unified-success",
		Document:  document,
	}

	// Analyze document
	response := pipeline.AnalyzeDocument(ctx, request)

	// Verify response
	if response == nil {
		t.Fatal("expected non-nil response")
	}

	if response.Error != nil {
		t.Fatalf("expected no error, got: %v", response.Error)
	}

	// Verify unified extraction was used
	if len(response.Result.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(response.Result.Entities))
	}

	if len(response.Result.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(response.Result.Citations))
	}

	if len(response.Result.Topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(response.Result.Topics))
	}

	// Verify extractor method was recorded
	if method, exists := response.ExtractorUsed["entities"]; !exists || method != types.ExtractorMethodLLM {
		t.Errorf("expected entities extractor method to be LLM, got %v", method)
	}

	if method, exists := response.ExtractorUsed["citations"]; !exists || method != types.ExtractorMethodLLM {
		t.Errorf("expected citations extractor method to be LLM, got %v", method)
	}

	if method, exists := response.ExtractorUsed["topics"]; !exists || method != types.ExtractorMethodLLM {
		t.Errorf("expected topics extractor method to be LLM, got %v", method)
	}

	// Verify entities
	if len(response.Result.Entities) > 0 {
		entity := response.Result.Entities[0]
		if entity.Text != "Unified Entity" {
			t.Errorf("expected entity text 'Unified Entity', got '%s'", entity.Text)
		}
		if entity.Type != types.EntityTypePerson {
			t.Errorf("expected entity type Person, got %s", entity.Type)
		}
	}
}

func TestUnifiedExtractionPipeline_FallbackOnFailure(t *testing.T) {
	ctx := t.Context()

	// Create pipeline with unified extraction enabled and fallback enabled
	config := &AnalysisConfig{
		UseUnifiedExtraction:     true,
		UnifiedFallbackEnabled:   true, // Enable fallback
		EnableEntityExtraction:   true,
		EnableCitationExtraction: false, // Disable to simplify test
		EnableTopicExtraction:    false, // Disable to simplify test
		MinConfidenceScore:       0.6,
		MaxConcurrentAnalyses:    6, // Ensure enough workers for all stages
		AnalysisTimeout:          time.Second * 5,
		PreferredEntityMethod:    types.ExtractorMethodLLM,
	}

	pipeline := NewAnalysisPipeline(ctx, nil, nil, config)
	defer func() {
		_ = pipeline.Shutdown(ctx)
	}()

	// Register failing unified extractor
	mockExtractor := &MockUnifiedExtractor{shouldFail: true}
	pipeline.RegisterUnifiedExtractor(mockExtractor)

	// Create test document
	document := types.NewDocument(
		"/test/fallback.txt",
		"Fallback Test Document",
		"This is a test document for fallback testing.",
		"text/plain",
	)

	// Create analysis request
	request := &AnalysisRequest{
		RequestID: "test-unified-fallback",
		Document:  document,
	}

	// Analyze document
	response := pipeline.AnalyzeDocument(ctx, request)

	// Verify response
	if response == nil {
		t.Fatal("expected non-nil response")
	}

	// Should succeed with fallback (but no actual extractors registered, so will fail)
	// This test demonstrates that the fallback logic is triggered
	if response.Error == nil {
		t.Error("expected fallback to attempt individual extraction (which should fail since no extractors registered)")
	}
}

func TestUnifiedExtractionPipeline_DisabledMode(t *testing.T) {
	ctx := t.Context()

	// Create pipeline with unified extraction DISABLED
	config := &AnalysisConfig{
		UseUnifiedExtraction:     false, // Unified disabled
		UnifiedFallbackEnabled:   false,
		EnableEntityExtraction:   true,
		EnableCitationExtraction: false,
		EnableTopicExtraction:    false,
		MinConfidenceScore:       0.6,
		MaxConcurrentAnalyses:    6, // Ensure enough workers for all stages
		AnalysisTimeout:          time.Second * 5,
		PreferredEntityMethod:    types.ExtractorMethodLLM,
	}

	pipeline := NewAnalysisPipeline(ctx, nil, nil, config)
	defer func() {
		_ = pipeline.Shutdown(ctx)
	}()

	// Register unified extractor (but it shouldn't be used)
	mockExtractor := &MockUnifiedExtractor{shouldFail: false}
	pipeline.RegisterUnifiedExtractor(mockExtractor)

	// Create test document
	document := types.NewDocument(
		"/test/disabled.txt",
		"Disabled Mode Test Document",
		"This should use individual extractors.",
		"text/plain",
	)

	// Create analysis request
	request := &AnalysisRequest{
		RequestID: "test-unified-disabled",
		Document:  document,
	}

	// Analyze document
	response := pipeline.AnalyzeDocument(ctx, request)

	// Verify response
	if response == nil {
		t.Fatal("expected non-nil response")
	}

	// Should attempt individual extraction (will fail since no individual extractors registered)
	// This verifies that unified extraction was NOT used
	if response.Error == nil {
		t.Error("expected individual extraction to fail since no individual extractors registered")
	}

	// Important: verify that unified extractor was not called by checking
	// that no results were produced (since the mock would have produced results)
	if len(response.Result.Entities) > 0 {
		t.Error("unified extractor should not have been used when disabled")
	}
}
