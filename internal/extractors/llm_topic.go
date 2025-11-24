package extractors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// LLMTopicExtractor implements topic extraction using Large Language Models
// This is a lightweight wrapper around the unified extractor for interface compatibility
type LLMTopicExtractor struct {
	config           *interfaces.TopicExtractionConfig
	metrics          *interfaces.ExtractorMetrics
	unifiedExtractor *LLMUnifiedExtractor
}

// NewLLMTopicExtractor creates a new LLM-based topic extractor
func NewLLMTopicExtractor(config *interfaces.TopicExtractionConfig, llmClient interfaces.LLMClient) *LLMTopicExtractor {
	if config == nil {
		config = &interfaces.TopicExtractionConfig{
			Enabled:       true,
			Method:        "llm",
			NumTopics:     10,
			MinConfidence: 0.6,
			MaxKeywords:   20,
		}
	}

	// Create unified extractor config from topic config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:      0.3,
		MaxTokens:        2048,
		MinConfidence:    config.MinConfidence,
		MaxTopics:        config.NumTopics,
		TopicMinKeywords: config.MaxKeywords,
		AnalysisDepth:    "detailed",
	}

	return &LLMTopicExtractor{
		config:           config,
		unifiedExtractor: NewLLMUnifiedExtractor(unifiedConfig, llmClient),
		metrics: &interfaces.ExtractorMetrics{
			ProcessingTime:       0,
			TotalDocuments:       0,
			AverageConfidence:    0.0,
			ErrorRate:            0.0,
			CacheHitRate:         0.0,
			ExternalServiceCalls: 0,
			MemoryUsage:          0,
		},
	}
}

// Extract implements the TopicExtractor interface
// This uses the unified extractor and filters results to return only topics
func (t *LLMTopicExtractor) Extract(ctx context.Context, document *types.Document, config *interfaces.TopicExtractionConfig) (*interfaces.TopicExtractionResult, error) {
	startTime := time.Now()

	slog.InfoContext(ctx, "Starting LLM topic extraction via unified extractor", "document_id", document.ID)

	// Use provided config or fall back to instance config
	if config == nil {
		config = t.config
	}

	// Create unified config from topic config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:      0.3,
		MaxTokens:        2048,
		MinConfidence:    config.MinConfidence,
		MaxEntities:      0, // We don't need entities for topic extraction
		MaxCitations:     0, // We don't need citations for topic extraction
		MaxTopics:        config.NumTopics,
		TopicMinKeywords: config.MaxKeywords,
		AnalysisDepth:    "detailed",
	}

	// Call unified extractor
	unifiedResult, err := t.unifiedExtractor.ExtractAll(ctx, document, unifiedConfig)
	if err != nil {
		t.updateMetrics(time.Since(startTime), false)
		return nil, fmt.Errorf("unified extraction failed: %w", err)
	}

	// Extract only topics from unified result
	result := &interfaces.TopicExtractionResult{
		Topics: unifiedResult.Topics,
		ExtractionMetadata: interfaces.ExtractionMetadata{
			ProcessingTime: time.Since(startTime),
			Method:         types.ExtractorMethodLLM,
			Confidence:     unifiedResult.Confidence,
			CacheHit:       false,
		},
	}

	// Update our own metrics
	t.updateMetrics(time.Since(startTime), true)

	slog.InfoContext(ctx, "Completed LLM topic extraction",
		"document_id", document.ID,
		"topics_found", len(result.Topics),
		"processing_time", time.Since(startTime))

	return result, nil
}

// GetCapabilities returns the capabilities of this extractor
func (t *LLMTopicExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes:     []types.EntityType{},
		SupportedCitationFormats: []types.CitationFormat{},
		SupportedDocumentTypes: []types.DocumentType{
			types.DocumentTypeResearch,
			types.DocumentTypeArticle,
			types.DocumentTypeReport,
			types.DocumentTypeBusiness,
			types.DocumentTypePersonal,
		},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		SupportsRelationships:    false,
		RequiresExternalService:  true,
		MaxDocumentSize:          10 * 1024 * 1024, // 10MB
		AverageProcessingTime:    time.Second * 3,
	}
}

// GetMetrics returns current extractor metrics
func (t *LLMTopicExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return *t.metrics
}

// Validate validates the configuration
func (t *LLMTopicExtractor) Validate(config *interfaces.TopicExtractionConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if config.MinConfidence < 0 || config.MinConfidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}
	if config.NumTopics < 1 {
		return fmt.Errorf("number of topics must be at least 1")
	}
	return nil
}

// Close cleans up resources
func (t *LLMTopicExtractor) Close() error {
	slog.Info("Closing LLM topic extractor")
	return nil
}

// updateMetrics updates the internal metrics
func (t *LLMTopicExtractor) updateMetrics(processingTime time.Duration, success bool) {
	t.metrics.TotalDocuments++
	t.metrics.ProcessingTime = (t.metrics.ProcessingTime + processingTime) / time.Duration(t.metrics.TotalDocuments)

	if !success {
		t.metrics.ErrorRate = (t.metrics.ErrorRate*float64(t.metrics.TotalDocuments-1) + 1.0) / float64(t.metrics.TotalDocuments)
	} else {
		t.metrics.ErrorRate = (t.metrics.ErrorRate * float64(t.metrics.TotalDocuments-1)) / float64(t.metrics.TotalDocuments)
	}
}
