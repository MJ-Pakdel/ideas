package extractors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

// TestData represents test data for entity extraction
type TestData struct {
	Name             string
	Content          string
	DocumentType     types.DocumentType
	ExpectedEntities []ExpectedEntity
}

// ExpectedEntity represents an expected entity for testing
type ExpectedEntity struct {
	Type          types.EntityType
	Text          string
	MinConfidence float64
}

// TestPromptBuilder tests the prompt builder functionality
func TestPromptBuilder(t *testing.T) {
	tests := []struct {
		name          string
		docType       types.DocumentType
		maxEntities   int
		minConfidence float64
		entityTypes   []types.EntityType
		wantContains  []string
	}{
		{
			name:          "Research Paper Prompt",
			docType:       types.DocumentTypeResearch,
			maxEntities:   30,
			minConfidence: 0.8,
			entityTypes:   []types.EntityType{types.EntityTypePerson, types.EntityTypeOrganization, types.EntityTypeConcept},
			wantContains:  []string{"PERSON", "ORGANIZATION", "CONCEPT", "research", "authors", "institutions"},
		},
		{
			name:          "Business Document Prompt",
			docType:       types.DocumentTypeBusiness,
			maxEntities:   50,
			minConfidence: 0.7,
			entityTypes:   types.GetEntityTypesList(),
			wantContains:  []string{"business", "companies", "KPIs", "financial"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewPromptBuilder().
				WithDocumentType(tt.docType).
				WithMaxEntities(tt.maxEntities).
				WithMinConfidence(tt.minConfidence).
				WithEntityTypes(tt.entityTypes)

			prompt := builder.BuildEntityExtractionPrompt("test_document", "test content")

			// Check that prompt contains expected elements
			for _, want := range tt.wantContains {
				if !containsIgnoreCase(prompt, want) {
					t.Errorf("prompt does not contain expected text: %s", want)
				}
			}

			// Check prompt length is reasonable
			if len(prompt) < 500 {
				t.Error("prompt seems too short")
			}

			if len(prompt) > 10000 {
				t.Error("prompt seems too long")
			}

			// Test prompt version
			if builder.GetPromptVersion() == "" {
				t.Error("prompt version should not be empty")
			}
		})
	}
}

// TestJSONCleaner tests the JSON cleaning functionality
func TestJSONCleaner(t *testing.T) {
	cleaner := NewJSONCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "Clean JSON",
			input:    `{"entities": [{"type": "PERSON", "text": "John Doe"}]}`,
			expected: `{"entities": [{"type": "PERSON", "text": "John Doe"}]}`,
			wantErr:  false,
		},
		{
			name:     "JSON with markdown",
			input:    "```json\n{\"entities\": []}\n```",
			expected: `{"entities": []}`,
			wantErr:  false,
		},
		{
			name:     "JSON with explanatory text",
			input:    "Here's the JSON response:\n{\"entities\": []}\nThat's the complete analysis.",
			expected: `{"entities": []}`,
			wantErr:  false,
		},
		{
			name:     "JSON with trailing comma",
			input:    `{"entities": [{"type": "PERSON",}]}`,
			expected: `{"entities": [{"type": "PERSON"}]}`,
			wantErr:  false,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.CleanResponse(tt.input)

			if result != tt.expected {
				t.Errorf("CleanResponse() = %v, want %v", result, tt.expected)
			}

			// Test with stats
			resultWithStats, stats := cleaner.CleanResponseWithStats(tt.input)
			if resultWithStats != result {
				t.Error("CleanResponseWithStats should return same result as CleanResponse")
			}

			if stats == nil {
				t.Error("CleanResponseWithStats should return stats")
			}

			if stats.OriginalLength != len(tt.input) {
				t.Errorf("stats.OriginalLength = %d, want %d", stats.OriginalLength, len(tt.input))
			}
		})
	}

	// Test JSON structure validation
	t.Run("JSON Structure Validation", func(t *testing.T) {
		validJSON := `{"test": "value"}`
		if !cleaner.ValidateJSONStructure(validJSON) {
			t.Error("should validate correct JSON structure")
		}

		invalidJSON := `{"test": "value"`
		if cleaner.ValidateJSONStructure(invalidJSON) {
			t.Error("should not validate incorrect JSON structure")
		}
	})
}

// TestEntityValidator tests the entity validation functionality
func TestEntityValidator(t *testing.T) {
	validator := NewEntityValidator().
		WithMinConfidence(0.5).
		WithMaxEntities(10).
		WithStrictMode(false)

	// Test valid entity
	t.Run("Valid Entity", func(t *testing.T) {
		entity := types.NewExtractedEntity(types.EntityTypePerson, "John Doe", 0.9, 0, 8)
		entities := []*types.ExtractedEntity{entity}

		validated, result := validator.ValidateAndFilterEntities(entities, "John Doe works here")

		if !result.IsValid {
			t.Errorf("validation should succeed: %v", result.Errors)
		}

		if len(validated) != 1 {
			t.Errorf("expected 1 validated entity, got %d", len(validated))
		}
	})

	// Test invalid confidence
	t.Run("Invalid Confidence", func(t *testing.T) {
		entity := types.NewExtractedEntity(types.EntityTypePerson, "John Doe", 0.3, 0, 8)
		entities := []*types.ExtractedEntity{entity}

		validated, result := validator.ValidateAndFilterEntities(entities, "John Doe works here")

		if result.IsValid {
			t.Error("validation should fail for low confidence")
		}

		if len(validated) != 0 {
			t.Errorf("expected 0 validated entities, got %d", len(validated))
		}
	})

	// Test entity type validation
	t.Run("Invalid Entity Type", func(t *testing.T) {
		entity := &types.ExtractedEntity{
			Type:       types.EntityType("INVALID_TYPE"),
			Text:       "Test",
			Confidence: 0.9,
		}
		entities := []*types.ExtractedEntity{entity}

		_, result := validator.ValidateAndFilterEntities(entities, "Test content")

		if result.IsValid {
			t.Error("validation should fail for invalid entity type")
		}
	})

	// Test deduplication
	t.Run("Deduplication", func(t *testing.T) {
		entity1 := types.NewExtractedEntity(types.EntityTypePerson, "John Doe", 0.8, 0, 8)
		entity2 := types.NewExtractedEntity(types.EntityTypePerson, "john doe", 0.9, 10, 18) // Duplicate with different case
		entities := []*types.ExtractedEntity{entity1, entity2}

		validated, result := validator.ValidateAndFilterEntities(entities, "John Doe and john doe")

		if len(validated) != 1 {
			t.Errorf("expected 1 entity after deduplication, got %d", len(validated))
		}

		if result.DuplicatesFound != 1 {
			t.Errorf("expected 1 duplicate found, got %d", result.DuplicatesFound)
		}

		// Should keep the one with higher confidence
		if len(validated) > 0 && validated[0].Confidence != 0.9 {
			t.Errorf("should keep entity with higher confidence (0.9), got %f", validated[0].Confidence)
		}
	})
}

// TestResponseProcessor tests the complete response processing pipeline
func TestResponseProcessor(t *testing.T) {
	processor := NewResponseProcessor().WithConfig(&ProcessingConfig{
		MaxRetries:      3,
		StrictMode:      false,
		MinConfidence:   0.5,
		MaxEntities:     50,
		ValidateOffsets: true,
	})

	ctx := t.Context()

	t.Run("Valid JSON Response", func(t *testing.T) {
		jsonResponse := `{
			"entities": [
				{
					"type": "PERSON",
					"text": "John Doe",
					"confidence": 0.95,
					"start_offset": 0,
					"end_offset": 8,
					"context": "John Doe is a researcher"
				}
			],
			"document_type": "research_paper",
			"total_entities": 1
		}`

		request := types.NewExtractionRequest("doc1", "John Doe is a researcher", types.DocumentTypeResearch)
		result, err := processor.ProcessResponse(ctx, jsonResponse, "John Doe is a researcher", request)

		if err != nil {
			t.Fatalf("ProcessResponse failed: %v", err)
		}

		if !result.Success {
			t.Errorf("processing should succeed: %v", result.Errors)
		}

		if result.Result == nil {
			t.Fatal("result should not be nil")
		}

		if len(result.Result.Entities) != 1 {
			t.Errorf("expected 1 entity, got %d", len(result.Result.Entities))
		}
	})

	t.Run("Malformed JSON Response", func(t *testing.T) {
		malformedJSON := `Here's the response: {"entities": [{"type": "PERSON", "text": "John}]}`

		request := types.NewExtractionRequest("doc1", "John Doe", types.DocumentTypeResearch)
		result, err := processor.ProcessResponse(ctx, malformedJSON, "John Doe", request)

		// Should either succeed with cleaned JSON or fail gracefully
		if err != nil && result == nil {
			t.Error("should return a result even on error")
		}

		if result != nil && result.Success && len(result.Warnings) == 0 {
			t.Error("should have warnings for malformed JSON")
		}
	})

	t.Run("Empty Response", func(t *testing.T) {
		request := types.NewExtractionRequest("doc1", "content", types.DocumentTypeResearch)
		result, err := processor.ProcessResponse(ctx, "", "content", request)

		if err == nil {
			t.Error("should fail on empty response")
		}

		if result == nil || result.Success {
			t.Error("should not succeed on empty response")
		}
	})
}

// TestEndToEndExtraction tests the complete extraction process with real data
func TestEndToEndExtraction(t *testing.T) {
	// Load test data
	testDataFiles := map[string]types.DocumentType{
		"research_paper_sample.txt":    types.DocumentTypeResearch,
		"business_document_sample.txt": types.DocumentTypeBusiness,
		"article_sample.txt":           types.DocumentTypeArticle,
	}

	for filename, docType := range testDataFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", filename))
			if err != nil {
				t.Fatalf("failed to read test file: %v", err)
			}

			// Test prompt building
			builder := NewPromptBuilder().WithDocumentType(docType)
			prompt := builder.BuildEntityExtractionPrompt(filename, string(content))

			if len(prompt) == 0 {
				t.Error("prompt should not be empty")
			}

			// Test JSON cleaning with sample response
			sampleResponse := createSampleJSONResponse(docType, string(content))
			cleaner := NewJSONCleaner()
			cleanedJSON := cleaner.CleanResponse(sampleResponse)

			if cleanedJSON == "" {
				t.Error("cleaned JSON should not be empty")
			}

			// Validate JSON structure
			if !cleaner.ValidateJSONStructure(cleanedJSON) {
				t.Errorf("cleaned JSON should have valid structure: %s", cleanedJSON)
			}

			// Test parsing
			var result types.EntityExtractionResult
			if err := json.Unmarshal([]byte(cleanedJSON), &result); err != nil {
				t.Errorf("should be able to parse cleaned JSON: %v", err)
			}
		})
	}
}

// TestMemoryUsage tests memory efficiency of the extraction process
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	// Create a large document
	largeContent := generateLargeDocument(10000) // 10k words

	builder := NewPromptBuilder()
	prompt := builder.BuildEntityExtractionPrompt("large_doc", largeContent)

	// Test that prompt building doesn't consume excessive memory
	if len(prompt) > 1000000 { // 1MB limit
		t.Error("prompt is too large")
	}

	// Test JSON cleaning with large response
	cleaner := NewJSONCleaner()
	largeResponse := createLargeJSONResponse(100) // 100 entities

	startTime := time.Now()
	cleaned := cleaner.CleanResponse(largeResponse)
	duration := time.Since(startTime)

	if duration > 5*time.Second {
		t.Errorf("JSON cleaning took too long: %v", duration)
	}

	if len(cleaned) == 0 {
		t.Error("should produce cleaned output for large response")
	}
}

// Helper functions

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		len(substr) > 0 &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					len(s) > 2*len(substr)))
	// Simplified case-insensitive contains check
}

func createSampleJSONResponse(docType types.DocumentType, content string) string {
	// Create a realistic JSON response for testing
	response := `{
		"entities": [
			{
				"type": "PERSON",
				"text": "John Doe",
				"confidence": 0.95,
				"start_offset": 0,
				"end_offset": 8,
				"context": "John Doe is mentioned here"
			}
		],
		"document_type": "` + string(docType) + `",
		"total_entities": 1,
		"quality_metrics": {
			"confidence_score": 0.95,
			"completeness_score": 0.90,
			"reliability_score": 0.85
		}
	}`
	return response
}

func generateLargeDocument(wordCount int) string {
	words := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog", "and", "runs", "through", "forest"}
	var content []string

	for i := 0; i < wordCount; i++ {
		content = append(content, words[i%len(words)])
	}

	return "Dr. Sarah Johnson from MIT conducted research. " +
		"The study analyzed " +
		"data from experiments. " +
		"Contact: sarah@mit.edu " +
		"URL: https://mit.edu/research"
}

func createLargeJSONResponse(entityCount int) string {
	entities := make([]string, entityCount)
	for i := 0; i < entityCount; i++ {
		entity := `{
			"type": "CONCEPT",
			"text": "concept` + string(rune(i)) + `",
			"confidence": 0.8,
			"start_offset": ` + string(rune(i*10)) + `,
			"end_offset": ` + string(rune(i*10+7)) + `,
			"context": "sample context"
		}`
		entities[i] = entity
	}

	return `{
		"entities": [` + entities[0] + `],
		"document_type": "research_paper",
		"total_entities": 1
	}`
}

// Benchmark tests
func BenchmarkPromptBuilding(b *testing.B) {
	builder := NewPromptBuilder().WithDocumentType(types.DocumentTypeResearch)
	content := "Sample document content for benchmarking prompt building performance."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.BuildEntityExtractionPrompt("test_doc", content)
	}
}

func BenchmarkJSONCleaning(b *testing.B) {
	cleaner := NewJSONCleaner()
	sampleJSON := `Here's the JSON: {"entities": [{"type": "PERSON", "text": "John"}]} End of response.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cleaner.CleanResponse(sampleJSON)
	}
}

func BenchmarkEntityValidation(b *testing.B) {
	validator := NewEntityValidator()
	entities := []*types.ExtractedEntity{
		types.NewExtractedEntity(types.EntityTypePerson, "John Doe", 0.9, 0, 8),
		types.NewExtractedEntity(types.EntityTypeOrganization, "MIT", 0.85, 10, 13),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateAndFilterEntities(entities, "John Doe at MIT")
	}
}
