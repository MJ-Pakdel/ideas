package extractors

import (
	"context"
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// MockEntityExtractor for testing the optimized extractor
type MockEntityExtractor struct {
	entities []*types.Entity
	delay    time.Duration
}

func NewMockEntityExtractor(entities []*types.Entity, delay time.Duration) *MockEntityExtractor {
	return &MockEntityExtractor{
		entities: entities,
		delay:    delay,
	}
}

func (m *MockEntityExtractor) Extract(ctx context.Context, document *types.Document, config *interfaces.EntityExtractionConfig) (*interfaces.EntityExtractionResult, error) {
	// Simulate processing time
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Return mock entities
	return &interfaces.EntityExtractionResult{
		Entities: m.entities,
		ExtractionMetadata: interfaces.ExtractionMetadata{
			ProcessingTime: m.delay,
			Method:         types.ExtractorMethodLLM,
			Confidence:     0.85,
			Metadata:       make(map[string]any),
			CacheHit:       false,
		},
	}, nil
}

func (m *MockEntityExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes:     []types.EntityType{types.EntityTypePerson, types.EntityTypeOrganization},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		MaxDocumentSize:          10000,
		AverageProcessingTime:    m.delay,
	}
}

func (m *MockEntityExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return interfaces.ExtractorMetrics{
		ProcessingTime: m.delay,
	}
}

func (m *MockEntityExtractor) Validate(config *interfaces.EntityExtractionConfig) error {
	return nil
}

func (m *MockEntityExtractor) Close() error {
	return nil
}

func TestOptimizedExtractorSingleDocument(t *testing.T) {
	// Create mock entities
	mockEntities := []*types.Entity{
		{
			ID:          "entity_1",
			Type:        types.EntityTypePerson,
			Text:        "Dr. John Smith",
			Confidence:  0.9,
			StartOffset: 0,
			EndOffset:   13,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
		{
			ID:          "entity_2",
			Type:        types.EntityTypeOrganization,
			Text:        "MIT",
			Confidence:  0.8,
			StartOffset: 25,
			EndOffset:   28,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 10*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = false // Test single document processing

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	input := types.ExtractionInput{
		DocumentID:   "test_doc",
		DocumentName: "test_document.txt",
		Content:      "Dr. John Smith works at MIT Research Laboratory.",
		DocumentType: types.DocumentTypeResearch,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
			types.EntityTypeOrganization,
		},
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if len(result.Entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(result.Entities))
	}

	// Check that metrics were collected
	metrics := optimizedExtractor.GetMetrics()
	if metrics.ProcessingTime == 0 {
		t.Error("Processing time not recorded")
	}

	if metrics.EntitiesExtracted != 2 {
		t.Errorf("Expected 2 entities extracted in metrics, got %d", metrics.EntitiesExtracted)
	}

	t.Logf("Processing took: %v", metrics.ProcessingTime)
	t.Logf("Extracted %d entities", metrics.EntitiesExtracted)
}

func TestOptimizedExtractorChunkedDocument(t *testing.T) {
	// Create mock entities that would be found in different chunks
	mockEntities := []*types.Entity{
		{
			ID:          "entity_1",
			Type:        types.EntityTypePerson,
			Text:        "Dr. John Smith",
			Confidence:  0.9,
			StartOffset: 0,
			EndOffset:   13,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 5*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = true
	config.ChunkingConfig.Strategy = ChunkingOverlapping // Force overlapping strategy
	config.ChunkingConfig.MaxChunkSize = 50              // Force chunking
	config.EnableCrossReferencing = true
	config.EnableEntityMerging = true

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	// Long content that will be chunked
	longContent := `Dr. John Smith works at MIT Research Laboratory. MIT is a prestigious institution that conducts cutting-edge research in artificial intelligence and machine learning. Dr. Smith has published numerous papers on neural networks and deep learning algorithms. His work at MIT focuses on advancing the state of the art in AI research and developing practical applications for machine learning technologies.`

	input := types.ExtractionInput{
		DocumentID:   "test_doc_chunked",
		DocumentName: "test_document_long.txt",
		Content:      longContent,
		DocumentType: types.DocumentTypeResearch,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
			types.EntityTypeOrganization,
		},
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Check that chunking was used
	metrics := optimizedExtractor.GetMetrics()
	if metrics.ChunksProcessed < 2 {
		t.Errorf("Expected at least 2 chunks processed, got %d", metrics.ChunksProcessed)
	}

	// Check metadata for chunking information
	if chunkingEnabled, exists := result.ExtractionMetadata["chunking_enabled"]; !exists || !chunkingEnabled.(bool) {
		t.Error("Chunking metadata not found or disabled")
	}

	if chunksProcessed, exists := result.ExtractionMetadata["chunks_processed"]; !exists || chunksProcessed.(int) < 2 {
		t.Error("Chunks processed metadata not found or incorrect")
	}

	t.Logf("Processed %d chunks", metrics.ChunksProcessed)
	t.Logf("Total processing time: %v", metrics.ProcessingTime)
	t.Logf("Cross-references found: %d", metrics.CrossReferencesFound)
}

func TestOptimizedExtractorCrossReferencing(t *testing.T) {
	// Create entities that should trigger cross-referencing
	mockEntities := []*types.Entity{
		{
			ID:          "entity_mit",
			Type:        types.EntityTypeOrganization,
			Text:        "MIT",
			Confidence:  0.8,
			StartOffset: 0,
			EndOffset:   3,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 1*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = true
	config.ChunkingConfig.Strategy = ChunkingOverlapping // Force overlapping strategy
	config.ChunkingConfig.MaxChunkSize = 20              // Very small chunks to force multiple chunks
	config.EnableCrossReferencing = true
	config.ChunkingConfig.EnableEntityMapping = true

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	// Content with repeated entities across chunks
	content := `MIT researchers study AI. Dr. Smith leads MIT AI lab. AI advances at MIT continue rapidly. MIT publishes breakthrough research.`

	input := types.ExtractionInput{
		DocumentID:   "test_cross_ref",
		DocumentName: "cross_ref_test.txt",
		Content:      content,
		DocumentType: types.DocumentTypeResearch,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypeOrganization,
		},
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	metrics := optimizedExtractor.GetMetrics()

	// We should have multiple chunks due to repeated "MIT" entities
	if metrics.ChunksProcessed < 2 {
		t.Errorf("Expected multiple chunks, got %d", metrics.ChunksProcessed)
	}

	// Check if cross-referencing metadata is present
	if crossRefEnabled, exists := result.ExtractionMetadata["cross_referencing"]; !exists || !crossRefEnabled.(bool) {
		t.Error("Cross-referencing metadata not found or disabled")
	}

	t.Logf("Cross-references found: %d", metrics.CrossReferencesFound)
	t.Logf("Chunks processed: %d", metrics.ChunksProcessed)
}

func TestOptimizedExtractorMemoryLimits(t *testing.T) {
	mockEntities := []*types.Entity{
		{
			ID:          "entity_1",
			Type:        types.EntityTypePerson,
			Text:        "Test Entity",
			Confidence:  0.7,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 1*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.MaxMemoryUsage = 1 // Very low memory limit - 1 byte

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	input := types.ExtractionInput{
		DocumentID:   "test_memory",
		DocumentName: "memory_test.txt",
		Content:      "Short content",
		DocumentType: types.DocumentTypeReport,
	}

	// This should fail due to memory limits
	_, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err == nil {
		t.Error("Expected memory limit error, but extraction succeeded")
	} else {
		t.Logf("Got expected error: %v", err) // This is expected
	}
}

func TestOptimizedExtractorEntityMerging(t *testing.T) {
	// Create duplicate entities that should be merged
	mockEntities := []*types.Entity{
		{
			ID:          "entity_1",
			Type:        types.EntityTypePerson,
			Text:        "John Smith",
			Confidence:  0.7,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
		{
			ID:          "entity_2",
			Type:        types.EntityTypePerson,
			Text:        "John Smith", // Duplicate
			Confidence:  0.9,          // Higher confidence
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 1*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = true
	config.ChunkingConfig.MaxChunkSize = 30 // Force multiple chunks
	config.EnableEntityMerging = true

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	content := `John Smith works at the company. John Smith is the lead researcher on the project.`

	input := types.ExtractionInput{
		DocumentID:   "test_merge",
		DocumentName: "merge_test.txt",
		Content:      content,
		DocumentType: types.DocumentTypeReport,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
		},
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	// Should have merged duplicates - expecting fewer entities than extracted
	metrics := optimizedExtractor.GetMetrics()
	totalExtracted := metrics.ChunksProcessed * len(mockEntities)

	if len(result.Entities) >= totalExtracted {
		t.Errorf("Expected entity merging to reduce count, got %d entities from %d total extractions",
			len(result.Entities), totalExtracted)
	}

	t.Logf("Chunks processed: %d", metrics.ChunksProcessed)
	t.Logf("Final entities after merging: %d", len(result.Entities))
}

func TestOptimizedExtractorConfidenceFiltering(t *testing.T) {
	// Create entities with different confidence levels
	mockEntities := []*types.Entity{
		{
			ID:          "entity_high",
			Type:        types.EntityTypePerson,
			Text:        "High Confidence",
			Confidence:  0.9, // Above threshold
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
		{
			ID:          "entity_low",
			Type:        types.EntityTypePerson,
			Text:        "Low Confidence",
			Confidence:  0.1, // Below threshold
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 1*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.ConfidenceThreshold = 0.5 // Filter out low confidence entities
	config.EnableChunking = false

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	input := types.ExtractionInput{
		DocumentID:   "test_confidence",
		DocumentName: "confidence_test.txt",
		Content:      "High Confidence person and Low Confidence person.",
		DocumentType: types.DocumentTypeReport,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
		},
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	// Should only have the high confidence entity
	if len(result.Entities) != 1 {
		t.Errorf("Expected 1 entity after confidence filtering, got %d", len(result.Entities))
	}

	if len(result.Entities) > 0 && result.Entities[0].Confidence < config.ConfidenceThreshold {
		t.Errorf("Entity with confidence %f should have been filtered out", result.Entities[0].Confidence)
	}

	t.Logf("Entities after confidence filtering: %d", len(result.Entities))
}

func TestOptimizedExtractorMetrics(t *testing.T) {
	mockEntities := []*types.Entity{
		{
			ID:          "entity_1",
			Type:        types.EntityTypePerson,
			Text:        "Test Person",
			Confidence:  0.8,
			DocumentID:  "test_doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}

	mockExtractor := NewMockEntityExtractor(mockEntities, 10*time.Millisecond)

	config := DefaultOptimizationConfig()
	config.EnableChunking = true
	config.ChunkingConfig.MaxChunkSize = 50

	optimizedExtractor := NewOptimizedExtractor(mockExtractor, config)

	// Reset metrics
	optimizedExtractor.Reset()

	initialMetrics := optimizedExtractor.GetMetrics()
	if initialMetrics.ProcessingTime != 0 {
		t.Error("Metrics not properly reset")
	}

	content := `Test Person works on research. Test Person publishes papers regularly.`

	input := types.ExtractionInput{
		DocumentID:   "test_metrics",
		DocumentName: "metrics_test.txt",
		Content:      content,
		DocumentType: types.DocumentTypeReport,
	}

	result, err := optimizedExtractor.ExtractEntities(context.Background(), input)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	finalMetrics := optimizedExtractor.GetMetrics()

	// Check that metrics were updated
	if finalMetrics.ProcessingTime == 0 {
		t.Error("Processing time not recorded")
	}

	if finalMetrics.EntitiesExtracted == 0 {
		t.Error("Entities extracted count not recorded")
	}

	if finalMetrics.ChunksProcessed == 0 {
		t.Error("Chunks processed count not recorded")
	}

	// Check metadata in result
	if result.ExtractionMetadata == nil {
		t.Fatal("Extraction metadata is nil")
	}

	expectedMetadataKeys := []string{
		"chunks_processed",
		"entities_extracted",
		"processing_time_ms",
		"chunking_enabled",
		"cross_referencing",
		"average_confidence",
	}

	for _, key := range expectedMetadataKeys {
		if _, exists := result.ExtractionMetadata[key]; !exists {
			t.Errorf("Missing metadata key: %s", key)
		}
	}

	t.Logf("Final metrics: %+v", finalMetrics)
}
