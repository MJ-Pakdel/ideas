package extractors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// LLMCitationExtractor implements citation extraction using Large Language Models
// This is a lightweight wrapper around the unified extractor for interface compatibility
type LLMCitationExtractor struct {
	config           *interfaces.CitationExtractionConfig
	metrics          *interfaces.ExtractorMetrics
	unifiedExtractor *LLMUnifiedExtractor
}

// NewLLMCitationExtractor creates a new LLM-based citation extractor
func NewLLMCitationExtractor(config *interfaces.CitationExtractionConfig, llmClient interfaces.LLMClient) *LLMCitationExtractor {
	if config == nil {
		config = &interfaces.CitationExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM,
			MinConfidence: 0.7,
			MaxCitations:  20,
		}
	}

	// Create unified extractor config from citation config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:     0.3,
		MaxTokens:       2048,
		MinConfidence:   config.MinConfidence,
		MaxCitations:    config.MaxCitations,
		CitationFormats: config.Formats,
		AnalysisDepth:   "detailed",
	}

	return &LLMCitationExtractor{
		config:           config,
		unifiedExtractor: NewLLMUnifiedExtractor(unifiedConfig, llmClient),
		metrics: &interfaces.ExtractorMetrics{
			ProcessingTime:       0,
			TotalDocuments:       0,
			TotalCitations:       0,
			AverageConfidence:    0.0,
			ErrorRate:            0.0,
			CacheHitRate:         0.0,
			ExternalServiceCalls: 0,
			MemoryUsage:          0,
		},
	}
}

// Extract implements the CitationExtractor interface
// This uses the unified extractor and filters results to return only citations
func (c *LLMCitationExtractor) Extract(ctx context.Context, document *types.Document, config *interfaces.CitationExtractionConfig) (*interfaces.CitationExtractionResult, error) {
	startTime := time.Now()

	slog.InfoContext(ctx, "Starting LLM citation extraction via unified extractor", "document_id", document.ID)

	// Use provided config or fall back to instance config
	if config == nil {
		config = c.config
	}

	// Create unified config from citation config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:     0.3,
		MaxTokens:       2048,
		MinConfidence:   config.MinConfidence,
		MaxEntities:     0, // We don't need entities for citation extraction
		MaxCitations:    config.MaxCitations,
		CitationFormats: config.Formats,
		MaxTopics:       0, // We don't need topics for citation extraction
		AnalysisDepth:   "detailed",
	}

	// Call unified extractor
	unifiedResult, err := c.unifiedExtractor.ExtractAll(ctx, document, unifiedConfig)
	if err != nil {
		c.updateMetrics(time.Since(startTime), false)
		return nil, fmt.Errorf("unified extraction failed: %w", err)
	}

	// Extract only citations from unified result
	result := &interfaces.CitationExtractionResult{
		Citations: unifiedResult.Citations,
		ExtractionMetadata: interfaces.ExtractionMetadata{
			ProcessingTime: time.Since(startTime),
			Method:         types.ExtractorMethodLLM,
			Confidence:     unifiedResult.Confidence,
			CacheHit:       false,
		},
	}

	// Update our own metrics
	c.metrics.TotalCitations += int64(len(result.Citations))
	c.updateMetrics(time.Since(startTime), true)

	slog.InfoContext(ctx, "Completed LLM citation extraction",
		"document_id", document.ID,
		"citations_found", len(result.Citations),
		"processing_time", time.Since(startTime))

	return result, nil
}

// GetCapabilities returns the capabilities of this extractor
func (c *LLMCitationExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes: []types.EntityType{},
		SupportedCitationFormats: []types.CitationFormat{
			types.CitationFormatAPA,
			types.CitationFormatMLA,
			types.CitationFormatChicago,
			types.CitationFormatIEEE,
			types.CitationFormatAuto,
		},
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
func (c *LLMCitationExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return *c.metrics
}

// Validate validates the configuration
func (c *LLMCitationExtractor) Validate(config *interfaces.CitationExtractionConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if config.MinConfidence < 0 || config.MinConfidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}
	if config.MaxCitations < 1 {
		return fmt.Errorf("max citations must be at least 1")
	}
	return nil
}

// Close cleans up resources
func (c *LLMCitationExtractor) Close() error {
	slog.Info("Closing LLM citation extractor")
	return nil
}

// updateMetrics updates the internal metrics
func (c *LLMCitationExtractor) updateMetrics(processingTime time.Duration, success bool) {
	c.metrics.TotalDocuments++
	c.metrics.ProcessingTime = (c.metrics.ProcessingTime + processingTime) / time.Duration(c.metrics.TotalDocuments)

	if !success {
		c.metrics.ErrorRate = (c.metrics.ErrorRate*float64(c.metrics.TotalDocuments-1) + 1.0) / float64(c.metrics.TotalDocuments)
	} else {
		c.metrics.ErrorRate = (c.metrics.ErrorRate * float64(c.metrics.TotalDocuments-1)) / float64(c.metrics.TotalDocuments)
	}
}
