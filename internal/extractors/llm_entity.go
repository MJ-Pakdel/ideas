package extractors

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// LLMEntityExtractor implements entity extraction using Large Language Models
// This is a lightweight wrapper around the unified extractor for interface compatibility
type LLMEntityExtractor struct {
	config           *interfaces.EntityExtractionConfig
	llmClient        interfaces.LLMClient
	metrics          *interfaces.ExtractorMetrics
	unifiedExtractor *LLMUnifiedExtractor
}

// NewLLMEntityExtractor creates a new LLM-based entity extractor
func NewLLMEntityExtractor(config *interfaces.EntityExtractionConfig, llmClient interfaces.LLMClient) *LLMEntityExtractor {
	if config == nil {
		config = &interfaces.EntityExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM,
			MinConfidence: 0.7,
			MaxEntities:   50,
		}
	}

	// Create unified extractor config from entity config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:   0.3,
		MaxTokens:     2048,
		MinConfidence: config.MinConfidence,
		MaxEntities:   config.MaxEntities,
		EntityTypes:   config.EntityTypes,
		AnalysisDepth: "detailed",
	}

	return &LLMEntityExtractor{
		config:           config,
		llmClient:        llmClient,
		unifiedExtractor: NewLLMUnifiedExtractor(unifiedConfig, llmClient),
		metrics: &interfaces.ExtractorMetrics{
			ProcessingTime:       0,
			TotalDocuments:       0,
			TotalEntities:        0,
			AverageConfidence:    0.0,
			ErrorRate:            0.0,
			CacheHitRate:         0.0,
			ExternalServiceCalls: 0,
			MemoryUsage:          0,
		},
	}
}

// Extract implements the EntityExtractor interface
// This uses the unified extractor and filters results to return only entities
func (e *LLMEntityExtractor) Extract(ctx context.Context, document *types.Document, config *interfaces.EntityExtractionConfig) (*interfaces.EntityExtractionResult, error) {
	startTime := time.Now()

	slog.InfoContext(ctx, "Starting LLM entity extraction via unified extractor", "document_id", document.ID)

	// Use provided config or fall back to instance config
	if config == nil {
		config = e.config
	}

	// Create unified config from entity config
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:   0.3,
		MaxTokens:     2048,
		MinConfidence: config.MinConfidence,
		MaxEntities:   config.MaxEntities,
		EntityTypes:   config.EntityTypes,
		MaxCitations:  0, // We don't need citations for entity extraction
		MaxTopics:     0, // We don't need topics for entity extraction
		AnalysisDepth: "detailed",
	}

	// Call unified extractor
	unifiedResult, err := e.unifiedExtractor.ExtractAll(ctx, document, unifiedConfig)
	if err != nil {
		e.updateMetrics(time.Since(startTime), false)
		return nil, fmt.Errorf("unified extraction failed: %w", err)
	}

	// Extract only entities from unified result
	result := &interfaces.EntityExtractionResult{
		Entities: unifiedResult.Entities,
		ExtractionMetadata: interfaces.ExtractionMetadata{
			ProcessingTime: time.Since(startTime),
			Method:         types.ExtractorMethodLLM,
			Confidence:     unifiedResult.Confidence,
			CacheHit:       false,
		},
	}

	// Update our own metrics
	e.metrics.TotalEntities += int64(len(result.Entities))
	e.updateMetrics(time.Since(startTime), true)

	slog.InfoContext(ctx, "Completed LLM entity extraction",
		"document_id", document.ID,
		"entities_found", len(result.Entities),
		"processing_time", time.Since(startTime))

	return result, nil
}

// GetCapabilities returns the capabilities of this extractor
func (e *LLMEntityExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes: []types.EntityType{
			types.EntityTypePerson,
			types.EntityTypeOrganization,
			types.EntityTypeLocation,
			types.EntityTypeDate,
			types.EntityTypeEmail,
			types.EntityTypeConcept,
			types.EntityTypeMetric,
			types.EntityTypeTechnology,
		},
		SupportedCitationFormats: []types.CitationFormat{},
		SupportedDocumentTypes:   []types.DocumentType{types.DocumentTypeResearch, types.DocumentTypeArticle, types.DocumentTypeReport, types.DocumentTypeBusiness, types.DocumentTypePersonal},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		SupportsRelationships:    false,
		RequiresExternalService:  true,
		MaxDocumentSize:          10 * 1024 * 1024, // 10MB
		AverageProcessingTime:    time.Second * 3,
	}
}

// GetMetrics returns current extractor metrics
func (e *LLMEntityExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return *e.metrics
}

// Validate validates the configuration
func (e *LLMEntityExtractor) Validate(config *interfaces.EntityExtractionConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	if config.MinConfidence < 0 || config.MinConfidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1")
	}
	if config.MaxEntities < 1 {
		return fmt.Errorf("max entities must be at least 1")
	}
	return nil
}

// Close cleans up resources
func (e *LLMEntityExtractor) Close() error {
	slog.Info("Closing LLM entity extractor")
	return nil
}

// updateMetrics updates the internal metrics
func (e *LLMEntityExtractor) updateMetrics(processingTime time.Duration, success bool) {
	e.metrics.TotalDocuments++
	e.metrics.ProcessingTime = (e.metrics.ProcessingTime + processingTime) / time.Duration(e.metrics.TotalDocuments)

	if !success {
		e.metrics.ErrorRate = (e.metrics.ErrorRate*float64(e.metrics.TotalDocuments-1) + 1.0) / float64(e.metrics.TotalDocuments)
	} else {
		e.metrics.ErrorRate = (e.metrics.ErrorRate * float64(e.metrics.TotalDocuments-1)) / float64(e.metrics.TotalDocuments)
	}
}
