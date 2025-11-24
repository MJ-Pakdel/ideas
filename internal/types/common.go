// Package types provides the unified type system for the IDAES intelligence layer.package types

// This package defines all core data structures used throughout the system,
// ensuring consistency and interoperability between components.
package types

import (
	"time"

	"github.com/google/uuid"
)

// ExtractorMethod defines the available extraction methods
type ExtractorMethod string

const (
	ExtractorMethodRegex ExtractorMethod = "regex" // Used for fallback implementations
	ExtractorMethodLLM   ExtractorMethod = "llm"   // Primary method for IDAES
)

// EntityType defines the types of entities that can be extracted
type EntityType string

const (
	EntityTypePerson       EntityType = "PERSON"
	EntityTypeOrganization EntityType = "ORGANIZATION"
	EntityTypeLocation     EntityType = "LOCATION"
	EntityTypeDate         EntityType = "DATE"
	EntityTypeEmail        EntityType = "EMAIL"
	EntityTypeConcept      EntityType = "CONCEPT"
	EntityTypeMetric       EntityType = "METRIC"
	EntityTypeTechnology   EntityType = "TECHNOLOGY"
	EntityTypeURL          EntityType = "URL"
	EntityTypeCitation     EntityType = "CITATION"
)

// CitationFormat defines supported citation formats
type CitationFormat string

const (
	CitationFormatAPA     CitationFormat = "apa"
	CitationFormatMLA     CitationFormat = "mla"
	CitationFormatChicago CitationFormat = "chicago"
	CitationFormatIEEE    CitationFormat = "ieee"
	CitationFormatAuto    CitationFormat = "auto"
)

// DocumentType defines the classification of documents
type DocumentType string

const (
	DocumentTypeResearch     DocumentType = "research_paper"
	DocumentTypeBook         DocumentType = "book"
	DocumentTypeArticle      DocumentType = "article"
	DocumentTypeReport       DocumentType = "report"
	DocumentTypePresentation DocumentType = "presentation"
	DocumentTypeBusiness     DocumentType = "business_document"
	DocumentTypePersonal     DocumentType = "personal_document"
	DocumentTypeLegal        DocumentType = "legal_document"
	DocumentTypeOther        DocumentType = "other"
)

// Document represents a document for analysis
type Document struct {
	ID          string         `json:"id" yaml:"id"`
	Path        string         `json:"path" yaml:"path"`
	Name        string         `json:"name" yaml:"name"`
	Content     string         `json:"content" yaml:"content"`
	ContentType string         `json:"content_type" yaml:"content_type"`
	Size        int64          `json:"size" yaml:"size"`
	Hash        string         `json:"hash" yaml:"hash"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" yaml:"updated_at"`
}

// NewDocument creates a new document with generated ID and timestamps
func NewDocument(path, name, content, contentType string) *Document {
	now := time.Now()
	return &Document{
		ID:          uuid.New().String(),
		Path:        path,
		Name:        name,
		Content:     content,
		ContentType: contentType,
		Size:        int64(len(content)),
		Metadata:    make(map[string]any),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Entity represents an extracted named entity
type Entity struct {
	ID          string          `json:"id" yaml:"id"`
	Type        EntityType      `json:"type" yaml:"type"`
	Text        string          `json:"text" yaml:"text"`
	Confidence  float64         `json:"confidence" yaml:"confidence"`
	StartOffset int             `json:"start_offset" yaml:"start_offset"`
	EndOffset   int             `json:"end_offset" yaml:"end_offset"`
	Context     *EntityContext  `json:"context,omitempty" yaml:"context,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	DocumentID  string          `json:"document_id" yaml:"document_id"`
	ExtractedAt time.Time       `json:"extracted_at" yaml:"extracted_at"`
	Method      ExtractorMethod `json:"method" yaml:"method"`
}

// EntityContext provides contextual information about an entity
type EntityContext struct {
	SurroundingText string `json:"surrounding_text" yaml:"surrounding_text"`
	SentenceBefore  string `json:"sentence_before,omitempty" yaml:"sentence_before,omitempty"`
	SentenceAfter   string `json:"sentence_after,omitempty" yaml:"sentence_after,omitempty"`
	Section         string `json:"section,omitempty" yaml:"section,omitempty"`
}

// Citation represents an extracted citation
type Citation struct {
	ID          string           `json:"id" yaml:"id"`
	Text        string           `json:"text" yaml:"text"`
	Authors     []string         `json:"authors" yaml:"authors"`
	Title       *string          `json:"title,omitempty" yaml:"title,omitempty"`
	Year        *int             `json:"year,omitempty" yaml:"year,omitempty"`
	Journal     *string          `json:"journal,omitempty" yaml:"journal,omitempty"`
	Volume      *string          `json:"volume,omitempty" yaml:"volume,omitempty"`
	Issue       *string          `json:"issue,omitempty" yaml:"issue,omitempty"`
	Pages       *string          `json:"pages,omitempty" yaml:"pages,omitempty"`
	Publisher   *string          `json:"publisher,omitempty" yaml:"publisher,omitempty"`
	URL         *string          `json:"url,omitempty" yaml:"url,omitempty"`
	DOI         *string          `json:"doi,omitempty" yaml:"doi,omitempty"`
	Format      CitationFormat   `json:"format" yaml:"format"`
	Confidence  float64          `json:"confidence" yaml:"confidence"`
	StartOffset int              `json:"start_offset" yaml:"start_offset"`
	EndOffset   int              `json:"end_offset" yaml:"end_offset"`
	Context     *CitationContext `json:"context,omitempty" yaml:"context,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	DocumentID  string           `json:"document_id" yaml:"document_id"`
	ExtractedAt time.Time        `json:"extracted_at" yaml:"extracted_at"`
	Method      ExtractorMethod  `json:"method" yaml:"method"`
}

// CitationContext provides contextual information about a citation
type CitationContext struct {
	SurroundingText string `json:"surrounding_text" yaml:"surrounding_text"`
	Section         string `json:"section,omitempty" yaml:"section,omitempty"`
	Purpose         string `json:"purpose,omitempty" yaml:"purpose,omitempty"`
}

// Topic represents an extracted topic or theme
type Topic struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Keywords    []string       `json:"keywords" yaml:"keywords"`
	Confidence  float64        `json:"confidence" yaml:"confidence"`
	Weight      float64        `json:"weight" yaml:"weight"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	DocumentID  string         `json:"document_id" yaml:"document_id"`
	ExtractedAt time.Time      `json:"extracted_at" yaml:"extracted_at"`
	Method      string         `json:"method" yaml:"method"`
}

// EntityRelationship represents a relationship between entities
type EntityRelationship struct {
	ID           string         `json:"id" yaml:"id"`
	SourceEntity string         `json:"source_entity" yaml:"source_entity"`
	TargetEntity string         `json:"target_entity" yaml:"target_entity"`
	RelationType string         `json:"relation_type" yaml:"relation_type"`
	Confidence   float64        `json:"confidence" yaml:"confidence"`
	Evidence     string         `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Context      string         `json:"context,omitempty" yaml:"context,omitempty"`
	DocumentIDs  []string       `json:"document_ids" yaml:"document_ids"`
	Metadata     map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	DetectedAt   time.Time      `json:"detected_at" yaml:"detected_at"`
}

// DocumentClassification represents the classification result of a document
type DocumentClassification struct {
	ID             string             `json:"id" yaml:"id"`
	DocumentID     string             `json:"document_id" yaml:"document_id"`
	PrimaryType    DocumentType       `json:"primary_type" yaml:"primary_type"`
	SecondaryTypes []DocumentType     `json:"secondary_types,omitempty" yaml:"secondary_types,omitempty"`
	Confidence     float64            `json:"confidence" yaml:"confidence"`
	Features       map[string]float64 `json:"features,omitempty" yaml:"features,omitempty"`
	Reasoning      string             `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	Metadata       map[string]any     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	ClassifiedAt   time.Time          `json:"classified_at" yaml:"classified_at"`
	Method         string             `json:"method" yaml:"method"`
}

// AnalysisResult represents the complete analysis result for a document
type AnalysisResult struct {
	ID             string                  `json:"id" yaml:"id"`
	DocumentID     string                  `json:"document_id" yaml:"document_id"`
	Classification *DocumentClassification `json:"classification,omitempty" yaml:"classification,omitempty"`
	Entities       []*Entity               `json:"entities,omitempty" yaml:"entities,omitempty"`
	Citations      []*Citation             `json:"citations,omitempty" yaml:"citations,omitempty"`
	Topics         []*Topic                `json:"topics,omitempty" yaml:"topics,omitempty"`
	Relationships  []*EntityRelationship   `json:"relationships,omitempty" yaml:"relationships,omitempty"`
	Summary        string                  `json:"summary,omitempty" yaml:"summary,omitempty"`
	KeyPoints      []string                `json:"key_points,omitempty" yaml:"key_points,omitempty"`
	ProcessingTime time.Duration           `json:"processing_time" yaml:"processing_time"`
	Confidence     float64                 `json:"confidence" yaml:"confidence"`
	Metadata       map[string]any          `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	AnalyzedAt     time.Time               `json:"analyzed_at" yaml:"analyzed_at"`
	Version        string                  `json:"version" yaml:"version"`
}

// NewAnalysisResult creates a new analysis result with generated ID and timestamp
func NewAnalysisResult(documentID string) *AnalysisResult {
	return &AnalysisResult{
		ID:            uuid.New().String(),
		DocumentID:    documentID,
		Entities:      make([]*Entity, 0),
		Citations:     make([]*Citation, 0),
		Topics:        make([]*Topic, 0),
		Relationships: make([]*EntityRelationship, 0),
		KeyPoints:     make([]string, 0),
		Metadata:      make(map[string]any),
		AnalyzedAt:    time.Now(),
		Version:       "1.0",
	}
}

// ProcessingMetrics tracks performance and quality metrics
type ProcessingMetrics struct {
	TotalDocuments        int64         `json:"total_documents" yaml:"total_documents"`
	TotalEntities         int64         `json:"total_entities" yaml:"total_entities"`
	TotalCitations        int64         `json:"total_citations" yaml:"total_citations"`
	TotalTopics           int64         `json:"total_topics" yaml:"total_topics"`
	AverageProcessingTime time.Duration `json:"average_processing_time" yaml:"average_processing_time"`
	AverageConfidence     float64       `json:"average_confidence" yaml:"average_confidence"`
	ErrorRate             float64       `json:"error_rate" yaml:"error_rate"`
	CacheHitRate          float64       `json:"cache_hit_rate" yaml:"cache_hit_rate"`
	LastUpdated           time.Time     `json:"last_updated" yaml:"last_updated"`
}

// Collection represents a ChromaDB collection with metadata
type Collection struct {
	ID          string         `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Count       int            `json:"count" yaml:"count"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" yaml:"updated_at"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SearchResult represents a search result from the vector database
type SearchResult struct {
	ID       string         `json:"id" yaml:"id"`
	Document *Document      `json:"document,omitempty" yaml:"document,omitempty"`
	Score    float64        `json:"score" yaml:"score"`
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ErrorInfo represents detailed error information
type ErrorInfo struct {
	Code      string         `json:"code" yaml:"code"`
	Message   string         `json:"message" yaml:"message"`
	Details   string         `json:"details,omitempty" yaml:"details,omitempty"`
	Timestamp time.Time      `json:"timestamp" yaml:"timestamp"`
	Context   map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}

// NewError creates a new ErrorInfo with timestamp
func NewError(code, message, details string) *ErrorInfo {
	return &ErrorInfo{
		Code:      code,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
		Context:   make(map[string]any),
	}
}

// ParseUUID parses a string UUID and returns an error if invalid
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
