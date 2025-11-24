package interfaces

import (
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

func TestUnifiedAnalysisAdapter_ConversionMethods(t *testing.T) {
	adapter := NewUnifiedAnalysisAdapter()

	// Create sample unified result
	unifiedResult := &UnifiedExtractionResult{
		Entities: []*types.Entity{
			{
				Text: "John Doe",
				Type: types.EntityTypePerson,
			},
		},
		Citations: []*types.Citation{
			{
				Text:   "Doe, J. (2023). Test paper.",
				Format: types.CitationFormatAPA,
			},
		},
		Topics: []*types.Topic{
			{
				Name: "Testing",
			},
		},
		Classification: &types.DocumentClassification{
			PrimaryType: types.DocumentTypeResearch,
			Confidence:  0.9,
		},
		ExtractionMetadata: ExtractionMetadata{
			ProcessingTime: time.Second,
			Method:         types.ExtractorMethodLLM,
			Confidence:     0.85,
		},
	}

	// Test entity extraction
	entityResult := adapter.ToEntityExtractionResult(unifiedResult)
	if entityResult == nil {
		t.Fatal("entity result is nil")
	}
	if len(entityResult.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entityResult.Entities))
	}
	if entityResult.Entities[0].Text != "John Doe" {
		t.Errorf("expected entity text 'John Doe', got '%s'", entityResult.Entities[0].Text)
	}

	// Test citation extraction
	citationResult := adapter.ToCitationExtractionResult(unifiedResult)
	if citationResult == nil {
		t.Fatal("citation result is nil")
	}
	if len(citationResult.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(citationResult.Citations))
	}

	// Test topic extraction
	topicResult := adapter.ToTopicExtractionResult(unifiedResult)
	if topicResult == nil {
		t.Fatal("topic result is nil")
	}
	if len(topicResult.Topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topicResult.Topics))
	}

	// Test classification extraction
	classificationResult := adapter.ToDocumentClassificationResult(unifiedResult)
	if classificationResult == nil {
		t.Fatal("classification result is nil")
	}
	if classificationResult.Classification.PrimaryType != types.DocumentTypeResearch {
		t.Errorf("expected primary type research_paper, got %s", classificationResult.Classification.PrimaryType)
	}
}
