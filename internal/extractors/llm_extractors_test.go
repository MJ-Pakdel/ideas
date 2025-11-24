package extractors

import (
	"context"
	"testing"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// TestNewLLMEntityExtractor tests the entity extractor constructor
func TestNewLLMEntityExtractor(t *testing.T) {
	tests := []struct {
		name           string
		config         *interfaces.EntityExtractionConfig
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
			name: "with_valid_config",
			config: &interfaces.EntityExtractionConfig{
				Enabled:       true,
				Method:        types.ExtractorMethodLLM,
				MinConfidence: 0.8,
				MaxEntities:   25,
			},
			llmClient:      &MockLLMClient{},
			expectDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewLLMEntityExtractor(tt.config, tt.llmClient)

			if extractor == nil {
				t.Fatal("Expected non-nil extractor")
			}

			if extractor.llmClient != tt.llmClient {
				t.Error("LLM client not set correctly")
			}

			if extractor.metrics == nil {
				t.Error("Metrics not initialized")
			}

			if extractor.unifiedExtractor == nil {
				t.Error("Unified extractor not initialized")
			}

			if tt.expectDefaults {
				if extractor.config.MinConfidence != 0.7 {
					t.Errorf("Expected default min confidence 0.7, got %f", extractor.config.MinConfidence)
				}
				if extractor.config.MaxEntities != 50 {
					t.Errorf("Expected default max entities 50, got %d", extractor.config.MaxEntities)
				}
			} else {
				if extractor.config.MinConfidence != 0.8 {
					t.Errorf("Expected configured min confidence 0.8, got %f", extractor.config.MinConfidence)
				}
				if extractor.config.MaxEntities != 25 {
					t.Errorf("Expected configured max entities 25, got %d", extractor.config.MaxEntities)
				}
			}
		})
	}
}

// TestLLMEntityExtractor_Extract tests entity extraction functionality
func TestLLMEntityExtractor_Extract(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name             string
		document         *types.Document
		config           *interfaces.EntityExtractionConfig
		mockResponse     string
		expectError      bool
		expectedEntities int
	}{
		{
			name:     "successful_entity_extraction",
			document: createTestDocument("doc1", "Dr. John Smith from Stanford University conducted research."),
			config: &interfaces.EntityExtractionConfig{
				Enabled:       true,
				Method:        types.ExtractorMethodLLM,
				MinConfidence: 0.7,
				MaxEntities:   10,
			},
			mockResponse:     mockUnifiedAnalysisResponse,
			expectError:      false,
			expectedEntities: 2, // Dr. John Smith and Stanford University
		},
		{
			name:     "extraction_with_low_confidence_filter",
			document: createTestDocument("doc2", "Test content"),
			config: &interfaces.EntityExtractionConfig{
				Enabled:       true,
				Method:        types.ExtractorMethodLLM,
				MinConfidence: 0.99, // Very high threshold
				MaxEntities:   10,
			},
			mockResponse:     mockUnifiedAnalysisResponse,
			expectError:      false,
			expectedEntities: 0, // All entities filtered out
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockClient := &MockLLMClient{
				completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
					callCount++
					if callCount == 1 {
						return mockClassificationResponse, nil
					}
					return tt.mockResponse, nil
				},
			}

			extractor := NewLLMEntityExtractor(tt.config, mockClient)

			result, err := extractor.Extract(ctx, tt.document, tt.config)

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

			// Validate result metadata
			if result.Method != types.ExtractorMethodLLM {
				t.Errorf("Expected method LLM, got %s", result.Method)
			}

			if result.ProcessingTime == 0 {
				t.Error("Expected processing time to be set")
			}

			// Validate entity structure if present
			if len(result.Entities) > 0 {
				entity := result.Entities[0]
				if entity.ID == "" {
					t.Error("Expected entity ID to be set")
				}
				if entity.DocumentID != tt.document.ID {
					t.Errorf("Expected entity document ID %s, got %s", tt.document.ID, entity.DocumentID)
				}
				if entity.ExtractedAt.IsZero() {
					t.Error("Expected entity extraction time to be set")
				}
			}
		})
	}
}

// TestLLMEntityExtractor_GetCapabilities tests entity extractor capabilities
func TestLLMEntityExtractor_GetCapabilities(t *testing.T) {
	mockClient := &MockLLMClient{}
	extractor := NewLLMEntityExtractor(nil, mockClient)

	capabilities := extractor.GetCapabilities()

	// Check that it supports entity extraction
	if len(capabilities.SupportedEntityTypes) == 0 {
		t.Error("Expected to support some entity types")
	}

	// Check that it doesn't support citation formats (entity extractor specific)
	if len(capabilities.SupportedCitationFormats) != 0 {
		t.Error("Entity extractor should not support citation formats")
	}

	if !capabilities.SupportsConfidenceScores {
		t.Error("Expected to support confidence scores")
	}

	if !capabilities.RequiresExternalService {
		t.Error("Expected to require external service")
	}
}

// TestLLMEntityExtractor_Validate tests configuration validation
func TestLLMEntityExtractor_Validate(t *testing.T) {
	mockClient := &MockLLMClient{}
	extractor := NewLLMEntityExtractor(nil, mockClient)

	tests := []struct {
		name        string
		config      *interfaces.EntityExtractionConfig
		expectError bool
	}{
		{
			name:        "nil_config",
			config:      nil,
			expectError: true,
		},
		{
			name: "valid_config",
			config: &interfaces.EntityExtractionConfig{
				MinConfidence: 0.7,
				MaxEntities:   50,
			},
			expectError: false,
		},
		{
			name: "invalid_min_confidence_low",
			config: &interfaces.EntityExtractionConfig{
				MinConfidence: -0.1,
				MaxEntities:   50,
			},
			expectError: true,
		},
		{
			name: "invalid_min_confidence_high",
			config: &interfaces.EntityExtractionConfig{
				MinConfidence: 1.1,
				MaxEntities:   50,
			},
			expectError: true,
		},
		{
			name: "invalid_max_entities",
			config: &interfaces.EntityExtractionConfig{
				MinConfidence: 0.7,
				MaxEntities:   0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := extractor.Validate(tt.config)

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

// TestLLMCitationExtractor_Extract tests citation extraction functionality
func TestLLMCitationExtractor_Extract(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name              string
		document          *types.Document
		config            *interfaces.CitationExtractionConfig
		mockResponse      string
		expectError       bool
		expectedCitations int
	}{
		{
			name:     "successful_citation_extraction",
			document: createTestDocument("doc1", "According to Smith, J. (2023), machine learning is important."),
			config: &interfaces.CitationExtractionConfig{
				Enabled:       true,
				Method:        types.ExtractorMethodLLM,
				MinConfidence: 0.7,
				MaxCitations:  10,
			},
			mockResponse:      mockUnifiedAnalysisResponse,
			expectError:       false,
			expectedCitations: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockClient := &MockLLMClient{
				completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
					callCount++
					if callCount == 1 {
						return mockClassificationResponse, nil
					}
					return tt.mockResponse, nil
				},
			}

			extractor := NewLLMCitationExtractor(tt.config, mockClient)

			result, err := extractor.Extract(ctx, tt.document, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result.Citations) != tt.expectedCitations {
				t.Errorf("Expected %d citations, got %d", tt.expectedCitations, len(result.Citations))
			}

			// Validate citation structure if present
			if len(result.Citations) > 0 {
				citation := result.Citations[0]
				if citation.ID == "" {
					t.Error("Expected citation ID to be set")
				}
				if citation.DocumentID != tt.document.ID {
					t.Errorf("Expected citation document ID %s, got %s", tt.document.ID, citation.DocumentID)
				}
			}
		})
	}
}

// TestLLMTopicExtractor_Extract tests topic extraction functionality
func TestLLMTopicExtractor_Extract(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		document       *types.Document
		config         *interfaces.TopicExtractionConfig
		mockResponse   string
		expectError    bool
		expectedTopics int
	}{
		{
			name:     "successful_topic_extraction",
			document: createTestDocument("doc1", "This paper discusses machine learning and data science techniques."),
			config: &interfaces.TopicExtractionConfig{
				Enabled:       true,
				Method:        "llm",
				NumTopics:     5,
				MinConfidence: 0.7,
			},
			mockResponse:   mockUnifiedAnalysisResponse,
			expectError:    false,
			expectedTopics: 2, // Machine Learning and Data Science
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockClient := &MockLLMClient{
				completeResponse: func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
					callCount++
					if callCount == 1 {
						return mockClassificationResponse, nil
					}
					return tt.mockResponse, nil
				},
			}

			extractor := NewLLMTopicExtractor(tt.config, mockClient)

			result, err := extractor.Extract(ctx, tt.document, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result.Topics) != tt.expectedTopics {
				t.Errorf("Expected %d topics, got %d", tt.expectedTopics, len(result.Topics))
			}

			// Validate topic structure if present
			if len(result.Topics) > 0 {
				topic := result.Topics[0]
				if topic.ID == "" {
					t.Error("Expected topic ID to be set")
				}
				if topic.Name == "" {
					t.Error("Expected topic name to be set")
				}
				if len(topic.Keywords) == 0 {
					t.Error("Expected topic keywords to be set")
				}
			}
		})
	}
}
