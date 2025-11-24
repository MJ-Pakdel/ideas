package extractors

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/example/idaes/internal/types"
)

// ChunkingStrategy defines different approaches to document chunking
type ChunkingStrategy int

const (
	ChunkingNone ChunkingStrategy = iota
	ChunkingFixed
	ChunkingSemantic
	ChunkingIntelligent
	ChunkingOverlapping
)

// ChunkingConfig contains parameters for document chunking optimization
type ChunkingConfig struct {
	Strategy            ChunkingStrategy `json:"strategy"`
	MaxChunkSize        int              `json:"max_chunk_size"`        // In characters
	MinChunkSize        int              `json:"min_chunk_size"`        // In characters
	OverlapSize         int              `json:"overlap_size"`          // Characters of overlap
	PreserveEntities    bool             `json:"preserve_entities"`     // Avoid splitting entities
	PreserveSentences   bool             `json:"preserve_sentences"`    // Split on sentence boundaries
	PreserveParagraphs  bool             `json:"preserve_paragraphs"`   // Split on paragraph boundaries
	ContextWindowSize   int              `json:"context_window_size"`   // For cross-referencing
	EnableEntityMapping bool             `json:"enable_entity_mapping"` // Map entities across chunks
}

// DefaultChunkingConfig returns optimal settings for Raspberry Pi 5
func DefaultChunkingConfig() ChunkingConfig {
	return ChunkingConfig{
		Strategy:            ChunkingIntelligent,
		MaxChunkSize:        2048, // Optimized for Pi 5 memory constraints
		MinChunkSize:        512,  // Ensure meaningful content
		OverlapSize:         256,  // 12.5% overlap for context preservation
		PreserveEntities:    true,
		PreserveSentences:   true,
		PreserveParagraphs:  true,
		ContextWindowSize:   128, // Characters for cross-reference context
		EnableEntityMapping: true,
	}
}

// DocumentChunk represents a processed chunk with cross-reference metadata
type DocumentChunk struct {
	ID            string                 `json:"id"`
	Index         int                    `json:"index"`
	Content       string                 `json:"content"`
	StartOffset   int                    `json:"start_offset"`
	EndOffset     int                    `json:"end_offset"`
	OverlapBefore string                 `json:"overlap_before,omitempty"`
	OverlapAfter  string                 `json:"overlap_after,omitempty"`
	CrossRefs     *CrossReferenceContext `json:"cross_refs,omitempty"`
	Metadata      map[string]any         `json:"metadata,omitempty"`
}

// CrossReferenceContext maintains context for intelligent cross-referencing
type CrossReferenceContext struct {
	PreviousEntities  []EntityReference   `json:"previous_entities,omitempty"`
	FollowingEntities []EntityReference   `json:"following_entities,omitempty"`
	Citations         []CitationReference `json:"citations,omitempty"`
	Keywords          []string            `json:"keywords,omitempty"`
	SectionContext    string              `json:"section_context,omitempty"`
	RelatedChunks     []int               `json:"related_chunks,omitempty"`
}

// EntityReference stores entity information for cross-chunk referencing
type EntityReference struct {
	ID         string           `json:"id"`
	Type       types.EntityType `json:"type"`
	Text       string           `json:"text"`
	Confidence float64          `json:"confidence"`
	ChunkIndex int              `json:"chunk_index"`
	Context    string           `json:"context"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
}

// CitationReference stores citation context for cross-referencing
type CitationReference struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	ChunkIndex int     `json:"chunk_index"`
	Context    string  `json:"context"`
	Relevance  float64 `json:"relevance"`
}

// IntelligentChunker implements smart document chunking with cross-reference preservation
type IntelligentChunker struct {
	config ChunkingConfig
}

// NewIntelligentChunker creates a new intelligent chunker
func NewIntelligentChunker(config ChunkingConfig) *IntelligentChunker {
	return &IntelligentChunker{
		config: config,
	}
}

// ChunkDocument splits a document into intelligent chunks with preserved cross-references
func (ic *IntelligentChunker) ChunkDocument(ctx context.Context, document *types.Document) ([]*DocumentChunk, error) {
	if len(document.Content) <= ic.config.MaxChunkSize {
		// Document is small enough to process as single chunk
		return []*DocumentChunk{
			{
				ID:          fmt.Sprintf("%s_chunk_0", document.ID),
				Index:       0,
				Content:     document.Content,
				StartOffset: 0,
				EndOffset:   len(document.Content),
				CrossRefs:   ic.buildInitialCrossRefs(document.Content),
				Metadata: map[string]any{
					"chunking_strategy": "single_chunk",
					"original_size":     len(document.Content),
				},
			},
		}, nil
	}

	switch ic.config.Strategy {
	case ChunkingIntelligent:
		return ic.intelligentChunking(document)
	case ChunkingSemantic:
		return ic.semanticChunking(document)
	case ChunkingOverlapping:
		return ic.overlappingChunking(document)
	case ChunkingFixed:
		return ic.fixedChunking(document)
	default:
		return ic.intelligentChunking(document)
	}
}

// intelligentChunking uses AI-driven approach to preserve entity boundaries and context
func (ic *IntelligentChunker) intelligentChunking(document *types.Document) ([]*DocumentChunk, error) {
	content := document.Content
	chunks := []*DocumentChunk{}

	// First pass: identify semantic boundaries
	boundaries := ic.findSemanticBoundaries(content)

	// Second pass: identify entity boundaries if enabled
	if ic.config.PreserveEntities {
		entityBoundaries := ic.findEntityBoundaries(content)
		boundaries = ic.mergeBoundaries(boundaries, entityBoundaries)
	}

	// Third pass: create chunks respecting boundaries
	currentPos := 0
	chunkIndex := 0

	for currentPos < len(content) {
		chunkEnd := ic.findOptimalChunkEnd(content, currentPos, boundaries)

		chunk := ic.createChunk(
			document.ID,
			chunkIndex,
			content,
			currentPos,
			chunkEnd,
		)

		// Add cross-reference context
		chunk.CrossRefs = ic.buildCrossReferenceContext(content, currentPos, chunkEnd, chunks)

		chunks = append(chunks, chunk)

		// Calculate next position with overlap
		nextPos := chunkEnd
		if ic.config.OverlapSize > 0 && chunkEnd < len(content) {
			nextPos = max(currentPos+ic.config.MinChunkSize, chunkEnd-ic.config.OverlapSize)
		}

		currentPos = nextPos
		chunkIndex++
	}

	// Post-processing: enhance cross-references across all chunks
	ic.enhanceCrossReferences(chunks, content)

	return chunks, nil
}

// findSemanticBoundaries identifies natural break points in the document
func (ic *IntelligentChunker) findSemanticBoundaries(content string) []int {
	var boundaries []int

	// Add paragraph boundaries
	if ic.config.PreserveParagraphs {
		for i, char := range content {
			if char == '\n' {
				// Look for double newlines (paragraph breaks)
				if i+1 < len(content) && rune(content[i+1]) == '\n' {
					boundaries = append(boundaries, i+2)
				}
			}
		}
	}

	// Add sentence boundaries
	if ic.config.PreserveSentences {
		for i, char := range content {
			if char == '.' || char == '!' || char == '?' {
				// Check if this is likely a sentence end
				if i+1 < len(content) && unicode.IsSpace(rune(content[i+1])) {
					boundaries = append(boundaries, i+1)
				}
			}
		}
	}

	// Add section boundaries (headers, etc.)
	for i := 0; i < len(content)-1; i++ {
		if content[i] == '\n' && content[i+1] == '#' {
			boundaries = append(boundaries, i+1)
		}
	}

	return ic.deduplicateAndSort(boundaries)
}

// findEntityBoundaries identifies potential entity boundaries to avoid splitting
func (ic *IntelligentChunker) findEntityBoundaries(content string) []int {
	var boundaries []int

	// Simple heuristic: look for capitalized words that might be entities
	words := strings.Fields(content)
	currentPos := 0

	for _, word := range words {
		wordStart := strings.Index(content[currentPos:], word)
		if wordStart == -1 {
			break
		}
		wordStart += currentPos

		// Check if word looks like an entity (capitalized, multiple words, etc.)
		if ic.looksLikeEntity(word) {
			boundaries = append(boundaries, wordStart)
			boundaries = append(boundaries, wordStart+len(word))
		}

		currentPos = wordStart + len(word)
	}

	return ic.deduplicateAndSort(boundaries)
}

// looksLikeEntity uses heuristics to identify potential entities
func (ic *IntelligentChunker) looksLikeEntity(word string) bool {
	if len(word) < 2 {
		return false
	}

	// Check for capitalization
	if !unicode.IsUpper(rune(word[0])) {
		return false
	}

	// Check for common entity patterns
	patterns := []string{
		"Dr.", "Prof.", "Mr.", "Ms.", "Mrs.", // Person titles
		"Inc.", "Ltd.", "Corp.", "LLC", // Organization suffixes
		"University", "Institute", "Company", // Organization types
	}

	for _, pattern := range patterns {
		if strings.Contains(word, pattern) {
			return true
		}
	}

	return len(word) > 3 && unicode.IsUpper(rune(word[0]))
}

// mergeBoundaries combines different types of boundaries intelligently
func (ic *IntelligentChunker) mergeBoundaries(boundaries1, boundaries2 []int) []int {
	merged := append(boundaries1, boundaries2...)
	return ic.deduplicateAndSort(merged)
}

// findOptimalChunkEnd finds the best ending position for a chunk
func (ic *IntelligentChunker) findOptimalChunkEnd(content string, start int, boundaries []int) int {
	maxEnd := min(start+ic.config.MaxChunkSize, len(content))
	minEnd := min(start+ic.config.MinChunkSize, len(content))

	// Find the best boundary within our target range
	bestBoundary := maxEnd

	for _, boundary := range boundaries {
		if boundary >= minEnd && boundary <= maxEnd {
			bestBoundary = boundary
			break // Take the first valid boundary
		}
	}

	return bestBoundary
}

// createChunk creates a document chunk with proper metadata
func (ic *IntelligentChunker) createChunk(docID string, index int, content string, start, end int) *DocumentChunk {
	chunkContent := content[start:end]

	chunk := &DocumentChunk{
		ID:          fmt.Sprintf("%s_chunk_%d", docID, index),
		Index:       index,
		Content:     chunkContent,
		StartOffset: start,
		EndOffset:   end,
		Metadata: map[string]any{
			"chunking_strategy": "intelligent",
			"chunk_size":        len(chunkContent),
			"overlap_enabled":   ic.config.OverlapSize > 0,
		},
	}

	// Add overlap content if configured
	if ic.config.OverlapSize > 0 {
		if start > 0 {
			overlapStart := max(0, start-ic.config.OverlapSize)
			chunk.OverlapBefore = content[overlapStart:start]
		}
		if end < len(content) {
			overlapEnd := min(len(content), end+ic.config.OverlapSize)
			chunk.OverlapAfter = content[end:overlapEnd]
		}
	}

	return chunk
}

// buildCrossReferenceContext creates cross-reference context for a chunk
func (ic *IntelligentChunker) buildCrossReferenceContext(content string, start, end int, previousChunks []*DocumentChunk) *CrossReferenceContext {
	crossRefs := &CrossReferenceContext{
		Keywords:       ic.extractKeywords(content[start:end]),
		SectionContext: ic.detectSectionContext(content, start),
		RelatedChunks:  make([]int, 0),
	}

	// Build context window around current chunk
	contextStart := max(0, start-ic.config.ContextWindowSize)
	contextEnd := min(len(content), end+ic.config.ContextWindowSize)

	// Extract entities from context window for cross-referencing
	if ic.config.EnableEntityMapping {
		// This would typically use the entity extractor, but for now we'll use simple heuristics
		crossRefs.PreviousEntities = ic.extractSimpleEntities(content[contextStart:start], len(previousChunks)-1)
		crossRefs.FollowingEntities = ic.extractSimpleEntities(content[end:contextEnd], len(previousChunks))
	}

	return crossRefs
}

// buildInitialCrossRefs creates cross-reference context for single-chunk documents
func (ic *IntelligentChunker) buildInitialCrossRefs(content string) *CrossReferenceContext {
	return &CrossReferenceContext{
		Keywords:       ic.extractKeywords(content),
		SectionContext: "full_document",
		RelatedChunks:  []int{0},
	}
}

// extractKeywords extracts key terms for cross-referencing
func (ic *IntelligentChunker) extractKeywords(content string) []string {
	// Simple keyword extraction - in production, this would use more sophisticated NLP
	words := strings.Fields(content)
	keywordMap := make(map[string]bool)

	for _, word := range words {
		cleaned := strings.ToLower(strings.Trim(word, ".,!?;:"))
		if len(cleaned) > 4 && !ic.isStopWord(cleaned) {
			keywordMap[cleaned] = true
		}
	}

	keywords := make([]string, 0, len(keywordMap))
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}

	return keywords
}

// isStopWord checks if a word is a common stop word
func (ic *IntelligentChunker) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "that": true, "with": true,
		"this": true, "they": true, "from": true, "have": true,
		"will": true, "been": true, "were": true, "said": true,
	}
	return stopWords[word]
}

// detectSectionContext identifies the document section for cross-referencing
func (ic *IntelligentChunker) detectSectionContext(content string, position int) string {
	// Look backwards for section headers
	for i := position; i >= 0; i-- {
		if i == 0 || content[i-1] == '\n' {
			if i < len(content) && content[i] == '#' {
				// Find end of header line
				end := i
				for end < len(content) && content[end] != '\n' {
					end++
				}
				return strings.TrimSpace(content[i:end])
			}
		}
	}

	return "unknown_section"
}

// extractSimpleEntities extracts potential entities for cross-referencing
func (ic *IntelligentChunker) extractSimpleEntities(content string, chunkIndex int) []EntityReference {
	entities := []EntityReference{}
	words := strings.Fields(content)

	for i, word := range words {
		cleaned := strings.Trim(word, ".,!?;:")
		if ic.looksLikeEntity(cleaned) {
			entities = append(entities, EntityReference{
				ID:         fmt.Sprintf("ref_%d_%d", chunkIndex, i),
				Type:       types.EntityTypePerson, // Default assumption
				Text:       cleaned,
				Confidence: 0.5, // Low confidence for heuristic detection
				ChunkIndex: chunkIndex,
				Context:    ic.getWordContext(words, i, 3),
			})
		}
	}

	return entities
}

// getWordContext extracts surrounding words for context
func (ic *IntelligentChunker) getWordContext(words []string, index, windowSize int) string {
	start := max(0, index-windowSize)
	end := min(len(words), index+windowSize+1)
	return strings.Join(words[start:end], " ")
}

// enhanceCrossReferences performs post-processing to improve cross-references
func (ic *IntelligentChunker) enhanceCrossReferences(chunks []*DocumentChunk, content string) {
	// Build global entity map
	entityMap := make(map[string][]EntityReference)

	for _, chunk := range chunks {
		if chunk.CrossRefs != nil {
			for _, entity := range chunk.CrossRefs.PreviousEntities {
				entityMap[entity.Text] = append(entityMap[entity.Text], entity)
			}
			for _, entity := range chunk.CrossRefs.FollowingEntities {
				entityMap[entity.Text] = append(entityMap[entity.Text], entity)
			}
		}
	}

	// Update related chunks based on shared entities
	for _, chunk := range chunks {
		if chunk.CrossRefs == nil {
			continue
		}

		relatedChunks := make(map[int]bool)

		// Find chunks with shared entities
		for _, entity := range chunk.CrossRefs.PreviousEntities {
			if refs, exists := entityMap[entity.Text]; exists {
				for _, ref := range refs {
					if ref.ChunkIndex != chunk.Index {
						relatedChunks[ref.ChunkIndex] = true
					}
				}
			}
		}

		for _, entity := range chunk.CrossRefs.FollowingEntities {
			if refs, exists := entityMap[entity.Text]; exists {
				for _, ref := range refs {
					if ref.ChunkIndex != chunk.Index {
						relatedChunks[ref.ChunkIndex] = true
					}
				}
			}
		}

		// Convert to slice
		for chunkIdx := range relatedChunks {
			chunk.CrossRefs.RelatedChunks = append(chunk.CrossRefs.RelatedChunks, chunkIdx)
		}
	}
}

// Utility functions

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (ic *IntelligentChunker) deduplicateAndSort(boundaries []int) []int {
	if len(boundaries) == 0 {
		return boundaries
	}

	// Remove duplicates and sort
	boundaryMap := make(map[int]bool)
	for _, b := range boundaries {
		boundaryMap[b] = true
	}

	result := make([]int, 0, len(boundaryMap))
	for b := range boundaryMap {
		result = append(result, b)
	}

	// Simple sort
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Additional chunking strategies

// semanticChunking uses semantic analysis for chunk boundaries
func (ic *IntelligentChunker) semanticChunking(document *types.Document) ([]*DocumentChunk, error) {
	// Placeholder for semantic chunking - would use NLP libraries in production
	return ic.overlappingChunking(document)
}

// overlappingChunking creates overlapping chunks for better context preservation
func (ic *IntelligentChunker) overlappingChunking(document *types.Document) ([]*DocumentChunk, error) {
	content := document.Content
	chunks := []*DocumentChunk{}
	chunkIndex := 0

	for start := 0; start < len(content); {
		end := min(start+ic.config.MaxChunkSize, len(content))

		chunk := ic.createChunk(document.ID, chunkIndex, content, start, end)
		chunk.CrossRefs = ic.buildCrossReferenceContext(content, start, end, chunks)

		chunks = append(chunks, chunk)

		// Move start position with overlap
		if end >= len(content) {
			break
		}

		start = end - ic.config.OverlapSize
		if start <= chunks[len(chunks)-1].StartOffset {
			start = end // Prevent infinite loop
		}
		chunkIndex++
	}

	ic.enhanceCrossReferences(chunks, content)
	return chunks, nil
}

// fixedChunking creates fixed-size chunks (least intelligent)
func (ic *IntelligentChunker) fixedChunking(document *types.Document) ([]*DocumentChunk, error) {
	content := document.Content
	chunks := []*DocumentChunk{}
	chunkIndex := 0

	for start := 0; start < len(content); start += ic.config.MaxChunkSize {
		end := min(start+ic.config.MaxChunkSize, len(content))

		chunk := ic.createChunk(document.ID, chunkIndex, content, start, end)
		chunk.CrossRefs = ic.buildCrossReferenceContext(content, start, end, chunks)

		chunks = append(chunks, chunk)
		chunkIndex++
	}

	ic.enhanceCrossReferences(chunks, content)
	return chunks, nil
}
