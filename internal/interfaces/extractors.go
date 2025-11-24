// Package interfaces defines the core interfaces for the IDAES intelligence system.package interfaces

// This package provides the abstraction layer that enables the strategy pattern
// and allows for different implementation approaches (regex, LLM, hybrid) to be
// used interchangeably.
package interfaces

import (
	"context"
	"time"

	"github.com/example/idaes/internal/types"
)

// ExtractorCapabilities describes what an extractor implementation can do
type ExtractorCapabilities struct {
	SupportedEntityTypes     []types.EntityType     `json:"supported_entity_types"`
	SupportedCitationFormats []types.CitationFormat `json:"supported_citation_formats"`
	SupportedDocumentTypes   []types.DocumentType   `json:"supported_document_types"`
	SupportsConfidenceScores bool                   `json:"supports_confidence_scores"`
	SupportsContext          bool                   `json:"supports_context"`
	SupportsRelationships    bool                   `json:"supports_relationships"`
	RequiresExternalService  bool                   `json:"requires_external_service"`
	MaxDocumentSize          int64                  `json:"max_document_size"`
	AverageProcessingTime    time.Duration          `json:"average_processing_time"`
}

// ExtractorMetrics provides performance and quality metrics for extractors
type ExtractorMetrics struct {
	ProcessingTime       time.Duration `json:"processing_time"`
	TotalDocuments       int64         `json:"total_documents"`
	TotalEntities        int64         `json:"total_entities"`
	TotalCitations       int64         `json:"total_citations"`
	AverageConfidence    float64       `json:"average_confidence"`
	ErrorRate            float64       `json:"error_rate"`
	LastError            error         `json:"last_error,omitempty"`
	CacheHitRate         float64       `json:"cache_hit_rate"`
	ExternalServiceCalls int64         `json:"external_service_calls"`
	MemoryUsage          int64         `json:"memory_usage"`
}

// EntityExtractionConfig contains configuration for entity extraction
type EntityExtractionConfig struct {
	Enabled        bool                  `json:"enabled" yaml:"enabled"`
	Method         types.ExtractorMethod `json:"method" yaml:"method"`
	MinConfidence  float64               `json:"min_confidence" yaml:"min_confidence"`
	MaxEntities    int                   `json:"max_entities" yaml:"max_entities"`
	IncludeContext bool                  `json:"include_context" yaml:"include_context"`
	Deduplicate    bool                  `json:"deduplicate" yaml:"deduplicate"`
	EntityTypes    []types.EntityType    `json:"entity_types" yaml:"entity_types"`

	// Hybrid method settings
	PrimaryMethod  types.ExtractorMethod `json:"primary_method,omitempty" yaml:"primary_method,omitempty"`
	FallbackMethod types.ExtractorMethod `json:"fallback_method,omitempty" yaml:"fallback_method,omitempty"`
	HybridStrategy string                `json:"hybrid_strategy,omitempty" yaml:"hybrid_strategy,omitempty"` // "prefilter", "fallback", "voting"

	// LLM-specific settings
	LLMConfig *LLMConfig `json:"llm_config,omitempty" yaml:"llm_config,omitempty"`

	// Caching settings
	CacheEnabled bool          `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
}

// CitationExtractionConfig contains configuration for citation extraction
type CitationExtractionConfig struct {
	Enabled            bool                   `json:"enabled" yaml:"enabled"`
	Method             types.ExtractorMethod  `json:"method" yaml:"method"`
	MinConfidence      float64                `json:"min_confidence" yaml:"min_confidence"`
	MaxCitations       int                    `json:"max_citations" yaml:"max_citations"`
	Formats            []types.CitationFormat `json:"formats" yaml:"formats"`
	EnrichWithDOI      bool                   `json:"enrich_with_doi" yaml:"enrich_with_doi"`
	EnrichWithMetadata bool                   `json:"enrich_with_metadata" yaml:"enrich_with_metadata"`
	ValidateReferences bool                   `json:"validate_references" yaml:"validate_references"`

	// Hybrid method settings
	PrimaryMethod  types.ExtractorMethod `json:"primary_method,omitempty" yaml:"primary_method,omitempty"`
	FallbackMethod types.ExtractorMethod `json:"fallback_method,omitempty" yaml:"fallback_method,omitempty"`
	HybridStrategy string                `json:"hybrid_strategy,omitempty" yaml:"hybrid_strategy,omitempty"`

	// LLM-specific settings
	LLMConfig *LLMConfig `json:"llm_config,omitempty" yaml:"llm_config,omitempty"`

	// DOI resolution settings
	DOIConfig *DOIConfig `json:"doi_config,omitempty" yaml:"doi_config,omitempty"`

	// Caching settings
	CacheEnabled bool          `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
}

// TopicExtractionConfig contains configuration for topic modeling
type TopicExtractionConfig struct {
	Enabled          bool    `json:"enabled" yaml:"enabled"`
	Method           string  `json:"method" yaml:"method"` // "statistical", "semantic", "llm", "hybrid"
	NumTopics        int     `json:"num_topics" yaml:"num_topics"`
	MinConfidence    float64 `json:"min_confidence" yaml:"min_confidence"`
	MaxKeywords      int     `json:"max_keywords" yaml:"max_keywords"`
	EnableClustering bool    `json:"enable_clustering" yaml:"enable_clustering"`

	// LLM-specific settings
	LLMConfig *LLMConfig `json:"llm_config,omitempty" yaml:"llm_config,omitempty"`

	// Caching settings
	CacheEnabled bool          `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
}

// LLMConfig contains configuration for LLM-based extraction
type LLMConfig struct {
	Provider       string        `json:"provider" yaml:"provider"`               // "ollama", "openai", etc.
	BaseURL        string        `json:"base_url" yaml:"base_url"`               // Service URL
	Model          string        `json:"model" yaml:"model"`                     // Model name
	EmbeddingModel string        `json:"embedding_model" yaml:"embedding_model"` // Embedding model
	Temperature    float64       `json:"temperature" yaml:"temperature"`         // Generation temperature
	MaxTokens      int           `json:"max_tokens" yaml:"max_tokens"`           // Maximum tokens
	Timeout        time.Duration `json:"timeout" yaml:"timeout"`                 // Request timeout
	RetryAttempts  int           `json:"retry_attempts" yaml:"retry_attempts"`   // Retry attempts
	SystemPrompt   string        `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	UserPrompt     string        `json:"user_prompt,omitempty" yaml:"user_prompt,omitempty"`
}

// UnifiedExtractionConfig contains configuration for unified document analysis
type UnifiedExtractionConfig struct {
	// General settings
	Temperature   float64 `json:"temperature" yaml:"temperature"`
	MaxTokens     int     `json:"max_tokens" yaml:"max_tokens"`
	MinConfidence float64 `json:"min_confidence" yaml:"min_confidence"`

	// Entity extraction settings
	MaxEntities int                `json:"max_entities" yaml:"max_entities"`
	EntityTypes []types.EntityType `json:"entity_types" yaml:"entity_types"`

	// Citation extraction settings
	MaxCitations    int                    `json:"max_citations" yaml:"max_citations"`
	CitationFormats []types.CitationFormat `json:"citation_formats" yaml:"citation_formats"`

	// Topic extraction settings
	MaxTopics        int `json:"max_topics" yaml:"max_topics"`
	TopicMinKeywords int `json:"topic_min_keywords" yaml:"topic_min_keywords"`

	// Classification settings
	ClassificationTypes []types.DocumentType `json:"classification_types" yaml:"classification_types"`

	// Analysis settings
	IncludeMetadata bool   `json:"include_metadata" yaml:"include_metadata"`
	AnalysisDepth   string `json:"analysis_depth" yaml:"analysis_depth"` // "basic", "detailed", "comprehensive"

	// LLM-specific settings
	LLMConfig *LLMConfig `json:"llm_config,omitempty" yaml:"llm_config,omitempty"`

	// Caching settings
	CacheEnabled bool          `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
}

// DOIConfig contains configuration for DOI resolution
type DOIConfig struct {
	Enabled      bool          `json:"enabled" yaml:"enabled"`
	CrossRefURL  string        `json:"crossref_url" yaml:"crossref_url"`
	Timeout      time.Duration `json:"timeout" yaml:"timeout"`
	UserAgent    string        `json:"user_agent" yaml:"user_agent"`
	RateLimit    time.Duration `json:"rate_limit" yaml:"rate_limit"`
	CacheEnabled bool          `json:"cache_enabled" yaml:"cache_enabled"`
	MaxCacheSize int           `json:"max_cache_size" yaml:"max_cache_size"`
}

// ExtractionMetadata contains common metadata for all extraction results
type ExtractionMetadata struct {
	ProcessingTime time.Duration         `json:"processing_time"`
	Method         types.ExtractorMethod `json:"method"`
	Confidence     float64               `json:"confidence"`
	Metadata       map[string]any        `json:"metadata,omitempty"`
	CacheHit       bool                  `json:"cache_hit"`
	Error          error                 `json:"error,omitempty"`
}

// EntityExtractionResult contains the result of entity extraction
type EntityExtractionResult struct {
	Entities []*types.Entity `json:"entities"`
	ExtractionMetadata
}

// CitationExtractionResult contains the result of citation extraction
type CitationExtractionResult struct {
	Citations []*types.Citation `json:"citations"`
	ExtractionMetadata
}

// TopicExtractionResult contains the result of topic extraction
type TopicExtractionResult struct {
	Topics []*types.Topic `json:"topics"`
	ExtractionMetadata
}

// DocumentClassificationResult contains the result of document classification
type DocumentClassificationResult struct {
	Classification *types.DocumentClassification `json:"classification"`
	ExtractionMetadata
}

// UnifiedExtractionResult contains the result of unified document analysis
type UnifiedExtractionResult struct {
	Entities       []*types.Entity               `json:"entities,omitempty"`
	Citations      []*types.Citation             `json:"citations,omitempty"`
	Topics         []*types.Topic                `json:"topics,omitempty"`
	Classification *types.DocumentClassification `json:"classification,omitempty"`
	Summary        string                        `json:"summary,omitempty"`
	QualityMetrics *QualityMetrics               `json:"quality_metrics,omitempty"`
	ExtractionMetadata
}

// QualityMetrics contains quality assessment for extraction results
type QualityMetrics struct {
	ConfidenceScore   float64 `json:"confidence_score"`   // Overall confidence (0-1)
	CompletenessScore float64 `json:"completeness_score"` // How complete the extraction appears (0-1)
	ReliabilityScore  float64 `json:"reliability_score"`  // How reliable the results are (0-1)
}

// EntityExtractor defines the interface for entity extraction
type EntityExtractor interface {
	// Extract entities from a document
	Extract(ctx context.Context, document *types.Document, config *EntityExtractionConfig) (*EntityExtractionResult, error)

	// Get capabilities of this extractor
	GetCapabilities() ExtractorCapabilities

	// Get performance and quality metrics
	GetMetrics() ExtractorMetrics

	// Validate configuration
	Validate(config *EntityExtractionConfig) error

	// Close and cleanup resources
	Close() error
}

// CitationExtractor defines the interface for citation extraction
type CitationExtractor interface {
	// Extract citations from a document
	Extract(ctx context.Context, document *types.Document, config *CitationExtractionConfig) (*CitationExtractionResult, error)

	// Get capabilities of this extractor
	GetCapabilities() ExtractorCapabilities

	// Get performance and quality metrics
	GetMetrics() ExtractorMetrics

	// Validate configuration
	Validate(config *CitationExtractionConfig) error

	// Close and cleanup resources
	Close() error
}

// TopicExtractor defines the interface for topic extraction/modeling
type TopicExtractor interface {
	// Extract topics from a document
	Extract(ctx context.Context, document *types.Document, config *TopicExtractionConfig) (*TopicExtractionResult, error)

	// Get capabilities of this extractor
	GetCapabilities() ExtractorCapabilities

	// Get performance and quality metrics
	GetMetrics() ExtractorMetrics

	// Validate configuration
	Validate(config *TopicExtractionConfig) error

	// Close and cleanup resources
	Close() error
}

// DocumentClassifier defines the interface for document classification
type DocumentClassifier interface {
	// Classify a document
	Classify(ctx context.Context, document *types.Document) (*DocumentClassificationResult, error)

	// Get capabilities of this classifier
	GetCapabilities() ExtractorCapabilities

	// Get performance and quality metrics
	GetMetrics() ExtractorMetrics

	// Close and cleanup resources
	Close() error
}

// UnifiedExtractor defines the interface for comprehensive document analysis in a single operation
type UnifiedExtractor interface {
	// ExtractAll performs comprehensive analysis and returns all results in a single operation
	ExtractAll(ctx context.Context, document *types.Document, config *UnifiedExtractionConfig) (*UnifiedExtractionResult, error)

	// Get capabilities of this unified extractor
	GetCapabilities() ExtractorCapabilities

	// Get performance and quality metrics
	GetMetrics() ExtractorMetrics

	// Close and cleanup resources
	Close() error
}

// AnalysisConfig contains configuration for comprehensive document analysis
type AnalysisConfig struct {
	EntityConfig         *EntityExtractionConfig   `json:"entity_config,omitempty" yaml:"entity_config,omitempty"`
	CitationConfig       *CitationExtractionConfig `json:"citation_config,omitempty" yaml:"citation_config,omitempty"`
	TopicConfig          *TopicExtractionConfig    `json:"topic_config,omitempty" yaml:"topic_config,omitempty"`
	ClassifyDocument     bool                      `json:"classify_document" yaml:"classify_document"`
	ExtractRelationships bool                      `json:"extract_relationships" yaml:"extract_relationships"`
	GenerateSummary      bool                      `json:"generate_summary" yaml:"generate_summary"`
	EnableCaching        bool                      `json:"enable_caching" yaml:"enable_caching"`
	Parallel             bool                      `json:"parallel" yaml:"parallel"` // Run extractions in parallel
}

// ExtractorFactory creates and manages extractors using the strategy pattern
type ExtractorFactory interface {
	// Create entity extractor with specified method and configuration
	CreateEntityExtractor(method types.ExtractorMethod, config *EntityExtractionConfig) (EntityExtractor, error)

	// Create citation extractor with specified method and configuration
	CreateCitationExtractor(method types.ExtractorMethod, config *CitationExtractionConfig) (CitationExtractor, error)

	// Create topic extractor with specified method and configuration
	CreateTopicExtractor(method string, config *TopicExtractionConfig) (TopicExtractor, error)

	// Close and cleanup all managed extractors
	Close() error
}

// LLMClient defines the interface for LLM service clients
type LLMClient interface {
	// Generate text completion
	Complete(ctx context.Context, prompt string, options *LLMOptions) (string, error)

	// Generate embeddings for text
	Embed(ctx context.Context, text string) ([]float64, error)

	// Generate embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

	// Check if the client is healthy
	Health(ctx context.Context) error

	// Get model information
	GetModelInfo() ModelInfo

	// Close client connection
	Close() error
}

// LLMOptions contains options for LLM requests
type LLMOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

// ModelInfo contains information about an LLM model
type ModelInfo struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Version      string `json:"version"`
	ContextSize  int    `json:"context_size"`
	EmbeddingDim int    `json:"embedding_dim,omitempty"`
}

// CollectionInfo provides information about a storage collection
type CollectionInfo struct {
	Name      string         `json:"name" yaml:"name"`
	Metadata  map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Documents int            `json:"documents" yaml:"documents"`
}

// StorageStats provides overall storage statistics
type StorageStats struct {
	Collections []CollectionInfo `json:"collections" yaml:"collections"`
	Metrics     map[string]any   `json:"metrics" yaml:"metrics"`
}

// StorageManager defines the interface for storage operations
type StorageManager interface {
	// Initialization
	Initialize(ctx context.Context) error

	// Document operations
	StoreDocument(ctx context.Context, document *types.Document) error
	StoreDocumentInCollection(ctx context.Context, collection string, document *types.Document) error
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error)
	DeleteDocument(ctx context.Context, id string) error
	DeleteDocumentFromCollection(ctx context.Context, collection, id string) error

	// Search operations
	SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error)
	SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error)
	SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error)
	SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error)
	SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error)

	// Entity, Citation, and Topic operations
	StoreEntity(ctx context.Context, entity *types.Entity) error
	StoreCitation(ctx context.Context, citation *types.Citation) error
	StoreTopic(ctx context.Context, topic *types.Topic) error

	// Collection operations
	ListCollections(ctx context.Context) ([]CollectionInfo, error)
	GetStats(ctx context.Context) (*StorageStats, error)

	// Health check
	Health(ctx context.Context) error

	// Close storage connection
	Close() error
}

// UnifiedAnalysisAdapter provides conversion methods between unified and individual extraction results
type UnifiedAnalysisAdapter interface {
	// Convert unified result to individual extraction results for backward compatibility
	ToEntityExtractionResult(unified *UnifiedExtractionResult) *EntityExtractionResult
	ToCitationExtractionResult(unified *UnifiedExtractionResult) *CitationExtractionResult
	ToTopicExtractionResult(unified *UnifiedExtractionResult) *TopicExtractionResult
	ToDocumentClassificationResult(unified *UnifiedExtractionResult) *DocumentClassificationResult

	// Convert individual results to unified result for forward compatibility
	FromIndividualResults(
		entities *EntityExtractionResult,
		citations *CitationExtractionResult,
		topics *TopicExtractionResult,
		classification *DocumentClassificationResult,
	) *UnifiedExtractionResult

	// Extract specific types of data from unified results
	ExtractEntities(unified *UnifiedExtractionResult) []*types.Entity
	ExtractCitations(unified *UnifiedExtractionResult) []*types.Citation
	ExtractTopics(unified *UnifiedExtractionResult) []*types.Topic
	ExtractClassification(unified *UnifiedExtractionResult) *types.DocumentClassification
}

// DefaultUnifiedAnalysisAdapter provides default implementation of UnifiedAnalysisAdapter
type DefaultUnifiedAnalysisAdapter struct{}

// NewUnifiedAnalysisAdapter creates a new unified analysis adapter
func NewUnifiedAnalysisAdapter() UnifiedAnalysisAdapter {
	return &DefaultUnifiedAnalysisAdapter{}
}

// ToEntityExtractionResult converts unified result to entity extraction result
func (a *DefaultUnifiedAnalysisAdapter) ToEntityExtractionResult(unified *UnifiedExtractionResult) *EntityExtractionResult {
	if unified == nil {
		return nil
	}

	return &EntityExtractionResult{
		Entities:           unified.Entities,
		ExtractionMetadata: unified.ExtractionMetadata,
	}
}

// ToCitationExtractionResult converts unified result to citation extraction result
func (a *DefaultUnifiedAnalysisAdapter) ToCitationExtractionResult(unified *UnifiedExtractionResult) *CitationExtractionResult {
	if unified == nil {
		return nil
	}

	return &CitationExtractionResult{
		Citations:          unified.Citations,
		ExtractionMetadata: unified.ExtractionMetadata,
	}
}

// ToTopicExtractionResult converts unified result to topic extraction result
func (a *DefaultUnifiedAnalysisAdapter) ToTopicExtractionResult(unified *UnifiedExtractionResult) *TopicExtractionResult {
	if unified == nil {
		return nil
	}

	return &TopicExtractionResult{
		Topics:             unified.Topics,
		ExtractionMetadata: unified.ExtractionMetadata,
	}
}

// ToDocumentClassificationResult converts unified result to document classification result
func (a *DefaultUnifiedAnalysisAdapter) ToDocumentClassificationResult(unified *UnifiedExtractionResult) *DocumentClassificationResult {
	if unified == nil {
		return nil
	}

	return &DocumentClassificationResult{
		Classification:     unified.Classification,
		ExtractionMetadata: unified.ExtractionMetadata,
	}
}

// FromIndividualResults converts individual extraction results to unified result
func (a *DefaultUnifiedAnalysisAdapter) FromIndividualResults(
	entities *EntityExtractionResult,
	citations *CitationExtractionResult,
	topics *TopicExtractionResult,
	classification *DocumentClassificationResult,
) *UnifiedExtractionResult {

	unified := &UnifiedExtractionResult{}

	// Combine entities
	if entities != nil {
		unified.Entities = entities.Entities
		unified.ExtractionMetadata = entities.ExtractionMetadata
	}

	// Combine citations
	if citations != nil {
		unified.Citations = citations.Citations
		// If no metadata from entities, use citations metadata
		if unified.ExtractionMetadata.ProcessingTime == 0 {
			unified.ExtractionMetadata = citations.ExtractionMetadata
		}
	}

	// Combine topics
	if topics != nil {
		unified.Topics = topics.Topics
		// If no metadata from previous, use topics metadata
		if unified.ExtractionMetadata.ProcessingTime == 0 {
			unified.ExtractionMetadata = topics.ExtractionMetadata
		}
	}

	// Combine classification
	if classification != nil {
		unified.Classification = classification.Classification
		// If no metadata from previous, use classification metadata
		if unified.ExtractionMetadata.ProcessingTime == 0 {
			unified.ExtractionMetadata = classification.ExtractionMetadata
		}
	}

	// Combine quality metrics if available
	if entities != nil || citations != nil || topics != nil {
		unified.QualityMetrics = &QualityMetrics{
			ConfidenceScore:   unified.ExtractionMetadata.Confidence,
			CompletenessScore: calculateCompleteness(entities, citations, topics),
			ReliabilityScore:  calculateReliability(entities, citations, topics),
		}
	}

	return unified
}

// ExtractEntities extracts entities from unified result
func (a *DefaultUnifiedAnalysisAdapter) ExtractEntities(unified *UnifiedExtractionResult) []*types.Entity {
	if unified == nil {
		return nil
	}
	return unified.Entities
}

// ExtractCitations extracts citations from unified result
func (a *DefaultUnifiedAnalysisAdapter) ExtractCitations(unified *UnifiedExtractionResult) []*types.Citation {
	if unified == nil {
		return nil
	}
	return unified.Citations
}

// ExtractTopics extracts topics from unified result
func (a *DefaultUnifiedAnalysisAdapter) ExtractTopics(unified *UnifiedExtractionResult) []*types.Topic {
	if unified == nil {
		return nil
	}
	return unified.Topics
}

// ExtractClassification extracts classification from unified result
func (a *DefaultUnifiedAnalysisAdapter) ExtractClassification(unified *UnifiedExtractionResult) *types.DocumentClassification {
	if unified == nil {
		return nil
	}
	return unified.Classification
}

// Helper functions for quality metrics calculation

// calculateCompleteness estimates how complete the extraction results are
func calculateCompleteness(entities *EntityExtractionResult, citations *CitationExtractionResult, topics *TopicExtractionResult) float64 {
	score := 0.0
	factors := 0.0

	if entities != nil {
		if len(entities.Entities) > 0 {
			score += 0.4 // 40% weight for entities
		}
		factors += 0.4
	}

	if citations != nil {
		if len(citations.Citations) > 0 {
			score += 0.3 // 30% weight for citations
		}
		factors += 0.3
	}

	if topics != nil {
		if len(topics.Topics) > 0 {
			score += 0.3 // 30% weight for topics
		}
		factors += 0.3
	}

	if factors > 0 {
		return score / factors
	}
	return 0.0
}

// calculateReliability estimates the reliability of the extraction results
func calculateReliability(entities *EntityExtractionResult, citations *CitationExtractionResult, topics *TopicExtractionResult) float64 {
	scores := []float64{}

	if entities != nil {
		scores = append(scores, entities.Confidence)
	}

	if citations != nil {
		scores = append(scores, citations.Confidence)
	}

	if topics != nil {
		scores = append(scores, topics.Confidence)
	}

	if len(scores) == 0 {
		return 0.0
	}

	// Calculate average confidence
	sum := 0.0
	for _, score := range scores {
		sum += score
	}
	return sum / float64(len(scores))
}
