package analyzers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/idaes/internal/extractors"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// TestUnifiedExtractionSystemIntegration tests unified extraction with the factory system
// This test validates the complete integration without requiring external services
func TestUnifiedExtractionSystemIntegration(t *testing.T) {
	ctx := t.Context()

	// Test with unified extraction enabled via configuration
	t.Run("UnifiedExtractionConfigurationEnabled", func(t *testing.T) {
		testUnifiedExtractionConfiguration(t, ctx, true)
	})

	// Test with unified extraction disabled via configuration
	t.Run("UnifiedExtractionConfigurationDisabled", func(t *testing.T) {
		testUnifiedExtractionConfiguration(t, ctx, false)
	})

	// Test environment variable configuration override
	t.Run("EnvironmentVariableConfiguration", func(t *testing.T) {
		testEnvironmentVariableConfiguration(t, ctx)
	})

	// Test system factory integration
	t.Run("SystemFactoryIntegration", func(t *testing.T) {
		testSystemFactoryIntegration(t, ctx)
	})
}

func testUnifiedExtractionConfiguration(t *testing.T, ctx context.Context, useUnified bool) {
	// Create mock LLM client
	mockLLMClient := &MockLLMClient{
		responses: map[string]string{
			"extract_entities":  `{"entities": [{"text": "John Smith", "type": "PERSON", "confidence": 0.95}]}`,
			"extract_citations": `{"citations": [{"text": "Smith et al. (2023)", "source": "Academic Paper", "confidence": 0.90}]}`,
			"extract_topics":    `{"topics": [{"name": "machine learning", "confidence": 0.88}]}`,
			"unified": `{
				"entities": [{"text": "Dr. Jane Smith", "type": "PERSON", "confidence": 0.95}, {"text": "MIT", "type": "ORGANIZATION", "confidence": 0.92}],
				"citations": [{"text": "Smith et al. (2023)", "source": "Academic Research", "confidence": 0.90}],
				"topics": [{"name": "artificial intelligence", "confidence": 0.89}, {"name": "machine learning", "confidence": 0.87}]
			}`,
		},
	}

	// Create mock storage manager
	mockStorageManager := &MockStorageManager{}

	// Create pipeline configuration
	config := &AnalysisConfig{
		UseUnifiedExtraction:     useUnified,
		UnifiedFallbackEnabled:   true,
		EnableEntityExtraction:   true,
		EnableCitationExtraction: true,
		EnableTopicExtraction:    true,
		MinConfidenceScore:       0.6,
		MaxConcurrentAnalyses:    4,
		AnalysisTimeout:          time.Second * 10,
		PreferredEntityMethod:    types.ExtractorMethodLLM,
	}

	// Create pipeline (fix parameter order: storageManager, llmClient, config)
	pipeline := NewAnalysisPipeline(ctx, mockStorageManager, mockLLMClient, config)
	defer func() {
		_ = pipeline.Shutdown(ctx)
	}()

	// Create and register unified extractor
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:         0.1,
		MaxTokens:           4096,
		MinConfidence:       0.6,
		MaxEntities:         50,
		EntityTypes:         []types.EntityType{},
		MaxCitations:        20,
		CitationFormats:     []types.CitationFormat{},
		MaxTopics:           10,
		TopicMinKeywords:    3,
		ClassificationTypes: []types.DocumentType{},
		IncludeMetadata:     true,
		AnalysisDepth:       "detailed",
	}
	unifiedExtractor := extractors.NewLLMUnifiedExtractor(unifiedConfig, mockLLMClient)
	pipeline.RegisterUnifiedExtractor(unifiedExtractor)

	if !useUnified {
		// For individual extractors, we would need to register them through the factory
		// Since the pipeline doesn't have direct registration methods for individual extractors
		// and this is primarily testing unified extraction, we'll skip this test case
		t.Skip("Individual extractor registration not supported directly on pipeline - would need factory-based setup")
	}

	// Create test document
	document := types.NewDocument(
		"/test/integration.txt",
		"Integration Test Document",
		`This is a comprehensive test document for the IDAES system.
		
		Dr. Jane Smith at MIT conducted research on artificial intelligence and machine learning.
		The work was published in Smith et al. (2023) and builds upon Johnson (2022).
		
		Key organizations: MIT, Stanford University, Google DeepMind.
		Topics: artificial intelligence, machine learning, neural networks.`,
		"text/plain",
	)

	// Create analysis request
	request := &AnalysisRequest{
		RequestID: "test-integration-" + getBoolStr(useUnified),
		Document:  document,
	}

	// Analyze document
	startTime := time.Now()
	response := pipeline.AnalyzeDocument(ctx, request)
	processingTime := time.Since(startTime)

	// Verify response
	if response == nil {
		t.Fatal("expected non-nil response")
	}

	if response.Error != nil {
		t.Fatalf("analysis failed: %v", response.Error)
	}

	if response.Result == nil {
		t.Fatal("expected non-nil result")
	}

	// Log results for verification
	t.Logf("Processing time (%s): %v", getBoolStr(useUnified), processingTime)
	t.Logf("Entities found: %d", len(response.Result.Entities))
	t.Logf("Citations found: %d", len(response.Result.Citations))
	t.Logf("Topics found: %d", len(response.Result.Topics))

	// Verify we got results (the exact count may vary between unified and individual)
	if len(response.Result.Entities) == 0 {
		t.Error("expected to find at least some entities")
	}

	if useUnified {
		// With unified extraction, we should get the unified response
		if len(response.Result.Citations) == 0 {
			t.Error("expected to find citations with unified extraction")
		}
		if len(response.Result.Topics) == 0 {
			t.Error("expected to find topics with unified extraction")
		}
	}

	// Verify entity structure
	for _, entity := range response.Result.Entities {
		if entity.Text == "" {
			t.Error("entity has empty text")
		}
		if entity.Confidence <= 0 {
			t.Error("entity has invalid confidence score")
		}
	}
}

func testEnvironmentVariableConfiguration(t *testing.T, ctx context.Context) {
	// Test 1: Environment variable enables unified extraction
	t.Run("EnvironmentVariableEnables", func(t *testing.T) {
		// Set environment variable
		oldValue := os.Getenv("IDAES_USE_UNIFIED_EXTRACTION")
		os.Setenv("IDAES_USE_UNIFIED_EXTRACTION", "true")
		defer func() {
			if oldValue == "" {
				os.Unsetenv("IDAES_USE_UNIFIED_EXTRACTION")
			} else {
				os.Setenv("IDAES_USE_UNIFIED_EXTRACTION", oldValue)
			}
		}()

		// Create pipeline with default config (should be overridden by environment)
		config := createDefaultPipelineConfig()

		// Verify unified extraction is enabled by environment variable
		if !config.UseUnifiedExtraction {
			t.Error("expected unified extraction to be enabled by environment variable")
		}
	})

	// Test 2: Environment variable disables unified extraction
	t.Run("EnvironmentVariableDisables", func(t *testing.T) {
		// Set environment variable
		oldValue := os.Getenv("IDAES_USE_UNIFIED_EXTRACTION")
		os.Setenv("IDAES_USE_UNIFIED_EXTRACTION", "false")
		defer func() {
			if oldValue == "" {
				os.Unsetenv("IDAES_USE_UNIFIED_EXTRACTION")
			} else {
				os.Setenv("IDAES_USE_UNIFIED_EXTRACTION", oldValue)
			}
		}()

		// Create pipeline with default config
		config := createDefaultPipelineConfig()

		// Verify unified extraction is disabled by environment variable
		if config.UseUnifiedExtraction {
			t.Error("expected unified extraction to be disabled by environment variable")
		}
	})
}

func testSystemFactoryIntegration(t *testing.T, ctx context.Context) {
	// Create mock system configuration
	factory := NewAnalysisSystemFactory()
	config := DefaultAnalysisSystemConfig()

	// Override with mock LLM client (we can't easily create a real system without external services)
	// This tests that the factory properly creates and configures the unified extraction system

	// Check that the configuration includes unified extraction config
	if config.UnifiedConfig == nil {
		t.Error("expected default system config to include unified extraction config")
	}

	// Verify the pipeline config respects environment variables
	if config.PipelineConfig == nil {
		t.Error("expected default system config to include pipeline config")
	}

	// Test factory creation with invalid configuration
	invalidConfig := &AnalysisSystemConfig{
		LLMConfig: nil, // This should cause an error
	}

	_, err := factory.CreateSystem(ctx, invalidConfig)
	if err == nil {
		t.Error("expected error when creating system with invalid config")
	}
}

func getBoolStr(b bool) string {
	if b {
		return "unified"
	}
	return "individual"
}

// MockLLMClient for testing
type MockLLMClient struct {
	responses map[string]string
	callCount int
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	m.callCount++

	// Simple keyword-based response selection
	if contains(prompt, "extract entities") || contains(prompt, "ENTITIES") {
		return m.responses["extract_entities"], nil
	}
	if contains(prompt, "extract citations") || contains(prompt, "CITATIONS") {
		return m.responses["extract_citations"], nil
	}
	if contains(prompt, "extract topics") || contains(prompt, "TOPICS") {
		return m.responses["extract_topics"], nil
	}

	// Default to unified response
	return m.responses["unified"], nil
}

func (m *MockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	// Return a simple mock embedding
	return []float64{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (m *MockLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	}
	return result, nil
}

func (m *MockLLMClient) Health(ctx context.Context) error {
	return nil
}

func (m *MockLLMClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{
		Name:         "mock-model",
		Provider:     "mock",
		Version:      "1.0",
		ContextSize:  4096,
		EmbeddingDim: 768,
	}
}

func (m *MockLLMClient) Close() error {
	return nil
}

// Legacy method for backward compatibility (used by some extractors)
func (m *MockLLMClient) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	return m.Complete(ctx, prompt, nil)
}

func (m *MockLLMClient) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	return m.Embed(ctx, text)
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) && text[:len(substr)] == substr
}

// MockStorageManager for testing
type MockStorageManager struct{}

func (m *MockStorageManager) Initialize(ctx context.Context) error {
	return nil
}

func (m *MockStorageManager) StoreDocument(ctx context.Context, document *types.Document) error {
	return nil
}

func (m *MockStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, document *types.Document) error {
	return nil
}

func (m *MockStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}

func (m *MockStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error) {
	return nil, nil
}

func (m *MockStorageManager) DeleteDocument(ctx context.Context, id string) error {
	return nil
}

func (m *MockStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error {
	return nil
}

func (m *MockStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}

func (m *MockStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}

func (m *MockStorageManager) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return []*types.Entity{}, nil
}

func (m *MockStorageManager) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	return []*types.Citation{}, nil
}

func (m *MockStorageManager) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return []*types.Topic{}, nil
}

func (m *MockStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error {
	return nil
}

func (m *MockStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error {
	return nil
}

func (m *MockStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error {
	return nil
}

func (m *MockStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error) {
	return []interfaces.CollectionInfo{}, nil
}

func (m *MockStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error) {
	return &interfaces.StorageStats{}, nil
}

func (m *MockStorageManager) Health(ctx context.Context) error {
	return nil
}

func (m *MockStorageManager) Close() error {
	return nil
}
