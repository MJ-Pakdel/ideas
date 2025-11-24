package extractors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/example/idaes/internal/types"
)

func TestIntelligentChunking(t *testing.T) {
	ctx := t.Context()
	config := DefaultChunkingConfig()
	config.MaxChunkSize = 50 // Very small chunks for testing
	config.OverlapSize = 10  // 20% overlap
	config.EnableEntityMapping = true

	chunker := NewIntelligentChunker(config)

	// Test document with entities that span chunks - make it longer
	content := `Dr. John Smith works at MIT Research Laboratory. MIT is a leading research institution focused on artificial intelligence and machine learning. John Smith published numerous papers on AI and deep learning algorithms. AI research at MIT focuses on neural networks, natural language processing, and computer vision. Dr. Smith's recent work explores advanced neural network architectures and their applications in real-world scenarios.`

	document := &types.Document{
		ID:      "test_doc",
		Content: content,
	}

	t.Logf("Content length: %d, MaxChunkSize: %d", len(content), config.MaxChunkSize)

	chunks, err := chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	t.Logf("Got %d chunks", len(chunks))

	// Debug: let's see what happens with overlapping strategy specifically
	config.Strategy = ChunkingOverlapping
	chunker = NewIntelligentChunker(config)

	chunks, err = chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument with overlapping failed: %v", err)
	}

	t.Logf("Overlapping strategy produced %d chunks", len(chunks))

	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks with overlapping strategy, got %d", len(chunks))
	}

	// Verify chunks have cross-reference context
	for i, chunk := range chunks {
		t.Logf("Chunk %d (%d chars): %s", i, len(chunk.Content), chunk.Content)

		if chunk.CrossRefs == nil {
			t.Errorf("Chunk %d missing cross-reference context", i)
			continue
		}

		t.Logf("Chunk %d keywords: %v", i, chunk.CrossRefs.Keywords)
		t.Logf("Chunk %d section: %s", i, chunk.CrossRefs.SectionContext)

		if len(chunk.CrossRefs.Keywords) == 0 {
			t.Errorf("Chunk %d has no keywords", i)
		}
	}

	// Verify overlaps exist
	for i := 0; i < len(chunks)-1; i++ {
		if chunks[i].OverlapAfter == "" && i < len(chunks)-1 {
			t.Errorf("Chunk %d missing overlap after", i)
		}
		if chunks[i+1].OverlapBefore == "" {
			t.Errorf("Chunk %d missing overlap before", i+1)
		}
	}
}

func TestSemanticBoundaries(t *testing.T) {
	config := DefaultChunkingConfig()
	config.MaxChunkSize = 200
	config.PreserveParagraphs = true
	config.PreserveSentences = true

	chunker := NewIntelligentChunker(config)

	content := `
# Introduction

This is the first paragraph. It discusses the background.

This is the second paragraph. It introduces the methodology.

## Methods

The methods section starts here. We used various techniques.

Statistical analysis was performed using Python.

## Results

The results show significant improvements.
`

	// Test finding semantic boundaries
	boundaries := chunker.findSemanticBoundaries(content)

	if len(boundaries) == 0 {
		t.Fatal("No semantic boundaries found")
	}

	t.Logf("Found %d semantic boundaries", len(boundaries))

	// Verify section headers are captured
	foundIntroduction := false
	foundMethods := false
	foundResults := false

	for _, boundary := range boundaries {
		if boundary < len(content) {
			snippet := content[boundary:minInt(boundary+20, len(content))]
			t.Logf("Boundary at %d: %q", boundary, snippet)

			if strings.Contains(snippet, "Introduction") {
				foundIntroduction = true
			}
			if strings.Contains(snippet, "Methods") {
				foundMethods = true
			}
			if strings.Contains(snippet, "Results") {
				foundResults = true
			}
		}
	}

	if !foundIntroduction || !foundMethods || !foundResults {
		t.Error("Not all section headers found in boundaries")
	}
}

func TestEntityBoundaryDetection(t *testing.T) {
	config := DefaultChunkingConfig()
	config.PreserveEntities = true

	chunker := NewIntelligentChunker(config)

	content := "Dr. John Smith from MIT collaborated with Prof. Jane Doe at Stanford University."

	boundaries := chunker.findEntityBoundaries(content)

	if len(boundaries) == 0 {
		t.Fatal("No entity boundaries found")
	}

	t.Logf("Found %d entity boundaries", len(boundaries))

	// Check that likely entities are detected
	expectedEntities := []string{"Dr.", "John", "Smith", "MIT", "Prof.", "Jane", "Doe", "Stanford", "University"}
	foundCount := 0

	for _, boundary := range boundaries {
		if boundary < len(content) {
			// Check if boundary aligns with expected entities
			for _, entity := range expectedEntities {
				if boundary < len(content) && strings.HasPrefix(content[boundary:], entity) {
					foundCount++
					t.Logf("Found entity boundary for: %s", entity)
					break
				}
			}
		}
	}

	if foundCount == 0 {
		t.Error("No entity boundaries aligned with expected entities")
	}
}

func TestCrossReferenceEnhancement(t *testing.T) {
	ctx := t.Context()
	config := DefaultChunkingConfig()
	config.MaxChunkSize = 40 // Reasonable chunk size for testing
	config.MinChunkSize = 10 // Small minimum for multiple chunks
	config.EnableEntityMapping = true

	chunker := NewIntelligentChunker(config)

	// Document with repeated entities across chunks
	content := "MIT researchers study AI. Dr. Smith leads MIT's AI lab. AI advances at MIT continue."

	document := &types.Document{
		ID:      "test_cross_ref",
		Content: content,
	}

	chunks, err := chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	// Verify we have multiple chunks
	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks, got %d", len(chunks))
	}

	// Check for cross-references
	hasRelatedChunks := false
	sharedEntityCount := 0

	for i, chunk := range chunks {
		if chunk.CrossRefs != nil {
			// Count shared entities
			if len(chunk.CrossRefs.PreviousEntities) > 0 || len(chunk.CrossRefs.FollowingEntities) > 0 {
				sharedEntityCount++
			}

			if len(chunk.CrossRefs.RelatedChunks) > 0 {
				hasRelatedChunks = true
				t.Logf("Chunk %d has %d related chunks", i, len(chunk.CrossRefs.RelatedChunks))
			}
		}
	}

	if !hasRelatedChunks {
		t.Error("No cross-references found between chunks with shared entities")
	}

	if sharedEntityCount == 0 {
		t.Error("No shared entities found across chunks")
	}
}

func TestIntelligentKeywordExtraction(t *testing.T) {
	config := DefaultChunkingConfig()
	chunker := NewIntelligentChunker(config)

	content := "Machine learning algorithms analyze big data efficiently. Neural networks process information systematically."

	keywords := chunker.extractKeywords(content)

	if len(keywords) == 0 {
		t.Fatal("No keywords extracted")
	}

	t.Logf("Extracted keywords: %v", keywords)

	// Check for meaningful keywords (longer than 4 characters, not stop words)
	meaningfulKeywords := 0
	for _, keyword := range keywords {
		if len(keyword) > 4 && !chunker.isStopWord(keyword) {
			meaningfulKeywords++
		}
	}

	if meaningfulKeywords == 0 {
		t.Error("No meaningful keywords found")
	}
}

func TestSectionContextDetection(t *testing.T) {
	config := DefaultChunkingConfig()
	chunker := NewIntelligentChunker(config)

	content := `
Some initial content.

# Introduction
This is the introduction section.

More content here.

## Methodology
This describes the methods.

Final content.
`

	testCases := []struct {
		position int
		expected string
	}{
		{50, "# Introduction"},  // Position in introduction
		{120, "## Methodology"}, // Position in methodology
		{10, "unknown_section"}, // Before any header
	}

	for _, tc := range testCases {
		result := chunker.detectSectionContext(content, tc.position)
		if !strings.Contains(result, strings.TrimSpace(tc.expected)) && tc.expected != "unknown_section" {
			t.Errorf("Expected section context to contain %q, got %q", tc.expected, result)
		}
		t.Logf("Position %d: %s", tc.position, result)
	}
}

func TestOverlappingChunking(t *testing.T) {
	ctx := t.Context()
	config := DefaultChunkingConfig()
	config.Strategy = ChunkingOverlapping
	config.MaxChunkSize = 100
	config.OverlapSize = 30

	chunker := NewIntelligentChunker(config)

	content := strings.Repeat("This is test content for overlapping chunks. ", 20)

	document := &types.Document{
		ID:      "test_overlap",
		Content: content,
	}

	chunks, err := chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks for overlapping test, got %d", len(chunks))
	}

	// Verify overlaps
	for i := 0; i < len(chunks)-1; i++ {
		currentChunk := chunks[i]
		nextChunk := chunks[i+1]

		// Check that end of current chunk overlaps with start of next chunk
		overlap := minInt(len(currentChunk.Content), config.OverlapSize)
		endSnippet := currentChunk.Content[len(currentChunk.Content)-overlap:]
		startSnippet := nextChunk.Content[:minInt(len(nextChunk.Content), overlap)]

		if !strings.Contains(nextChunk.Content, endSnippet[:min(len(endSnippet), 20)]) {
			t.Errorf("No overlap detected between chunks %d and %d", i, i+1)
		}

		t.Logf("Chunk %d end: %q", i, endSnippet[:minInt(len(endSnippet), 30)])
		t.Logf("Chunk %d start: %q", i+1, startSnippet[:minInt(len(startSnippet), 30)])
	}
}

func TestFixedChunking(t *testing.T) {
	ctx := t.Context()
	config := DefaultChunkingConfig()
	config.Strategy = ChunkingFixed
	config.MaxChunkSize = 50

	chunker := NewIntelligentChunker(config)

	content := strings.Repeat("Test ", 50) // 250 characters

	document := &types.Document{
		ID:      "test_fixed",
		Content: content,
	}

	chunks, err := chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	expectedChunks := (len(content) + config.MaxChunkSize - 1) / config.MaxChunkSize
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// Verify chunk sizes (except possibly the last one)
	for i, chunk := range chunks {
		if i < len(chunks)-1 && len(chunk.Content) != config.MaxChunkSize {
			t.Errorf("Chunk %d has size %d, expected %d", i, len(chunk.Content), config.MaxChunkSize)
		}
		t.Logf("Chunk %d: %d characters", i, len(chunk.Content))
	}
}

func TestSmallDocumentNoChunking(t *testing.T) {
	ctx := t.Context()
	config := DefaultChunkingConfig()
	config.MaxChunkSize = 1000

	chunker := NewIntelligentChunker(config)

	content := "This is a small document that should not be chunked."

	document := &types.Document{
		ID:      "test_small",
		Content: content,
	}

	chunks, err := chunker.ChunkDocument(ctx, document)
	if err != nil {
		t.Fatalf("ChunkDocument failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for small document, got %d", len(chunks))
	}

	if chunks[0].Content != content {
		t.Error("Single chunk content doesn't match original")
	}

	if chunks[0].CrossRefs == nil {
		t.Error("Single chunk missing cross-reference context")
	}
}

func TestChunkingStrategies(t *testing.T) {
	ctx := t.Context()
	strategies := []ChunkingStrategy{
		ChunkingFixed,
		ChunkingSemantic,
		ChunkingIntelligent,
		ChunkingOverlapping,
	}

	content := `
# Research Paper

## Introduction
This section introduces the research topic and provides background information.

## Methodology
We used statistical analysis and machine learning techniques.

## Results
The results demonstrate significant improvements in accuracy.

## Conclusion
Our findings suggest that the proposed method is effective.
`

	for _, strategy := range strategies {
		t.Run(fmt.Sprintf("Strategy_%d", strategy), func(t *testing.T) {
			config := DefaultChunkingConfig()
			config.Strategy = strategy
			config.MaxChunkSize = 100

			chunker := NewIntelligentChunker(config)

			document := &types.Document{
				ID:      fmt.Sprintf("test_strategy_%d", strategy),
				Content: content,
			}

			chunks, err := chunker.ChunkDocument(ctx, document)
			if err != nil {
				t.Fatalf("ChunkDocument failed for strategy %d: %v", strategy, err)
			}

			if len(chunks) == 0 {
				t.Errorf("No chunks created for strategy %d", strategy)
			}

			t.Logf("Strategy %d created %d chunks", strategy, len(chunks))

			// Verify all chunks have cross-reference context
			for i, chunk := range chunks {
				if chunk.CrossRefs == nil {
					t.Errorf("Chunk %d missing cross-reference context for strategy %d", i, strategy)
				}
			}
		})
	}
}

// Helper function for testing
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
