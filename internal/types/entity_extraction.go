package types

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExtractionInput represents input parameters for entity extraction
type ExtractionInput struct {
	DocumentID          string         `json:"document_id"`
	DocumentName        string         `json:"document_name"`
	Content             string         `json:"content"`
	DocumentType        DocumentType   `json:"document_type"`
	AnalysisDepth       AnalysisDepth  `json:"analysis_depth"`
	RequiredEntityTypes []EntityType   `json:"required_entity_types,omitempty"`
	Options             map[string]any `json:"options,omitempty"`
}

// EntityExtractionResult represents the complete result of entity extraction with enhanced metadata
type EntityExtractionResult struct {
	ID                 string             `json:"id" yaml:"id"`
	DocumentID         string             `json:"document_id" yaml:"document_id"`
	DocumentType       DocumentType       `json:"document_type" yaml:"document_type"`
	Entities           []*ExtractedEntity `json:"entities" yaml:"entities"`
	TotalEntities      int                `json:"total_entities" yaml:"total_entities"`
	ProcessingTime     string             `json:"processing_time" yaml:"processing_time"`
	ProcessingTimeMs   int64              `json:"processing_time_ms" yaml:"processing_time_ms"`
	QualityMetrics     *QualityMetrics    `json:"quality_metrics" yaml:"quality_metrics"`
	ExtractionMetadata map[string]any     `json:"extraction_metadata,omitempty" yaml:"extraction_metadata,omitempty"`
	ExtractorVersion   string             `json:"extractor_version" yaml:"extractor_version"`
	ModelUsed          string             `json:"model_used" yaml:"model_used"`
	PromptVersion      string             `json:"prompt_version" yaml:"prompt_version"`
	CreatedAt          time.Time          `json:"created_at" yaml:"created_at"`
	Method             ExtractorMethod    `json:"method" yaml:"method"`
}

// ExtractedEntity represents an entity with enhanced metadata and context
type ExtractedEntity struct {
	ID              string                `json:"id" yaml:"id"`
	Type            EntityType            `json:"type" yaml:"type"`
	Text            string                `json:"text" yaml:"text"`
	NormalizedText  string                `json:"normalized_text" yaml:"normalized_text"`
	Confidence      float64               `json:"confidence" yaml:"confidence"`
	StartOffset     int                   `json:"start_offset" yaml:"start_offset"`
	EndOffset       int                   `json:"end_offset" yaml:"end_offset"`
	Context         string                `json:"context" yaml:"context"`
	ContextBefore   string                `json:"context_before,omitempty" yaml:"context_before,omitempty"`
	ContextAfter    string                `json:"context_after,omitempty" yaml:"context_after,omitempty"`
	Section         string                `json:"section,omitempty" yaml:"section,omitempty"`
	EntityMetadata  *EntityExtractionMeta `json:"entity_metadata,omitempty" yaml:"entity_metadata,omitempty"`
	ValidationFlags *ValidationFlags      `json:"validation_flags,omitempty" yaml:"validation_flags,omitempty"`
	DocumentID      string                `json:"document_id" yaml:"document_id"`
	ExtractedAt     time.Time             `json:"extracted_at" yaml:"extracted_at"`
	ExtractionID    string                `json:"extraction_id" yaml:"extraction_id"`
}

// EntityExtractionMeta contains type-specific metadata for entities
type EntityExtractionMeta struct {
	// Person-specific fields
	PersonRole    string   `json:"person_role,omitempty" yaml:"person_role,omitempty"`
	PersonTitle   string   `json:"person_title,omitempty" yaml:"person_title,omitempty"`
	PersonAliases []string `json:"person_aliases,omitempty" yaml:"person_aliases,omitempty"`

	// Organization-specific fields
	OrgType   string `json:"org_type,omitempty" yaml:"org_type,omitempty"`
	OrgSector string `json:"org_sector,omitempty" yaml:"org_sector,omitempty"`
	OrgSize   string `json:"org_size,omitempty" yaml:"org_size,omitempty"`

	// Location-specific fields
	LocationType string  `json:"location_type,omitempty" yaml:"location_type,omitempty"`
	Country      string  `json:"country,omitempty" yaml:"country,omitempty"`
	Coordinates  *LatLng `json:"coordinates,omitempty" yaml:"coordinates,omitempty"`

	// Date-specific fields
	DateType      string     `json:"date_type,omitempty" yaml:"date_type,omitempty"`
	DatePrecision string     `json:"date_precision,omitempty" yaml:"date_precision,omitempty"`
	ParsedDate    *time.Time `json:"parsed_date,omitempty" yaml:"parsed_date,omitempty"`

	// Technology/Concept-specific fields
	TechCategory  string   `json:"tech_category,omitempty" yaml:"tech_category,omitempty"`
	ConceptDomain string   `json:"concept_domain,omitempty" yaml:"concept_domain,omitempty"`
	Synonyms      []string `json:"synonyms,omitempty" yaml:"synonyms,omitempty"`

	// Metric-specific fields
	MetricValue *float64 `json:"metric_value,omitempty" yaml:"metric_value,omitempty"`
	MetricUnit  string   `json:"metric_unit,omitempty" yaml:"metric_unit,omitempty"`
	MetricType  string   `json:"metric_type,omitempty" yaml:"metric_type,omitempty"`

	// General fields
	Category     string            `json:"category,omitempty" yaml:"category,omitempty"`
	Subcategory  string            `json:"subcategory,omitempty" yaml:"subcategory,omitempty"`
	Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExternalRefs map[string]string `json:"external_refs,omitempty" yaml:"external_refs,omitempty"`
	CustomFields map[string]any    `json:"custom_fields,omitempty" yaml:"custom_fields,omitempty"`
}

// LatLng represents geographical coordinates
type LatLng struct {
	Latitude  float64 `json:"latitude" yaml:"latitude"`
	Longitude float64 `json:"longitude" yaml:"longitude"`
}

// ValidationFlags contains validation results for an entity
type ValidationFlags struct {
	IsValid            bool     `json:"is_valid" yaml:"is_valid"`
	HasValidOffsets    bool     `json:"has_valid_offsets" yaml:"has_valid_offsets"`
	HasValidType       bool     `json:"has_valid_type" yaml:"has_valid_type"`
	HasValidContext    bool     `json:"has_valid_context" yaml:"has_valid_context"`
	IsDuplicate        bool     `json:"is_duplicate" yaml:"is_duplicate"`
	ValidationErrors   []string `json:"validation_errors,omitempty" yaml:"validation_errors,omitempty"`
	ValidationWarnings []string `json:"validation_warnings,omitempty" yaml:"validation_warnings,omitempty"`
}

// QualityMetrics provides comprehensive quality assessment for extraction results
type QualityMetrics struct {
	OverallScore       float64                `json:"overall_score" yaml:"overall_score"`
	ConfidenceScore    float64                `json:"confidence_score" yaml:"confidence_score"`
	CompletenessScore  float64                `json:"completeness_score" yaml:"completeness_score"`
	ReliabilityScore   float64                `json:"reliability_score" yaml:"reliability_score"`
	ConsistencyScore   float64                `json:"consistency_score" yaml:"consistency_score"`
	ProcessingTimeMs   int64                  `json:"processing_time_ms" yaml:"processing_time_ms"`
	EntityTypeScores   map[EntityType]float64 `json:"entity_type_scores,omitempty" yaml:"entity_type_scores,omitempty"`
	ValidationResults  *ValidationResults     `json:"validation_results,omitempty" yaml:"validation_results,omitempty"`
	PerformanceMetrics *PerformanceMetrics    `json:"performance_metrics,omitempty" yaml:"performance_metrics,omitempty"`
}

// ValidationResults contains detailed validation outcomes
type ValidationResults struct {
	TotalEntities     int     `json:"total_entities" yaml:"total_entities"`
	ValidEntities     int     `json:"valid_entities" yaml:"valid_entities"`
	InvalidEntities   int     `json:"invalid_entities" yaml:"invalid_entities"`
	DuplicateEntities int     `json:"duplicate_entities" yaml:"duplicate_entities"`
	ValidationRate    float64 `json:"validation_rate" yaml:"validation_rate"`
	ErrorCount        int     `json:"error_count" yaml:"error_count"`
	WarningCount      int     `json:"warning_count" yaml:"warning_count"`
}

// PerformanceMetrics contains performance-related measurements
type PerformanceMetrics struct {
	MemoryUsageBytes   int64   `json:"memory_usage_bytes" yaml:"memory_usage_bytes"`
	CPUTimeMs          int64   `json:"cpu_time_ms" yaml:"cpu_time_ms"`
	TokensProcessed    int     `json:"tokens_processed" yaml:"tokens_processed"`
	TokensGenerated    int     `json:"tokens_generated" yaml:"tokens_generated"`
	CacheHitRate       float64 `json:"cache_hit_rate" yaml:"cache_hit_rate"`
	APICallCount       int     `json:"api_call_count" yaml:"api_call_count"`
	RetryCount         int     `json:"retry_count" yaml:"retry_count"`
	ErrorRecoveryCount int     `json:"error_recovery_count" yaml:"error_recovery_count"`
}

// ExtractionRequest represents a request for entity extraction
type ExtractionRequest struct {
	ID              string             `json:"id" yaml:"id"`
	DocumentID      string             `json:"document_id" yaml:"document_id"`
	DocumentContent string             `json:"document_content" yaml:"document_content"`
	DocumentType    DocumentType       `json:"document_type" yaml:"document_type"`
	MaxEntities     int                `json:"max_entities" yaml:"max_entities"`
	MinConfidence   float64            `json:"min_confidence" yaml:"min_confidence"`
	EntityTypes     []EntityType       `json:"entity_types,omitempty" yaml:"entity_types,omitempty"`
	Options         *ExtractionOptions `json:"options,omitempty" yaml:"options,omitempty"`
	Metadata        map[string]any     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt       time.Time          `json:"created_at" yaml:"created_at"`
}

// ExtractionOptions contains optional settings for extraction
type ExtractionOptions struct {
	EnableCaching         bool             `json:"enable_caching" yaml:"enable_caching"`
	StrictValidation      bool             `json:"strict_validation" yaml:"strict_validation"`
	IncludeContext        bool             `json:"include_context" yaml:"include_context"`
	IncludeMetadata       bool             `json:"include_metadata" yaml:"include_metadata"`
	AnalysisDepth         AnalysisDepth    `json:"analysis_depth" yaml:"analysis_depth"`
	ChunkSize             int              `json:"chunk_size,omitempty" yaml:"chunk_size,omitempty"`
	OverlapSize           int              `json:"overlap_size,omitempty" yaml:"overlap_size,omitempty"`
	EnableEntityLinking   bool             `json:"enable_entity_linking" yaml:"enable_entity_linking"`
	EnableCorefResolution bool             `json:"enable_coref_resolution" yaml:"enable_coref_resolution"`
	CustomPromptParams    map[string]any   `json:"custom_prompt_params,omitempty" yaml:"custom_prompt_params,omitempty"`
	ModelParameters       *ModelParameters `json:"model_parameters,omitempty" yaml:"model_parameters,omitempty"`
}

// AnalysisDepth defines the depth of analysis to perform
type AnalysisDepth string

const (
	AnalysisDepthBasic         AnalysisDepth = "basic"         // Fast, basic entity extraction
	AnalysisDepthStandard      AnalysisDepth = "standard"      // Standard extraction with context
	AnalysisDepthDetailed      AnalysisDepth = "detailed"      // Detailed analysis with metadata
	AnalysisDepthComprehensive AnalysisDepth = "comprehensive" // Full analysis with relationships
)

// ModelParameters contains LLM-specific parameters
type ModelParameters struct {
	Temperature      float64  `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	TopK             int      `json:"top_k,omitempty" yaml:"top_k,omitempty"`
	MaxTokens        int      `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
	StopSequences    []string `json:"stop_sequences,omitempty" yaml:"stop_sequences,omitempty"`
}

// ExtractionError represents an error that occurred during extraction
type ExtractionError struct {
	ID          string              `json:"id" yaml:"id"`
	RequestID   string              `json:"request_id" yaml:"request_id"`
	ErrorType   ExtractionErrorType `json:"error_type" yaml:"error_type"`
	Message     string              `json:"message" yaml:"message"`
	Details     string              `json:"details,omitempty" yaml:"details,omitempty"`
	Context     map[string]any      `json:"context,omitempty" yaml:"context,omitempty"`
	Recoverable bool                `json:"recoverable" yaml:"recoverable"`
	RetryCount  int                 `json:"retry_count" yaml:"retry_count"`
	Timestamp   time.Time           `json:"timestamp" yaml:"timestamp"`
}

// ExtractionErrorType defines types of extraction errors
type ExtractionErrorType string

const (
	ErrorTypeLLMUnavailable     ExtractionErrorType = "llm_unavailable"
	ErrorTypeLLMTimeout         ExtractionErrorType = "llm_timeout"
	ErrorTypeJSONParsing        ExtractionErrorType = "json_parsing"
	ErrorTypeValidation         ExtractionErrorType = "validation"
	ErrorTypeMemoryExhaustion   ExtractionErrorType = "memory_exhaustion"
	ErrorTypeConfigurationError ExtractionErrorType = "configuration_error"
	ErrorTypeContentTooLarge    ExtractionErrorType = "content_too_large"
	ErrorTypeUnknown            ExtractionErrorType = "unknown"
)

// NewExtractionRequest creates a new extraction request with generated ID and timestamp
func NewExtractionRequest(documentID, content string, docType DocumentType) *ExtractionRequest {
	return &ExtractionRequest{
		ID:              uuid.New().String(),
		DocumentID:      documentID,
		DocumentContent: content,
		DocumentType:    docType,
		MaxEntities:     50,  // Default
		MinConfidence:   0.7, // Default
		Metadata:        make(map[string]any),
		CreatedAt:       time.Now(),
	}
}

// NewEntityExtractionResult creates a new entity extraction result with generated ID and timestamp
func NewEntityExtractionResult(documentID string, docType DocumentType) *EntityExtractionResult {
	return &EntityExtractionResult{
		ID:                 uuid.New().String(),
		DocumentID:         documentID,
		DocumentType:       docType,
		Entities:           make([]*ExtractedEntity, 0),
		TotalEntities:      0,
		QualityMetrics:     &QualityMetrics{},
		ExtractionMetadata: make(map[string]any),
		ExtractorVersion:   "1.0.0",
		PromptVersion:      "1.0.0",
		CreatedAt:          time.Now(),
		Method:             ExtractorMethodLLM,
	}
}

// NewExtractedEntity creates a new extracted entity with generated ID and timestamp
func NewExtractedEntity(entityType EntityType, text string, confidence float64, start, end int) *ExtractedEntity {
	return &ExtractedEntity{
		ID:              uuid.New().String(),
		Type:            entityType,
		Text:            text,
		NormalizedText:  text, // Can be enhanced later
		Confidence:      confidence,
		StartOffset:     start,
		EndOffset:       end,
		EntityMetadata:  &EntityExtractionMeta{},
		ValidationFlags: &ValidationFlags{IsValid: true},
		ExtractedAt:     time.Now(),
	}
}

// Validate checks if the entity extraction result is valid
func (eer *EntityExtractionResult) Validate() []string {
	var errors []string

	if eer.DocumentID == "" {
		errors = append(errors, "document_id is required")
	}

	if eer.DocumentType == "" {
		errors = append(errors, "document_type is required")
	}

	if eer.TotalEntities != len(eer.Entities) {
		errors = append(errors, "total_entities does not match entities array length")
	}

	// Validate individual entities
	for i, entity := range eer.Entities {
		if entityErrors := entity.Validate(); len(entityErrors) > 0 {
			for _, err := range entityErrors {
				errors = append(errors, fmt.Sprintf("entity[%d]: %s", i, err))
			}
		}
	}

	return errors
}

// Validate checks if the extracted entity is valid
func (ee *ExtractedEntity) Validate() []string {
	var errors []string

	if ee.Type == "" {
		errors = append(errors, "entity type is required")
	}

	if ee.Text == "" {
		errors = append(errors, "entity text is required")
	}

	if ee.Confidence < 0.0 || ee.Confidence > 1.0 {
		errors = append(errors, "confidence must be between 0.0 and 1.0")
	}

	if ee.StartOffset < 0 {
		errors = append(errors, "start_offset must be non-negative")
	}

	if ee.EndOffset <= ee.StartOffset {
		errors = append(errors, "end_offset must be greater than start_offset")
	}

	return errors
}

// GetEntityTypesList returns a list of all supported entity types
func GetEntityTypesList() []EntityType {
	return []EntityType{
		EntityTypePerson,
		EntityTypeOrganization,
		EntityTypeLocation,
		EntityTypeDate,
		EntityTypeEmail,
		EntityTypeConcept,
		EntityTypeMetric,
		EntityTypeTechnology,
		EntityTypeURL,
		EntityTypeCitation,
	}
}

// IsValidEntityType checks if the given entity type is supported
func IsValidEntityType(entityType EntityType) bool {
	validTypes := map[EntityType]bool{
		EntityTypePerson:       true,
		EntityTypeOrganization: true,
		EntityTypeLocation:     true,
		EntityTypeDate:         true,
		EntityTypeEmail:        true,
		EntityTypeConcept:      true,
		EntityTypeMetric:       true,
		EntityTypeTechnology:   true,
		EntityTypeURL:          true,
		EntityTypeCitation:     true,
	}
	return validTypes[entityType]
}
