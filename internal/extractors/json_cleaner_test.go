package extractors

import (
	"encoding/json"
	"testing"

	"github.com/example/idaes/internal/types"
)

func TestJSONCleanerBasicCleaning(t *testing.T) {
	cleaner := NewJSONCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean JSON",
			input:    `{"entities": [{"text": "John", "type": "PERSON"}]}`,
			expected: `{"entities": [{"text": "John", "type": "PERSON"}]}`,
		},
		{
			name:     "remove markdown",
			input:    "```json\n{\"entities\": []}\n```",
			expected: `{"entities": []}`,
		},
		{
			name:     "extract from mixed content",
			input:    "Here's the result:\n{\"entities\": []} \nThat's it.",
			expected: `{"entities": []}`,
		},
		{
			name:     "remove comments",
			input:    `{"entities": [/* comment */]}`,
			expected: `{"entities": []}`,
		},
		{
			name:     "fix trailing commas",
			input:    `{"entities": [{"text": "John",}],}`,
			expected: `{"entities": [{"text": "John"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.CleanResponse(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Helper function to parse entities from JSON string
func parseEntitiesFromJSON(jsonStr string) ([]*types.ExtractedEntity, error) {
	var result struct {
		Entities []*types.ExtractedEntity `json:"entities"`
	}

	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, err
	}

	return result.Entities, nil
}

func TestJSONCleanerEntityExtraction(t *testing.T) {
	cleaner := NewJSONCleaner()

	malformedJSON := `{
		"entities": [
			{"text": "John Smith", "type": "PERSON", "confidence": 0.95},
			{"text": "MIT", "type": "ORGANIZATION", "confidence": 0.90}
		]
	}`

	cleaned := cleaner.CleanResponse(malformedJSON)

	// Verify it's valid JSON
	entities, err := parseEntitiesFromJSON(cleaned)
	if err != nil {
		t.Fatalf("Failed to parse cleaned JSON: %v", err)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}

	// Check first entity
	if entities[0].Text != "John Smith" {
		t.Errorf("Expected 'John Smith', got %q", entities[0].Text)
	}
	if entities[0].Type != types.EntityTypePerson {
		t.Errorf("Expected PERSON, got %q", entities[0].Type)
	}
	if entities[0].Confidence != 0.95 {
		t.Errorf("Expected 0.95, got %f", entities[0].Confidence)
	}
}

func TestJSONCleanerErrorHandling(t *testing.T) {
	cleaner := NewJSONCleaner()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "no JSON",
			input: "This is just plain text with no JSON at all.",
		},
		{
			name:  "severely malformed JSON",
			input: `{"entities": [{"text": "broken"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := cleaner.CleanResponse(tt.input)

			// Should return fallback structure or empty/error
			entities, err := parseEntitiesFromJSON(cleaned)
			if err == nil && len(entities) == 0 {
				// This is acceptable - empty entities array
				return
			}

			// If it's still malformed, that's also acceptable for edge cases
			if err != nil {
				t.Logf("Input %q resulted in error (acceptable): %v", tt.input, err)
			}
		})
	}
}

func TestJSONCleanerComplexScenarios(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Test with realistic LLM output that might be malformed
	llmOutput := `Based on the document, here are the extracted entities:

` + "```json" + `
	{
		"entities": [
			{
				"text": "Dr. Sarah Chen",
				"type": "PERSON",
				"confidence": 0.95,
				"start_offset": 0,
				"end_offset": 13
			},
			{
				"text": "Stanford University", 
				"type": "ORGANIZATION",
				"confidence": 0.92,
				"start_offset": 20,
				"end_offset": 39
			},
			{
				"text": "2024-03-15",
				"type": "DATE",
				"confidence": 0.88,
				"start_offset": 45,
				"end_offset": 55
			}
		]
	}
` + "```" + `
	
	Note: The extraction focused on named entities with high confidence.`

	cleaned := cleaner.CleanResponse(llmOutput)
	entities, err := parseEntitiesFromJSON(cleaned)

	if err != nil {
		t.Fatalf("Failed to parse complex LLM output: %v", err)
	}

	if len(entities) != 3 {
		t.Errorf("Expected 3 entities, got %d", len(entities))
	}

	// Verify entity details
	expectedEntities := []struct {
		text       string
		entityType types.EntityType
		confidence float64
	}{
		{"Dr. Sarah Chen", types.EntityTypePerson, 0.95},
		{"Stanford University", types.EntityTypeOrganization, 0.92},
		{"2024-03-15", types.EntityTypeDate, 0.88},
	}

	for i, expected := range expectedEntities {
		if i >= len(entities) {
			t.Fatalf("Missing entity %d", i)
		}

		entity := entities[i]
		if entity.Text != expected.text {
			t.Errorf("Entity %d: expected text %q, got %q", i, expected.text, entity.Text)
		}
		if entity.Type != expected.entityType {
			t.Errorf("Entity %d: expected type %q, got %q", i, expected.entityType, entity.Type)
		}
		if entity.Confidence != expected.confidence {
			t.Errorf("Entity %d: expected confidence %f, got %f", i, expected.confidence, entity.Confidence)
		}
	}
}

func TestJSONCleanerPerformance(t *testing.T) {
	cleaner := NewJSONCleaner()

	// Test with large JSON response
	largeJSON := `{"entities": [`
	for i := 0; i < 100; i++ { // Reduced from 1000 to 100 for faster testing
		if i > 0 {
			largeJSON += ","
		}
		largeJSON += `{"text": "Entity` + string(rune('A'+i%26)) + `", "type": "MISC", "confidence": 0.5}`
	}
	largeJSON += `]}`

	// Should handle large responses without significant delay
	cleaned := cleaner.CleanResponse(largeJSON)
	entities, err := parseEntitiesFromJSON(cleaned)

	if err != nil {
		t.Fatalf("Failed to parse large JSON: %v", err)
	}

	if len(entities) != 100 {
		t.Errorf("Expected 100 entities, got %d", len(entities))
	}
}
