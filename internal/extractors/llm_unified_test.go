package extractors

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// Test constants
const (
	testTimeout     = 10 * time.Second
	testMaxRetries  = 3
	testMaxEntities = 100
)

// MockLLMClient for testing
type MockLLMClient struct {
	completeResponse func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error)
	embedResponse    func(ctx context.Context, text string) ([]float64, error)
	healthResponse   func(ctx context.Context) error
	modelInfo        interfaces.ModelInfo
	closeResponse    func() error
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	if m.completeResponse != nil {
		return m.completeResponse(ctx, prompt, options)
	}
	return "", errors.New("mock complete not implemented")
}

func (m *MockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	if m.embedResponse != nil {
		return m.embedResponse(ctx, text)
	}
	return nil, errors.New("mock embed not implemented")
}

func (m *MockLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		embedding, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = embedding
	}
	return results, nil
}

func (m *MockLLMClient) Health(ctx context.Context) error {
	if m.healthResponse != nil {
		return m.healthResponse(ctx)
	}
	return nil
}

func (m *MockLLMClient) GetModelInfo() interfaces.ModelInfo {
	return m.modelInfo
}

func (m *MockLLMClient) Close() error {
	if m.closeResponse != nil {
		return m.closeResponse()
	}
	return nil
}

// Test helper functions
func createTestDocument(id, content string) *types.Document {
	return &types.Document{
		ID:      id,
		Name:    "test-document.txt",
		Content: content,
		Path:    "/test/path",
	}
}

func createTestUnifiedConfig() *interfaces.UnifiedExtractionConfig {
	return &interfaces.UnifiedExtractionConfig{
		Temperature:   0.3,
		MaxTokens:     2048,
		MinConfidence: 0.7,
		MaxEntities:   50,
		MaxCitations:  20,
		MaxTopics:     10,
		AnalysisDepth: "detailed",
	}
}

// Test data for mock responses
const mockClassificationResponse = `{
	"primary_type": "research_paper",
	"confidence": 0.95,
	"reasoning": "Contains abstract, references, and academic language"
}`

const mockUnifiedAnalysisResponse = `{
	"classification": {
		"primary_type": "research_paper",
		"secondary_types": ["article"],
		"confidence": 0.95,
		"features": {
			"has_abstract": 1.0,
			"has_references": 1.0,
			"academic_language": 0.9
		},
		"reasoning": "Contains typical research paper elements"
	},
	"entities": [
		{
			"type": "PERSON",
			"text": "Dr. John Smith",
			"confidence": 0.95,
			"start_offset": 120,
			"end_offset": 133,
			"context": "Dr. John Smith conducted the research",
			"metadata": {"role": "author"}
		},
		{
			"type": "ORGANIZATION",
			"text": "Stanford University",
			"confidence": 0.92,
			"start_offset": 200,
			"end_offset": 219,
			"context": "at Stanford University",
			"metadata": {"type": "institution"}
		}
	],
	"citations": [
		{
			"text": "Smith, J. (2023). Research Methods. Journal Name, 15(2), 123-145.",
			"authors": ["Smith, J."],
			"title": "Research Methods",
			"year": 2023,
			"journal": "Journal Name",
			"volume": "15",
			"issue": "2",
			"pages": "123-145",
			"format": "apa",
			"confidence": 0.95,
			"start_offset": 1500,
			"end_offset": 1580,
			"context": "As referenced in Smith, J. (2023)"
		}
	],
	"topics": [
		{
			"name": "Machine Learning",
			"keywords": ["neural networks", "deep learning", "algorithms"],
			"confidence": 0.9,
			"weight": 0.8,
			"description": "Main research topic focusing on ML techniques"
		},
		{
			"name": "Data Science",
			"keywords": ["data analysis", "statistics", "visualization"],
			"confidence": 0.85,
			"weight": 0.6,
			"description": "Secondary topic related to data analysis"
		}
	],
	"summary": "This research paper discusses machine learning techniques and their applications in data science.",
	"quality_metrics": {
		"confidence_score": 0.92,
		"completeness_score": 0.88,
		"reliability_score": 0.95
	}
}`

// TestNewLLMUnifiedExtractor tests the constructor
func TestNewLLMUnifiedExtractor(t *testing.T) {
	tests := []struct {
		name           string
		config         *interfaces.UnifiedExtractionConfig
		llmClient      interfaces.LLMClient
		expectDefaults bool
	}{
		{
			name:           "with_nil_config",
			config:         nil,
			llmClient:      &MockLLMClient{},
			expectDefaults: true,
		},
		{
			name:           "with_valid_config",
			config:         createTestUnifiedConfig(),
			llmClient:      &MockLLMClient{},
			expectDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewLLMUnifiedExtractor(tt.config, tt.llmClient)

			if extractor == nil {
				t.Fatal("Expected non-nil extractor")
			}

			if extractor.llmClient != tt.llmClient {
				t.Error("LLM client not set correctly")
			}

			if extractor.metrics == nil {
				t.Error("Metrics not initialized")
			}

			if extractor.prompts == nil {
				t.Error("Prompts not initialized")
			}

			if tt.expectDefaults {
				if extractor.config.Temperature != 0.3 {
					t.Errorf("Expected default temperature 0.3, got %f", extractor.config.Temperature)
				}
				if extractor.config.MaxTokens != 2048 {
					t.Errorf("Expected default max tokens 2048, got %d", extractor.config.MaxTokens)
				}
			}
		})
	}
}

// TestLLMUnifiedExtractor_ExtractAll tests the main extraction functionality
func TestLLMUnifiedExtractor_ExtractAll(t *testing.T) {
	tests := []struct {
		name               string
		document           *types.Document
		config             *interfaces.UnifiedExtractionConfig
		mockClassification string
		mockUnified        string
		expectError        bool
		expectedEntities   int
		expectedCitations  int
		expectedTopics     int
	}{
		{
			name:               "successful_extraction",
			document:           createTestDocument("doc1", "This is a research paper about machine learning by Dr. John Smith."),
			config:             createTestUnifiedConfig(),
			mockClassification: mockClassificationResponse,
			mockUnified:        mockUnifiedAnalysisResponse,
			expectError:        false,
			expectedEntities:   2,
			expectedCitations:  1,
			expectedTopics:     2,
		},
		{
			name:               "classification_failure",
			document:           createTestDocument("doc2", "Test content"),
			config:             createTestUnifiedConfig(),
			mockClassification: "", // Will cause parsing error
			mockUnified:        mockUnifiedAnalysisResponse,
			expectError:        false, // Classification failure should fall back to "other"
			expectedEntities:   2,
			expectedCitations:  1,
			expectedTopics:     2,
		},
		{
			name:               "unified_analysis_failure",
			document:           createTestDocument("doc3", "Test content"),
			config:             createTestUnifiedConfig(),
			mockClassification: mockClassificationResponse,
			mockUnified:        "invalid json",
			expectError:        true,
			expectedEntities:   0,
			expectedCitations:  0,
			expectedTopics:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockClient := &MockLLMClient{
				completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
					callCount++
					if callCount == 1 {
						// First call is classification
						return tt.mockClassification, nil
					}
					// Second call is unified analysis
					return tt.mockUnified, nil
				},
				modelInfo: interfaces.ModelInfo{
					Name:     "test-model",
					Provider: "test",
				},
			}

			extractor := NewLLMUnifiedExtractor(tt.config, mockClient)
			ctx := context.Background()

			result, err := extractor.ExtractAll(ctx, tt.document, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if len(result.Entities) != tt.expectedEntities {
				t.Errorf("Expected %d entities, got %d", tt.expectedEntities, len(result.Entities))
			}

			if len(result.Citations) != tt.expectedCitations {
				t.Errorf("Expected %d citations, got %d", tt.expectedCitations, len(result.Citations))
			}

			if len(result.Topics) != tt.expectedTopics {
				t.Errorf("Expected %d topics, got %d", tt.expectedTopics, len(result.Topics))
			}

			// Validate entity data if present
			if len(result.Entities) > 0 {
				entity := result.Entities[0]
				if entity.Type != types.EntityTypePerson {
					t.Errorf("Expected entity type PERSON, got %s", entity.Type)
				}
				if entity.Text != "Dr. John Smith" {
					t.Errorf("Expected entity text 'Dr. John Smith', got '%s'", entity.Text)
				}
				if entity.Confidence != 0.95 {
					t.Errorf("Expected entity confidence 0.95, got %f", entity.Confidence)
				}
			}

			// Validate citation data if present
			if len(result.Citations) > 0 {
				citation := result.Citations[0]
				if len(citation.Authors) != 1 || citation.Authors[0] != "Smith, J." {
					t.Errorf("Expected citation author 'Smith, J.', got %v", citation.Authors)
				}
				if *citation.Title != "Research Methods" {
					t.Errorf("Expected citation title 'Research Methods', got '%s'", *citation.Title)
				}
				if *citation.Year != 2023 {
					t.Errorf("Expected citation year 2023, got %d", *citation.Year)
				}
			}

			// Validate topic data if present
			if len(result.Topics) > 0 {
				topic := result.Topics[0]
				if topic.Name != "Machine Learning" {
					t.Errorf("Expected topic name 'Machine Learning', got '%s'", topic.Name)
				}
				if len(topic.Keywords) != 3 {
					t.Errorf("Expected 3 keywords, got %d", len(topic.Keywords))
				}
				if topic.Confidence != 0.9 {
					t.Errorf("Expected topic confidence 0.9, got %f", topic.Confidence)
				}
			}

			// Validate processing time is set
			if result.ProcessingTime == 0 {
				t.Error("Expected processing time to be set")
			}

			// Validate method is set
			if result.Method != types.ExtractorMethodLLM {
				t.Errorf("Expected method LLM, got %s", result.Method)
			}
		})
	}
}

// TestLLMUnifiedExtractor_GetCapabilities tests the capabilities method
func TestLLMUnifiedExtractor_GetCapabilities(t *testing.T) {
	mockClient := &MockLLMClient{}
	extractor := NewLLMUnifiedExtractor(nil, mockClient)

	capabilities := extractor.GetCapabilities()

	// Check supported entity types
	expectedEntityTypes := []types.EntityType{
		types.EntityTypePerson,
		types.EntityTypeOrganization,
		types.EntityTypeLocation,
		types.EntityTypeDate,
		types.EntityTypeEmail,
		types.EntityTypeConcept,
		types.EntityTypeMetric,
		types.EntityTypeTechnology,
	}

	if len(capabilities.SupportedEntityTypes) != len(expectedEntityTypes) {
		t.Errorf("Expected %d entity types, got %d", len(expectedEntityTypes), len(capabilities.SupportedEntityTypes))
	}

	// Check supported citation formats
	expectedCitationFormats := []types.CitationFormat{
		types.CitationFormatAPA,
		types.CitationFormatMLA,
		types.CitationFormatChicago,
		types.CitationFormatIEEE,
		types.CitationFormatAuto,
	}

	if len(capabilities.SupportedCitationFormats) != len(expectedCitationFormats) {
		t.Errorf("Expected %d citation formats, got %d", len(expectedCitationFormats), len(capabilities.SupportedCitationFormats))
	}

	// Check key capabilities
	if !capabilities.SupportsConfidenceScores {
		t.Error("Expected to support confidence scores")
	}

	if !capabilities.SupportsContext {
		t.Error("Expected to support context")
	}

	if !capabilities.RequiresExternalService {
		t.Error("Expected to require external service")
	}

	if capabilities.MaxDocumentSize != 10*1024*1024 {
		t.Errorf("Expected max document size 10MB, got %d", capabilities.MaxDocumentSize)
	}
}

// TestLLMUnifiedExtractor_GetMetrics tests the metrics functionality
func TestLLMUnifiedExtractor_GetMetrics(t *testing.T) {
	mockClient := &MockLLMClient{
		completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
			return mockClassificationResponse, nil
		},
	}

	extractor := NewLLMUnifiedExtractor(nil, mockClient)
	ctx := context.Background()

	// Initial metrics should be zero
	initialMetrics := extractor.GetMetrics()
	if initialMetrics.TotalDocuments != 0 {
		t.Errorf("Expected initial TotalDocuments to be 0, got %d", initialMetrics.TotalDocuments)
	}

	// Simulate a successful extraction
	mockClient.completeResponse = func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
		// Return classification response for first call, analysis response for second
		if strings.Contains(prompt, "classification") {
			return mockClassificationResponse, nil
		}
		return mockUnifiedAnalysisResponse, nil
	}

	_, err := extractor.ExtractAll(ctx, createTestDocument("test", "content"), createTestUnifiedConfig())
	if err != nil {
		t.Fatalf("Failed to extract document: %v", err)
	}

	// Metrics should be updated
	updatedMetrics := extractor.GetMetrics()
	if updatedMetrics.TotalDocuments != 1 {
		t.Errorf("Expected TotalDocuments to be 1, got %d", updatedMetrics.TotalDocuments)
	}

	if updatedMetrics.ProcessingTime == 0 {
		t.Error("Expected processing time to be greater than 0")
	}
}

// TestLLMUnifiedExtractor_Close tests the close functionality
func TestLLMUnifiedExtractor_Close(t *testing.T) {
	closeCalled := false
	mockClient := &MockLLMClient{
		closeResponse: func() error {
			closeCalled = true
			return nil
		},
	}

	extractor := NewLLMUnifiedExtractor(nil, mockClient)

	err := extractor.Close()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !closeCalled {
		t.Error("Expected LLM client Close() to be called")
	}
}

// TestLLMUnifiedExtractor_mapDocumentType tests document type mapping
func TestLLMUnifiedExtractor_mapDocumentType(t *testing.T) {
	mockClient := &MockLLMClient{}
	extractor := NewLLMUnifiedExtractor(nil, mockClient)

	tests := []struct {
		input    string
		expected types.DocumentType
	}{
		{"research_paper", types.DocumentTypeResearch},
		{"research", types.DocumentTypeResearch},
		{"academic", types.DocumentTypeResearch},
		{"business_document", types.DocumentTypeBusiness},
		{"business", types.DocumentTypeBusiness},
		{"personal_document", types.DocumentTypePersonal},
		{"personal", types.DocumentTypePersonal},
		{"article", types.DocumentTypeArticle},
		{"report", types.DocumentTypeReport},
		{"book", types.DocumentTypeBook},
		{"presentation", types.DocumentTypePresentation},
		{"legal_document", types.DocumentTypeLegal},
		{"legal", types.DocumentTypeLegal},
		{"unknown", types.DocumentTypeOther},
		{"", types.DocumentTypeOther},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := extractor.mapDocumentType(test.input)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkLLMUnifiedExtractor_ExtractAll(b *testing.B) {
	mockClient := &MockLLMClient{
		completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
			if strings.Contains(prompt, "Analyze the following document and classify") {
				return mockClassificationResponse, nil
			}
			return mockUnifiedAnalysisResponse, nil
		},
	}

	extractor := NewLLMUnifiedExtractor(createTestUnifiedConfig(), mockClient)
	document := createTestDocument("bench", "This is a test document for benchmarking performance.")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractAll(ctx, document, nil)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}
