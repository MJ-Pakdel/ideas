package extractors

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// OptimizedExtractor implements memory-efficient entity extraction for edge hardware
type OptimizedExtractor struct {
	baseExtractor interfaces.EntityExtractor
	chunker       *IntelligentChunker

	// Memory management
	memoryLimit        int64
	currentMemoryUsage int64
	memoryMutex        sync.RWMutex

	// Performance tracking
	metrics *ExtractionMetrics

	// Configuration
	config OptimizationConfig
}

// OptimizationConfig contains settings for edge hardware optimization
type OptimizationConfig struct {
	// Memory settings
	MaxMemoryUsage int64          `json:"max_memory_usage"` // In bytes
	EnableChunking bool           `json:"enable_chunking"`  // Whether to use chunking
	ChunkingConfig ChunkingConfig `json:"chunking_config"`

	// Performance settings
	MaxConcurrentChunks int  `json:"max_concurrent_chunks"` // Parallel processing limit
	EnableCompression   bool `json:"enable_compression"`    // Compress intermediate results
	EnableCaching       bool `json:"enable_caching"`        // Cache results for reuse

	// Quality settings
	EnableCrossReferencing bool    `json:"enable_cross_referencing"` // Link entities across chunks
	ConfidenceThreshold    float64 `json:"confidence_threshold"`     // Minimum confidence for results
	EnableEntityMerging    bool    `json:"enable_entity_merging"`    // Merge duplicate entities

	// Hardware-specific
	CPUOptimization    bool `json:"cpu_optimization"`    // Enable CPU-specific optimizations
	MemoryOptimization bool `json:"memory_optimization"` // Enable memory-specific optimizations
	IOOptimization     bool `json:"io_optimization"`     // Enable I/O optimizations
}

// DefaultOptimizationConfig returns optimal settings for Raspberry Pi 5
func DefaultOptimizationConfig() OptimizationConfig {
	return OptimizationConfig{
		MaxMemoryUsage:         500 * 1024 * 1024, // 500MB limit for Pi 5
		EnableChunking:         true,
		ChunkingConfig:         DefaultChunkingConfig(),
		MaxConcurrentChunks:    2, // Conservative for Pi 5's 4 cores
		EnableCompression:      true,
		EnableCaching:          true,
		EnableCrossReferencing: true,
		ConfidenceThreshold:    0.3, // Lower threshold for edge hardware
		EnableEntityMerging:    true,
		CPUOptimization:        true,
		MemoryOptimization:     true,
		IOOptimization:         true,
	}
}

// ExtractionMetrics tracks performance for optimization
type ExtractionMetrics struct {
	ProcessingTime       time.Duration `json:"processing_time"`
	MemoryUsage          int64         `json:"memory_usage"`
	ChunksProcessed      int           `json:"chunks_processed"`
	EntitiesExtracted    int           `json:"entities_extracted"`
	CrossReferencesFound int           `json:"cross_references_found"`
	CacheHits            int           `json:"cache_hits"`
	CacheMisses          int           `json:"cache_misses"`
	RetryAttempts        int           `json:"retry_attempts"`
	FallbacksUsed        int           `json:"fallbacks_used"`
}

// ChunkResult contains extraction results for a single chunk
type ChunkResult struct {
	ChunkID         string           `json:"chunk_id"`
	ChunkIndex      int              `json:"chunk_index"`
	Entities        []*types.Entity  `json:"entities"`
	Confidence      float64          `json:"confidence"`
	ProcessingTime  time.Duration    `json:"processing_time"`
	MemoryUsed      int64            `json:"memory_used"`
	CrossReferences []CrossReference `json:"cross_references,omitempty"`
	Error           error            `json:"error,omitempty"`
}

// CrossReference represents a reference between entities across chunks
type CrossReference struct {
	SourceEntityID   string  `json:"source_entity_id"`
	TargetEntityID   string  `json:"target_entity_id"`
	RelationType     string  `json:"relation_type"`
	Confidence       float64 `json:"confidence"`
	SourceChunkIndex int     `json:"source_chunk_index"`
	TargetChunkIndex int     `json:"target_chunk_index"`
	Context          string  `json:"context"`
}

// NewOptimizedExtractor creates a new optimized extractor for edge hardware
func NewOptimizedExtractor(baseExtractor interfaces.EntityExtractor, config OptimizationConfig) *OptimizedExtractor {
	chunker := NewIntelligentChunker(config.ChunkingConfig)

	return &OptimizedExtractor{
		baseExtractor:      baseExtractor,
		chunker:            chunker,
		memoryLimit:        config.MaxMemoryUsage,
		currentMemoryUsage: 0,
		config:             config,
		metrics:            &ExtractionMetrics{},
	}
}

// ExtractEntities performs optimized entity extraction with chunking and cross-referencing
func (oe *OptimizedExtractor) ExtractEntities(ctx context.Context, input types.ExtractionInput) (*types.EntityExtractionResult, error) {
	startTime := time.Now()
	defer func() {
		oe.metrics.ProcessingTime = time.Since(startTime)
	}()

	// Estimate initial memory usage for this operation
	inputMemoryEstimate := int64(len(input.Content) + len(input.DocumentName) + 1024) // Base overhead
	oe.updateMemoryUsage(inputMemoryEstimate)
	defer oe.updateMemoryUsage(-inputMemoryEstimate) // Clean up when done

	// Check memory limits before processing
	if err := oe.checkMemoryLimits(); err != nil {
		return nil, fmt.Errorf("memory limit exceeded: %w", err)
	}

	// Create document for processing
	document := &types.Document{
		ID:          input.DocumentID,
		Name:        input.DocumentName,
		Content:     input.Content,
		ContentType: string(input.DocumentType),
	}

	// Determine if chunking is needed
	if !oe.config.EnableChunking || len(input.Content) <= oe.config.ChunkingConfig.MaxChunkSize {
		return oe.processSingleDocument(ctx, document, input)
	}

	return oe.processChunkedDocument(ctx, document, input)
}

// processSingleDocument handles documents that don't need chunking
func (oe *OptimizedExtractor) processSingleDocument(ctx context.Context, document *types.Document, input types.ExtractionInput) (*types.EntityExtractionResult, error) {
	// Create extraction config
	extractionConfig := &interfaces.EntityExtractionConfig{
		Enabled:       true,
		MinConfidence: oe.config.ConfidenceThreshold,
		EntityTypes:   input.RequiredEntityTypes,
	}

	// Extract entities using base extractor
	result, err := oe.baseExtractor.Extract(ctx, document, extractionConfig)
	if err != nil {
		return nil, err
	}

	// Convert interface result to types result
	converted := oe.convertResult(result, input)

	// Apply confidence filtering to ensure consistency
	converted.Entities = oe.filterExtractedEntitiesByConfidence(converted.Entities)
	converted.TotalEntities = len(converted.Entities)

	oe.metrics.EntitiesExtracted = len(converted.Entities)
	return converted, nil
}

// processChunkedDocument handles large documents that need chunking
func (oe *OptimizedExtractor) processChunkedDocument(ctx context.Context, document *types.Document, originalInput types.ExtractionInput) (*types.EntityExtractionResult, error) {
	// Split document into intelligent chunks
	chunks, err := oe.chunker.ChunkDocument(ctx, document)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk document: %w", err)
	}

	oe.metrics.ChunksProcessed = len(chunks)

	// Process chunks with concurrency control
	results := make([]*ChunkResult, len(chunks))
	semaphore := make(chan struct{}, oe.config.MaxConcurrentChunks)
	var wg sync.WaitGroup
	var resultMutex sync.Mutex

	for i, chunk := range chunks {
		wg.Add(1)
		go func(index int, chunk *DocumentChunk) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process chunk
			result := oe.processChunk(ctx, chunk, originalInput)

			// Store result
			resultMutex.Lock()
			results[index] = result
			resultMutex.Unlock()
		}(i, chunk)
	}

	wg.Wait()

	// Merge results from all chunks
	return oe.mergeChunkResults(results, originalInput)
}

// processChunk extracts entities from a single chunk
func (oe *OptimizedExtractor) processChunk(ctx context.Context, chunk *DocumentChunk, originalInput types.ExtractionInput) *ChunkResult {
	startTime := time.Now()

	result := &ChunkResult{
		ChunkID:    chunk.ID,
		ChunkIndex: chunk.Index,
	}

	// Create document for this chunk
	chunkDoc := &types.Document{
		ID:          chunk.ID,
		Name:        fmt.Sprintf("%s_chunk_%d", originalInput.DocumentID, chunk.Index),
		Content:     chunk.Content,
		ContentType: string(originalInput.DocumentType),
	}

	// Create extraction config
	extractionConfig := &interfaces.EntityExtractionConfig{
		Enabled:       true,
		MinConfidence: oe.config.ConfidenceThreshold,
		EntityTypes:   originalInput.RequiredEntityTypes,
	}

	// Track memory usage for this chunk
	memoryBefore := oe.getCurrentMemoryUsage()

	// Extract entities from chunk
	extractionResult, err := oe.baseExtractor.Extract(ctx, chunkDoc, extractionConfig)
	if err != nil {
		result.Error = err
		return result
	}

	result.Entities = extractionResult.Entities
	result.Confidence = extractionResult.Confidence
	result.ProcessingTime = time.Since(startTime)
	result.MemoryUsed = oe.getCurrentMemoryUsage() - memoryBefore

	// Add cross-references if enabled
	if oe.config.EnableCrossReferencing && chunk.CrossRefs != nil {
		result.CrossReferences = oe.buildCrossReferences(result.Entities, chunk.CrossRefs)
	}

	return result
}

// buildCrossReferences creates cross-references for entities in a chunk
func (oe *OptimizedExtractor) buildCrossReferences(entities []*types.Entity, crossRefContext *CrossReferenceContext) []CrossReference {
	var crossRefs []CrossReference

	// Create cross-references to entities in related chunks
	for _, entity := range entities {
		// Look for references to previous entities
		for _, prevEntity := range crossRefContext.PreviousEntities {
			if oe.entitiesMatch(*entity, prevEntity) {
				crossRef := CrossReference{
					SourceEntityID:   entity.ID,
					TargetEntityID:   prevEntity.ID,
					RelationType:     "same_entity",
					Confidence:       0.8,
					SourceChunkIndex: 0, // We'll set this later
					TargetChunkIndex: prevEntity.ChunkIndex,
					Context:          prevEntity.Context,
				}
				crossRefs = append(crossRefs, crossRef)
			}
		}

		// Look for references to following entities
		for _, followingEntity := range crossRefContext.FollowingEntities {
			if oe.entitiesMatch(*entity, followingEntity) {
				crossRef := CrossReference{
					SourceEntityID:   entity.ID,
					TargetEntityID:   followingEntity.ID,
					RelationType:     "same_entity",
					Confidence:       0.8,
					SourceChunkIndex: 0, // We'll set this later
					TargetChunkIndex: followingEntity.ChunkIndex,
					Context:          followingEntity.Context,
				}
				crossRefs = append(crossRefs, crossRef)
			}
		}
	}

	oe.metrics.CrossReferencesFound += len(crossRefs)
	return crossRefs
}

// entitiesMatch checks if two entities likely refer to the same thing
func (oe *OptimizedExtractor) entitiesMatch(entity types.Entity, refEntity EntityReference) bool {
	// Simple matching based on text similarity and type
	if entity.Type != refEntity.Type {
		return false
	}

	// Exact text match
	if entity.Text == refEntity.Text {
		return true
	}

	// Fuzzy matching for minor variations
	return oe.calculateSimilarity(entity.Text, refEntity.Text) > 0.8
}

// calculateSimilarity computes text similarity (simplified implementation)
func (oe *OptimizedExtractor) calculateSimilarity(text1, text2 string) float64 {
	if text1 == text2 {
		return 1.0
	}

	// Simple similarity based on common words
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	common := 0
	total := len(words1) + len(words2)

	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 {
				common++
				break
			}
		}
	}

	if total == 0 {
		return 0.0
	}

	return float64(common*2) / float64(total)
}

// mergeChunkResults combines results from all chunks into a final result
func (oe *OptimizedExtractor) mergeChunkResults(results []*ChunkResult, originalInput types.ExtractionInput) (*types.EntityExtractionResult, error) {
	var allEntities []*types.Entity
	var totalConfidence float64
	var validResults int

	// Collect all entities from successful chunk results
	for _, result := range results {
		if result.Error != nil {
			continue
		}

		validResults++
		totalConfidence += result.Confidence

		// Add entities (with chunk context in metadata)
		for _, entity := range result.Entities {
			if entity.Metadata == nil {
				entity.Metadata = make(map[string]any)
			}
			entity.Metadata["chunk_index"] = result.ChunkIndex
			entity.Metadata["chunk_id"] = result.ChunkID
			allEntities = append(allEntities, entity)
		}
	}

	if validResults == 0 {
		return nil, fmt.Errorf("no chunks processed successfully")
	}

	// Calculate average confidence
	avgConfidence := totalConfidence / float64(validResults)

	// Merge duplicate entities if enabled
	if oe.config.EnableEntityMerging {
		allEntities = oe.mergeDuplicateEntities(allEntities)
	}

	// Filter by confidence threshold
	filteredEntities := oe.filterByConfidence(allEntities)

	oe.metrics.EntitiesExtracted = len(filteredEntities)

	// Create final result
	result := &types.EntityExtractionResult{
		ID:               fmt.Sprintf("extraction_%s_%d", originalInput.DocumentID, time.Now().UnixNano()),
		DocumentID:       originalInput.DocumentID,
		DocumentType:     originalInput.DocumentType,
		Entities:         oe.convertToExtractedEntities(filteredEntities),
		TotalEntities:    len(filteredEntities),
		ProcessingTime:   oe.metrics.ProcessingTime.String(),
		ProcessingTimeMs: oe.metrics.ProcessingTime.Milliseconds(),
		ExtractionMetadata: map[string]any{
			"chunks_processed":       oe.metrics.ChunksProcessed,
			"entities_extracted":     oe.metrics.EntitiesExtracted,
			"cross_references_found": oe.metrics.CrossReferencesFound,
			"processing_time_ms":     oe.metrics.ProcessingTime.Milliseconds(),
			"memory_usage_bytes":     oe.metrics.MemoryUsage,
			"chunking_enabled":       oe.config.EnableChunking,
			"cross_referencing":      oe.config.EnableCrossReferencing,
			"average_confidence":     avgConfidence,
		},
		CreatedAt: time.Now(),
		Method:    types.ExtractorMethodLLM, // Default to LLM method
	}

	return result, nil
}

// convertToExtractedEntities converts types.Entity to types.ExtractedEntity
func (oe *OptimizedExtractor) convertToExtractedEntities(entities []*types.Entity) []*types.ExtractedEntity {
	extracted := make([]*types.ExtractedEntity, len(entities))

	for i, entity := range entities {
		extracted[i] = &types.ExtractedEntity{
			ID:             entity.ID,
			Type:           entity.Type,
			Text:           entity.Text,
			NormalizedText: strings.ToLower(strings.TrimSpace(entity.Text)),
			Confidence:     entity.Confidence,
			StartOffset:    entity.StartOffset,
			EndOffset:      entity.EndOffset,
			Context:        "",
			DocumentID:     entity.DocumentID,
			ExtractedAt:    entity.ExtractedAt,
			ExtractionID:   fmt.Sprintf("extraction_%s", entity.DocumentID),
		}

		// Convert entity context if available
		if entity.Context != nil {
			extracted[i].Context = entity.Context.SurroundingText
			extracted[i].ContextBefore = entity.Context.SentenceBefore
			extracted[i].ContextAfter = entity.Context.SentenceAfter
			extracted[i].Section = entity.Context.Section
		}
	}

	return extracted
}

// convertResult converts interface result to types result
func (oe *OptimizedExtractor) convertResult(result *interfaces.EntityExtractionResult, input types.ExtractionInput) *types.EntityExtractionResult {
	return &types.EntityExtractionResult{
		ID:               fmt.Sprintf("extraction_%s_%d", input.DocumentID, time.Now().UnixNano()),
		DocumentID:       input.DocumentID,
		DocumentType:     input.DocumentType,
		Entities:         oe.convertToExtractedEntities(result.Entities),
		TotalEntities:    len(result.Entities),
		ProcessingTime:   result.ProcessingTime.String(),
		ProcessingTimeMs: result.ProcessingTime.Milliseconds(),
		ExtractionMetadata: map[string]any{
			"method":     result.Method,
			"confidence": result.Confidence,
			"cache_hit":  result.CacheHit,
			"metadata":   result.Metadata,
		},
		CreatedAt: time.Now(),
		Method:    result.Method,
	}
}

// mergeDuplicateEntities removes or merges duplicate entities across chunks
func (oe *OptimizedExtractor) mergeDuplicateEntities(entities []*types.Entity) []*types.Entity {
	entityMap := make(map[string]*types.Entity)

	for _, entity := range entities {
		key := fmt.Sprintf("%s:%s", entity.Type, strings.ToLower(entity.Text))

		if existing, exists := entityMap[key]; exists {
			// Merge entities by taking higher confidence
			if entity.Confidence > existing.Confidence {
				entityMap[key] = entity
			}
		} else {
			entityMap[key] = entity
		}
	}

	// Convert back to slice
	merged := make([]*types.Entity, 0, len(entityMap))
	for _, entity := range entityMap {
		merged = append(merged, entity)
	}

	return merged
}

// filterByConfidence removes entities below confidence threshold
func (oe *OptimizedExtractor) filterByConfidence(entities []*types.Entity) []*types.Entity {
	filtered := make([]*types.Entity, 0, len(entities))

	for _, entity := range entities {
		if entity.Confidence >= oe.config.ConfidenceThreshold {
			filtered = append(filtered, entity)
		}
	}

	return filtered
}

// filterExtractedEntitiesByConfidence removes extracted entities below confidence threshold
func (oe *OptimizedExtractor) filterExtractedEntitiesByConfidence(entities []*types.ExtractedEntity) []*types.ExtractedEntity {
	filtered := make([]*types.ExtractedEntity, 0, len(entities))

	for _, entity := range entities {
		if entity.Confidence >= oe.config.ConfidenceThreshold {
			filtered = append(filtered, entity)
		}
	}

	return filtered
}

// Memory management methods

// checkMemoryLimits verifies current memory usage is within limits
func (oe *OptimizedExtractor) checkMemoryLimits() error {
	oe.memoryMutex.RLock()
	defer oe.memoryMutex.RUnlock()

	if oe.currentMemoryUsage > oe.config.MaxMemoryUsage {
		return fmt.Errorf("memory usage %d exceeds limit %d", oe.currentMemoryUsage, oe.config.MaxMemoryUsage)
	}

	return nil
}

// getCurrentMemoryUsage returns current memory usage estimate
func (oe *OptimizedExtractor) getCurrentMemoryUsage() int64 {
	oe.memoryMutex.RLock()
	defer oe.memoryMutex.RUnlock()
	return oe.currentMemoryUsage
}

// updateMemoryUsage updates the current memory usage tracking
func (oe *OptimizedExtractor) updateMemoryUsage(delta int64) {
	oe.memoryMutex.Lock()
	defer oe.memoryMutex.Unlock()
	oe.currentMemoryUsage += delta
	if oe.currentMemoryUsage < 0 {
		oe.currentMemoryUsage = 0
	}
}

// GetMetrics returns current extraction metrics
func (oe *OptimizedExtractor) GetMetrics() *ExtractionMetrics {
	return oe.metrics
}

// Reset clears metrics and resets state
func (oe *OptimizedExtractor) Reset() {
	oe.metrics = &ExtractionMetrics{}
	oe.updateMemoryUsage(-oe.getCurrentMemoryUsage())
}
