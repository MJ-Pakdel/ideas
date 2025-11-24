package extractors

import (
	"strings"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

// TestChunkedDocumentProcessingIntegration tests processing of large documents with chunking
func TestChunkedDocumentProcessingIntegration(t *testing.T) {
	ctx := t.Context()

	// Create a large document that will require chunking
	largeContent := ""
	for i := 0; i < 10; i++ {
		largeContent += "Dr. John Smith works at MIT Research Laboratory. MIT is a leading research institution focused on artificial intelligence and machine learning. John Smith published numerous papers on AI and deep learning algorithms. AI research at MIT focuses on neural networks, natural language processing, and computer vision. Dr. Smith's recent work explores advanced neural network architectures and their applications in real-world scenarios. "
	}

	// Create optimized extractor with chunking enabled
	mockExtractor := NewMockEntityExtractor([]*types.Entity{
		{
			ID:         "entity_1",
			Type:       types.EntityTypePerson,
			Text:       "Dr. John Smith",
			Confidence: 0.9,
			Method:     types.ExtractorMethodLLM,
		},
		{
			ID:         "entity_2",
			Type:       types.EntityTypeOrganization,
			Text:       "MIT Research Laboratory",
			Confidence: 0.8,
			Method:     types.ExtractorMethodLLM,
		},
	}, 5*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = true
	config.ChunkingConfig.MaxChunkSize = 500 // Force chunking
	config.MaxConcurrentChunks = 2

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	input := types.ExtractionInput{
		DocumentID:   "large-doc-test",
		DocumentName: "large_document.txt",
		Content:      largeContent,
		DocumentType: types.DocumentTypeResearch,
	}

	result, err := optimizedExtractor.ExtractEntities(ctx, input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	// Validate that chunking occurred
	if optimizedExtractor.metrics.ChunksProcessed <= 1 {
		t.Errorf("Expected multiple chunks to be processed, got %d", optimizedExtractor.metrics.ChunksProcessed)
	}

	// Validate entities were extracted from chunks
	if len(result.Entities) == 0 {
		t.Error("No entities extracted from chunked document")
	}

	// Validate processing time is reasonable
	if optimizedExtractor.metrics.ProcessingTime > 100*time.Millisecond {
		t.Errorf("Processing time too high: %v", optimizedExtractor.metrics.ProcessingTime)
	}

	t.Logf("Chunked processing completed: %d chunks, %d entities, %v processing time",
		optimizedExtractor.metrics.ChunksProcessed, len(result.Entities), optimizedExtractor.metrics.ProcessingTime)
}

// TestErrorRecoveryPipelineIntegration tests error handling and recovery in the pipeline
func TestErrorRecoveryPipelineIntegration(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name           string
		errorCondition string
		expectError    bool
	}{
		{
			name:           "Empty Content",
			errorCondition: "empty",
			expectError:    false, // Should handle gracefully
		},
		{
			name:           "Very Long Content",
			errorCondition: "long",
			expectError:    false, // Should chunk and process
		},
		{
			name:           "Special Characters",
			errorCondition: "special",
			expectError:    false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content string
			switch tt.errorCondition {
			case "empty":
				content = ""
			case "long":
				content = strings.Repeat("This is a very long document with many repeated sentences that should test the chunking and memory management capabilities of the system. ", 1000)
			case "special":
				content = "Text with special chars: €£¥§©®™ and emojis 🚀🔬💡 and unicode: αβγδε"
			}

			mockExtractor := NewMockEntityExtractor([]*types.Entity{}, 1*time.Millisecond)

			config := DefaultOptimizationConfig()
			config.EnableChunking = true
			optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

			input := types.ExtractionInput{
				DocumentID:   "error-test-" + tt.name,
				DocumentName: "error_test.txt",
				Content:      content,
				DocumentType: types.DocumentTypeReport,
			}

			result, err := optimizedExtractor.ExtractEntities(ctx, input)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && result == nil {
				t.Error("Expected result but got nil")
			}

			t.Logf("Error recovery test %s: handled gracefully", tt.name)
		})
	}
}

// TestFullProcessingPipelineIntegration tests the complete processing pipeline with different document types
func TestFullProcessingPipelineIntegration(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name         string
		documentType types.DocumentType
		content      string
		minEntities  int
	}{
		{
			name:         "Research Paper",
			documentType: types.DocumentTypeResearch,
			content:      "Dr. Sarah Chen from Stanford University published research on machine learning algorithms. The paper demonstrates significant improvements in neural network efficiency.",
			minEntities:  2, // At least Dr. Sarah Chen and Stanford University
		},
		{
			name:         "Business Report",
			documentType: types.DocumentTypeBusiness,
			content:      "Acme Corporation reported strong quarterly results. CEO John Smith announced expansion plans for California and Texas markets.",
			minEntities:  3, // At least Acme Corporation, John Smith, California
		},
		{
			name:         "Technical Article",
			documentType: types.DocumentTypeArticle,
			content:      "The latest developments in artificial intelligence include advances in natural language processing and computer vision. Companies like Google and Microsoft are leading this innovation.",
			minEntities:  2, // At least Google and Microsoft
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock entities relevant to the content
			mockEntities := []*types.Entity{
				{
					ID:         "entity_1",
					Type:       types.EntityTypePerson,
					Text:       "Dr. Sarah Chen",
					Confidence: 0.9,
					Method:     types.ExtractorMethodLLM,
				},
				{
					ID:         "entity_2",
					Type:       types.EntityTypeOrganization,
					Text:       "Stanford University",
					Confidence: 0.8,
					Method:     types.ExtractorMethodLLM,
				},
				{
					ID:         "entity_3",
					Type:       types.EntityTypeOrganization,
					Text:       "Acme Corporation",
					Confidence: 0.9,
					Method:     types.ExtractorMethodLLM,
				},
			}

			mockExtractor := NewMockEntityExtractor(mockEntities, 2*time.Millisecond)
			config := DefaultOptimizationConfig()
			optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

			input := types.ExtractionInput{
				DocumentID:   "pipeline-test-" + tt.name,
				DocumentName: tt.name + ".txt",
				Content:      tt.content,
				DocumentType: tt.documentType,
			}

			result, err := optimizedExtractor.ExtractEntities(ctx, input)
			if err != nil {
				t.Fatalf("ExtractEntities failed: %v", err)
			}

			// Validate minimum entity count
			if len(result.Entities) < tt.minEntities {
				t.Errorf("Expected at least %d entities, got %d", tt.minEntities, len(result.Entities))
			}

			// Validate result structure
			if result.DocumentID != input.DocumentID {
				t.Errorf("Expected DocumentID %s, got %s", input.DocumentID, result.DocumentID)
			}

			if result.DocumentType != input.DocumentType {
				t.Errorf("Expected DocumentType %s, got %s", input.DocumentType, result.DocumentType)
			}

			// Validate timing
			if optimizedExtractor.metrics.ProcessingTime == 0 {
				t.Error("Processing time should be recorded")
			}

			t.Logf("Pipeline test %s: %d entities extracted in %v",
				tt.name, len(result.Entities), optimizedExtractor.metrics.ProcessingTime)
		})
	}
}
