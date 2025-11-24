// Package extractors provides LLM-based unified extraction using structured prompts.
package extractors

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// LLMUnifiedExtractor implements comprehensive document analysis using Large Language Models
type LLMUnifiedExtractor struct {
	config    *interfaces.UnifiedExtractionConfig
	llmClient interfaces.LLMClient
	metrics   *interfaces.ExtractorMetrics
	prompts   *PromptTemplates
}

// PromptTemplates contains structured prompts for different document types and analysis tasks
type PromptTemplates struct {
	// Document classification prompts
	ClassificationPrompt string

	// Entity extraction prompts by document type
	EntityPrompts map[types.DocumentType]string

	// Citation extraction prompts by document type
	CitationPrompts map[types.DocumentType]string

	// Topic extraction prompts by document type
	TopicPrompts map[types.DocumentType]string

	// Unified analysis prompts by document type
	UnifiedPrompts map[types.DocumentType]string
}

// LLMAnalysisResponse represents the structured JSON response from the LLM
type LLMAnalysisResponse struct {
	DocumentClassification *ClassificationResponse `json:"classification,omitempty"`
	Entities               []EntityResponse        `json:"entities,omitempty"`
	Citations              []CitationResponse      `json:"citations,omitempty"`
	Topics                 []TopicResponse         `json:"topics,omitempty"`
	Summary                string                  `json:"summary,omitempty"`
	QualityMetrics         *QualityResponse        `json:"quality_metrics,omitempty"`
}

// ClassificationResponse represents classification data from LLM
type ClassificationResponse struct {
	PrimaryType    string             `json:"primary_type"`
	SecondaryTypes []string           `json:"secondary_types,omitempty"`
	Confidence     float64            `json:"confidence"`
	Features       map[string]float64 `json:"features,omitempty"`
	Reasoning      string             `json:"reasoning,omitempty"`
}

// EntityResponse represents entity data from LLM
type EntityResponse struct {
	Type        string         `json:"type"`
	Text        string         `json:"text"`
	Confidence  float64        `json:"confidence"`
	StartOffset int            `json:"start_offset"`
	EndOffset   int            `json:"end_offset"`
	Context     string         `json:"context,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// CitationResponse represents citation data from LLM
type CitationResponse struct {
	Text        string         `json:"text"`
	Authors     []string       `json:"authors"`
	Title       string         `json:"title,omitempty"`
	Year        int            `json:"year,omitempty"`
	Journal     string         `json:"journal,omitempty"`
	Volume      string         `json:"volume,omitempty"`
	Issue       string         `json:"issue,omitempty"`
	Pages       string         `json:"pages,omitempty"`
	Publisher   string         `json:"publisher,omitempty"`
	URL         string         `json:"url,omitempty"`
	DOI         string         `json:"doi,omitempty"`
	Format      string         `json:"format"`
	Confidence  float64        `json:"confidence"`
	StartOffset int            `json:"start_offset"`
	EndOffset   int            `json:"end_offset"`
	Context     string         `json:"context,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TopicResponse represents topic data from LLM
type TopicResponse struct {
	Name        string         `json:"name"`
	Keywords    []string       `json:"keywords"`
	Confidence  float64        `json:"confidence"`
	Weight      float64        `json:"weight"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// QualityResponse represents quality metrics from LLM
type QualityResponse struct {
	ConfidenceScore   float64 `json:"confidence_score"`
	CompletenessScore float64 `json:"completeness_score"`
	ReliabilityScore  float64 `json:"reliability_score"`
}

// NewLLMUnifiedExtractor creates a new LLM-based unified extractor
func NewLLMUnifiedExtractor(config *interfaces.UnifiedExtractionConfig, llmClient interfaces.LLMClient) *LLMUnifiedExtractor {
	if config == nil {
		config = &interfaces.UnifiedExtractionConfig{
			Temperature:   0.3,
			MaxTokens:     2048,
			MinConfidence: 0.5,
			MaxEntities:   50,
			MaxCitations:  20,
			MaxTopics:     10,
			AnalysisDepth: "detailed",
		}
	}

	return &LLMUnifiedExtractor{
		config:    config,
		llmClient: llmClient,
		metrics: &interfaces.ExtractorMetrics{
			ProcessingTime:       time.Duration(0),
			TotalDocuments:       0,
			TotalEntities:        0,
			TotalCitations:       0,
			AverageConfidence:    0.0,
			ErrorRate:            0.0,
			CacheHitRate:         0.0,
			ExternalServiceCalls: 0,
			MemoryUsage:          0,
		},
		prompts: initializePromptTemplates(),
	}
}

// ExtractAll implements the UnifiedExtractor interface
func (e *LLMUnifiedExtractor) ExtractAll(ctx context.Context, document *types.Document, config *interfaces.UnifiedExtractionConfig) (*interfaces.UnifiedExtractionResult, error) {
	startTime := time.Now()

	// Use provided config or fall back to instance config
	if config == nil {
		config = e.config
	}

	slog.Info("Starting unified LLM extraction",
		"document_id", document.ID,
		"document_name", document.Name,
		"analysis_depth", config.AnalysisDepth)

	// First, classify the document to determine the appropriate prompt
	docType, err := e.classifyDocument(ctx, document, config)
	if err != nil {
		return nil, fmt.Errorf("document classification failed: %w", err)
	}

	// Perform unified analysis based on document type
	result, err := e.performUnifiedAnalysis(ctx, document, docType, config)
	if err != nil {
		return nil, fmt.Errorf("unified analysis failed: %w", err)
	}

	// Update metrics
	processingTime := time.Since(startTime)
	e.updateMetrics(processingTime, result != nil)

	result.ProcessingTime = processingTime
	result.Method = types.ExtractorMethodLLM

	slog.Info("Completed unified LLM extraction",
		"document_id", document.ID,
		"processing_time", processingTime,
		"entities_found", len(result.Entities),
		"citations_found", len(result.Citations),
		"topics_found", len(result.Topics))

	return result, nil
}

// classifyDocument performs initial document classification
func (e *LLMUnifiedExtractor) classifyDocument(ctx context.Context, document *types.Document, config *interfaces.UnifiedExtractionConfig) (types.DocumentType, error) {
	prompt := e.buildClassificationPrompt(document)

	options := &interfaces.LLMOptions{
		Temperature: config.Temperature,
		MaxTokens:   256, // Small response for classification
	}

	response, err := e.llmClient.Complete(ctx, prompt, options)
	if err != nil {
		return "", fmt.Errorf("LLM classification request failed: %w", err)
	}

	// Parse classification response
	classificationResult, err := e.parseClassificationResponse(response)
	if err != nil {
		slog.Warn("Failed to parse classification response, using default", "error", err)
		return types.DocumentTypeOther, nil
	}

	return e.mapDocumentType(classificationResult.PrimaryType), nil
}

// performUnifiedAnalysis conducts comprehensive analysis based on document type
func (e *LLMUnifiedExtractor) performUnifiedAnalysis(ctx context.Context, document *types.Document, docType types.DocumentType, config *interfaces.UnifiedExtractionConfig) (*interfaces.UnifiedExtractionResult, error) {
	prompt := e.buildUnifiedAnalysisPrompt(document, docType, config)

	options := &interfaces.LLMOptions{
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
	}

	response, err := e.llmClient.Complete(ctx, prompt, options)
	if err != nil {
		return nil, fmt.Errorf("LLM unified analysis request failed: %w", err)
	}

	// Parse the unified response
	analysisResponse, err := e.parseUnifiedResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Convert to UnifiedExtractionResult
	result := e.convertToUnifiedResult(document.ID, analysisResponse, docType, config)

	return result, nil
}

// GetCapabilities returns the capabilities of this extractor
func (e *LLMUnifiedExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
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
			types.DocumentTypeBook,
			types.DocumentTypePresentation,
			types.DocumentTypeLegal,
		},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		SupportsRelationships:    false, // Not implemented in this version
		RequiresExternalService:  true,
		MaxDocumentSize:          10 * 1024 * 1024, // 10MB
		AverageProcessingTime:    time.Second * 10, // Unified analysis takes longer
	}
}

// GetMetrics returns current extractor metrics
func (e *LLMUnifiedExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return *e.metrics
}

// Close cleans up resources
func (e *LLMUnifiedExtractor) Close() error {
	slog.Info("Closing LLM unified extractor")
	return e.llmClient.Close()
}

// updateMetrics updates the internal metrics
func (e *LLMUnifiedExtractor) updateMetrics(processingTime time.Duration, success bool) {
	e.metrics.TotalDocuments++
	e.metrics.ProcessingTime = (e.metrics.ProcessingTime + processingTime) / time.Duration(e.metrics.TotalDocuments)

	if !success {
		e.metrics.ErrorRate = (e.metrics.ErrorRate*float64(e.metrics.TotalDocuments-1) + 1.0) / float64(e.metrics.TotalDocuments)
	} else {
		e.metrics.ErrorRate = (e.metrics.ErrorRate * float64(e.metrics.TotalDocuments-1)) / float64(e.metrics.TotalDocuments)
	}
}

// mapDocumentType maps string response to DocumentType
func (e *LLMUnifiedExtractor) mapDocumentType(typeStr string) types.DocumentType {
	switch strings.ToLower(typeStr) {
	case "research_paper", "research", "academic":
		return types.DocumentTypeResearch
	case "business_document", "business":
		return types.DocumentTypeBusiness
	case "personal_document", "personal":
		return types.DocumentTypePersonal
	case "article":
		return types.DocumentTypeArticle
	case "report":
		return types.DocumentTypeReport
	case "book":
		return types.DocumentTypeBook
	case "presentation":
		return types.DocumentTypePresentation
	case "legal_document", "legal":
		return types.DocumentTypeLegal
	default:
		return types.DocumentTypeOther
	}
}
