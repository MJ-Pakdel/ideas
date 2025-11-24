package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestNewDocument tests the creation of new documents
func TestNewDocument(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		docName     string
		content     string
		contentType string
	}{
		{
			name:        "basic document",
			path:        "/test/path/document.txt",
			docName:     "test-document.txt",
			content:     "This is test content for the document",
			contentType: "text/plain",
		},
		{
			name:        "empty content",
			path:        "/test/empty.txt",
			docName:     "empty.txt",
			content:     "",
			contentType: "text/plain",
		},
		{
			name:        "unicode content",
			path:        "/test/unicode.txt",
			docName:     "unicode.txt",
			content:     "Hello 世界! Testing üñíçødé content",
			contentType: "text/plain",
		},
		{
			name:        "json content",
			path:        "/data/config.json",
			docName:     "config.json",
			content:     `{"key": "value", "number": 42}`,
			contentType: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			doc := NewDocument(tt.path, tt.docName, tt.content, tt.contentType)

			// Verify basic fields
			if doc.ID == "" {
				t.Error("ID should be generated")
			}
			if doc.Path != tt.path {
				t.Errorf("Path = %v, want %v", doc.Path, tt.path)
			}
			if doc.Name != tt.docName {
				t.Errorf("Name = %v, want %v", doc.Name, tt.docName)
			}
			if doc.Content != tt.content {
				t.Errorf("Content = %v, want %v", doc.Content, tt.content)
			}
			if doc.ContentType != tt.contentType {
				t.Errorf("ContentType = %v, want %v", doc.ContentType, tt.contentType)
			}
			if doc.Size != int64(len(tt.content)) {
				t.Errorf("Size = %v, want %v", doc.Size, int64(len(tt.content)))
			}
			if doc.Metadata == nil {
				t.Error("Metadata should be initialized")
			}
			if doc.CreatedAt.IsZero() {
				t.Error("CreatedAt should be set")
			}
			if doc.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should be set")
			}
			if doc.CreatedAt != doc.UpdatedAt {
				t.Error("CreatedAt and UpdatedAt should be equal for new document")
			}

			// Verify ID is valid UUID
			if _, err := uuid.Parse(doc.ID); err != nil {
				t.Errorf("ID should be valid UUID: %v", err)
			}

			// Verify timestamps are recent
			if time.Since(doc.CreatedAt) > time.Second {
				t.Error("CreatedAt should be recent")
			}
			if doc.CreatedAt.Before(now.Add(-time.Second)) {
				t.Error("CreatedAt should be after test start")
			}
		})
	}
}

// TestDocumentJSONSerialization tests JSON marshaling/unmarshaling
func TestDocumentJSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		document *Document
	}{
		{
			name:     "basic document",
			document: NewDocument("/test/path", "test.txt", "content", "text/plain"),
		},
		{
			name: "document with metadata",
			document: func() *Document {
				doc := NewDocument("/test/path", "test.txt", "content", "text/plain")
				doc.Metadata["test_key"] = "test_value"
				doc.Metadata["number"] = 42
				return doc
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			jsonData, err := json.Marshal(tt.document)
			if err != nil {
				t.Fatalf("Failed to marshal to JSON: %v", err)
			}

			// Unmarshal from JSON
			var unmarshaled Document
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal from JSON: %v", err)
			}

			// Verify data integrity
			if unmarshaled.ID != tt.document.ID {
				t.Errorf("ID = %v, want %v", unmarshaled.ID, tt.document.ID)
			}
			if unmarshaled.Path != tt.document.Path {
				t.Errorf("Path = %v, want %v", unmarshaled.Path, tt.document.Path)
			}
			if unmarshaled.Name != tt.document.Name {
				t.Errorf("Name = %v, want %v", unmarshaled.Name, tt.document.Name)
			}
			if unmarshaled.Content != tt.document.Content {
				t.Errorf("Content = %v, want %v", unmarshaled.Content, tt.document.Content)
			}
		})
	}
}

// TestEntityCreation tests entity creation and validation
func TestEntityCreation(t *testing.T) {
	tests := []struct {
		name   string
		entity *Entity
	}{
		{
			name: "person entity",
			entity: &Entity{
				ID:          uuid.New().String(),
				Type:        EntityTypePerson,
				Text:        "John Doe",
				Confidence:  0.95,
				StartOffset: 100,
				EndOffset:   108,
				Context: &EntityContext{
					SurroundingText: "Hello John Doe, how are you?",
					SentenceBefore:  "Hello",
					SentenceAfter:   "how are you?",
					Section:         "Introduction",
				},
				Metadata:    map[string]any{"extracted_by": "test"},
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      ExtractorMethodLLM,
			},
		},
		{
			name: "organization entity",
			entity: &Entity{
				ID:          uuid.New().String(),
				Type:        EntityTypeOrganization,
				Text:        "ACME Corp",
				Confidence:  0.88,
				StartOffset: 50,
				EndOffset:   59,
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      ExtractorMethodRegex,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := tt.entity

			// Verify entity fields
			if entity.ID == "" {
				t.Error("ID should not be empty")
			}
			if entity.Text == "" {
				t.Error("Text should not be empty")
			}
			if entity.Confidence < 0 || entity.Confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", entity.Confidence)
			}
			if entity.StartOffset > entity.EndOffset {
				t.Errorf("StartOffset (%d) should not be greater than EndOffset (%d)", entity.StartOffset, entity.EndOffset)
			}
			if entity.DocumentID == "" {
				t.Error("DocumentID should not be empty")
			}
			if entity.ExtractedAt.IsZero() {
				t.Error("ExtractedAt should be set")
			}

			// Test JSON serialization
			jsonData, err := json.Marshal(entity)
			if err != nil {
				t.Fatalf("Failed to marshal entity: %v", err)
			}

			var unmarshaled Entity
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal entity: %v", err)
			}

			if unmarshaled.ID != entity.ID {
				t.Errorf("ID = %v, want %v", unmarshaled.ID, entity.ID)
			}
			if unmarshaled.Type != entity.Type {
				t.Errorf("Type = %v, want %v", unmarshaled.Type, entity.Type)
			}
			if unmarshaled.Text != entity.Text {
				t.Errorf("Text = %v, want %v", unmarshaled.Text, entity.Text)
			}
		})
	}
}

// TestCitationCreation tests citation creation and validation
func TestCitationCreation(t *testing.T) {
	title := "Test Paper Title"
	year := 2023
	journal := "Test Journal"
	doi := "10.1000/test.doi"

	tests := []struct {
		name     string
		citation *Citation
	}{
		{
			name: "APA citation",
			citation: &Citation{
				ID:          uuid.New().String(),
				Text:        "Smith, J. (2023). Test Paper Title. Test Journal, 1(1), 1-10.",
				Authors:     []string{"Smith, J."},
				Title:       &title,
				Year:        &year,
				Journal:     &journal,
				DOI:         &doi,
				Format:      CitationFormatAPA,
				Confidence:  0.89,
				StartOffset: 200,
				EndOffset:   280,
				Context: &CitationContext{
					SurroundingText: "As shown in Smith, J. (2023). Test Paper Title.",
					Section:         "Related Work",
					Purpose:         "Supporting evidence",
				},
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      ExtractorMethodRegex,
			},
		},
		{
			name: "basic citation",
			citation: &Citation{
				ID:          uuid.New().String(),
				Text:        "Doe et al. (2022)",
				Authors:     []string{"Doe, J.", "Smith, A."},
				Format:      CitationFormatMLA,
				Confidence:  0.75,
				StartOffset: 100,
				EndOffset:   118,
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      ExtractorMethodLLM,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			citation := tt.citation

			// Verify citation fields
			if citation.ID == "" {
				t.Error("ID should not be empty")
			}
			if citation.Text == "" {
				t.Error("Text should not be empty")
			}
			if len(citation.Authors) == 0 {
				t.Error("Authors should not be empty")
			}
			if citation.Confidence < 0 || citation.Confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", citation.Confidence)
			}
			if citation.DocumentID == "" {
				t.Error("DocumentID should not be empty")
			}

			// Test JSON serialization
			jsonData, err := json.Marshal(citation)
			if err != nil {
				t.Fatalf("Failed to marshal citation: %v", err)
			}

			var unmarshaled Citation
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal citation: %v", err)
			}

			if unmarshaled.ID != citation.ID {
				t.Errorf("ID = %v, want %v", unmarshaled.ID, citation.ID)
			}
			if len(unmarshaled.Authors) != len(citation.Authors) {
				t.Errorf("Authors length = %v, want %v", len(unmarshaled.Authors), len(citation.Authors))
			}
		})
	}
}

// TestTopicCreation tests topic creation and validation
func TestTopicCreation(t *testing.T) {
	tests := []struct {
		name  string
		topic *Topic
	}{
		{
			name: "machine learning topic",
			topic: &Topic{
				ID:          uuid.New().String(),
				Name:        "Machine Learning",
				Keywords:    []string{"neural networks", "deep learning", "AI"},
				Confidence:  0.92,
				Weight:      0.75,
				Description: "Topic related to machine learning and artificial intelligence",
				Metadata:    map[string]any{"source": "topic_modeling"},
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      "lda",
			},
		},
		{
			name: "simple topic",
			topic: &Topic{
				ID:          uuid.New().String(),
				Name:        "Data Science",
				Keywords:    []string{"statistics", "analytics"},
				Confidence:  0.85,
				Weight:      0.60,
				DocumentID:  uuid.New().String(),
				ExtractedAt: time.Now(),
				Method:      "keyword_extraction",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := tt.topic

			// Verify topic fields
			if topic.ID == "" {
				t.Error("ID should not be empty")
			}
			if topic.Name == "" {
				t.Error("Name should not be empty")
			}
			if len(topic.Keywords) == 0 {
				t.Error("Keywords should not be empty")
			}
			if topic.Confidence < 0 || topic.Confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", topic.Confidence)
			}
			if topic.Weight < 0 || topic.Weight > 1 {
				t.Errorf("Weight should be between 0 and 1, got %f", topic.Weight)
			}
			if topic.DocumentID == "" {
				t.Error("DocumentID should not be empty")
			}

			// Test JSON serialization
			jsonData, err := json.Marshal(topic)
			if err != nil {
				t.Fatalf("Failed to marshal topic: %v", err)
			}

			var unmarshaled Topic
			if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal topic: %v", err)
			}

			if unmarshaled.ID != topic.ID {
				t.Errorf("ID = %v, want %v", unmarshaled.ID, topic.ID)
			}
			if unmarshaled.Name != topic.Name {
				t.Errorf("Name = %v, want %v", unmarshaled.Name, topic.Name)
			}
		})
	}
}

// TestConstants tests type constants
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "entity types",
			testFunc: func(t *testing.T) {
				entityTypes := []EntityType{
					EntityTypePerson,
					EntityTypeOrganization,
					EntityTypeLocation,
					EntityTypeDate,
					EntityTypeEmail,
					EntityTypeConcept,
					EntityTypeMetric,
					EntityTypeTechnology,
				}

				for _, entityType := range entityTypes {
					if string(entityType) == "" {
						t.Errorf("Entity type %v should not be empty", entityType)
					}
				}

				// Test specific values
				if string(EntityTypePerson) != "PERSON" {
					t.Errorf("EntityTypePerson = %v, want PERSON", string(EntityTypePerson))
				}
				if string(EntityTypeOrganization) != "ORGANIZATION" {
					t.Errorf("EntityTypeOrganization = %v, want ORGANIZATION", string(EntityTypeOrganization))
				}
			},
		},
		{
			name: "citation formats",
			testFunc: func(t *testing.T) {
				formats := []CitationFormat{
					CitationFormatAPA,
					CitationFormatMLA,
					CitationFormatChicago,
					CitationFormatIEEE,
					CitationFormatAuto,
				}

				for _, format := range formats {
					if string(format) == "" {
						t.Errorf("Citation format %v should not be empty", format)
					}
				}

				// Test specific values
				if string(CitationFormatAPA) != "apa" {
					t.Errorf("CitationFormatAPA = %v, want apa", string(CitationFormatAPA))
				}
			},
		},
		{
			name: "extractor methods",
			testFunc: func(t *testing.T) {
				methods := []ExtractorMethod{
					ExtractorMethodLLM, // Only LLM for entity extraction
				}

				for _, method := range methods {
					if string(method) == "" {
						t.Errorf("Extractor method %v should not be empty", method)
					}
				}

				// Test specific values
				if string(ExtractorMethodRegex) != "regex" {
					t.Errorf("ExtractorMethodRegex = %v, want regex", string(ExtractorMethodRegex))
				}
			},
		},
		{
			name: "document types",
			testFunc: func(t *testing.T) {
				docTypes := []DocumentType{
					DocumentTypeResearch,
					DocumentTypeBook,
					DocumentTypeArticle,
					DocumentTypeReport,
					DocumentTypePresentation,
					DocumentTypeBusiness,
					DocumentTypePersonal,
					DocumentTypeLegal,
					DocumentTypeOther,
				}

				for _, docType := range docTypes {
					if string(docType) == "" {
						t.Errorf("Document type %v should not be empty", docType)
					}
				}

				// Test specific values
				if string(DocumentTypeResearch) != "research_paper" {
					t.Errorf("DocumentTypeResearch = %v, want research_paper", string(DocumentTypeResearch))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestNewAnalysisResult tests the creation of new analysis results
func TestNewAnalysisResult(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name       string
		documentID string
		wantFields map[string]any
	}{
		{
			name:       "valid document ID",
			documentID: "test-doc-123",
			wantFields: map[string]any{
				"documentID": "test-doc-123",
				"version":    "1.0",
			},
		},
		{
			name:       "empty document ID",
			documentID: "",
			wantFields: map[string]any{
				"documentID": "",
				"version":    "1.0",
			},
		},
		{
			name:       "UUID document ID",
			documentID: "550e8400-e29b-41d4-a716-446655440000",
			wantFields: map[string]any{
				"documentID": "550e8400-e29b-41d4-a716-446655440000",
				"version":    "1.0",
			},
		},
		{
			name:       "long document ID",
			documentID: "very-long-document-identifier-with-lots-of-characters",
			wantFields: map[string]any{
				"documentID": "very-long-document-identifier-with-lots-of-characters",
				"version":    "1.0",
			},
		},
	}

	testStart := time.Now()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAnalysisResult(tt.documentID)

			// Test required fields
			if result == nil {
				t.Fatal("NewAnalysisResult() returned nil")
			}

			// Test ID generation
			if result.ID == "" {
				t.Error("NewAnalysisResult() ID should not be empty")
			}

			// Validate ID is a valid UUID
			if _, err := uuid.Parse(result.ID); err != nil {
				t.Errorf("NewAnalysisResult() ID is not a valid UUID: %v", err)
			}

			// Test DocumentID
			if result.DocumentID != tt.wantFields["documentID"] {
				t.Errorf("NewAnalysisResult() DocumentID = %v, want %v", result.DocumentID, tt.wantFields["documentID"])
			}

			// Test Version
			if result.Version != tt.wantFields["version"] {
				t.Errorf("NewAnalysisResult() Version = %v, want %v", result.Version, tt.wantFields["version"])
			}

			// Test initialized slices are not nil
			if result.Entities == nil {
				t.Error("NewAnalysisResult() Entities should be initialized, not nil")
			}
			if result.Citations == nil {
				t.Error("NewAnalysisResult() Citations should be initialized, not nil")
			}
			if result.Topics == nil {
				t.Error("NewAnalysisResult() Topics should be initialized, not nil")
			}
			if result.Relationships == nil {
				t.Error("NewAnalysisResult() Relationships should be initialized, not nil")
			}
			if result.KeyPoints == nil {
				t.Error("NewAnalysisResult() KeyPoints should be initialized, not nil")
			}
			if result.Metadata == nil {
				t.Error("NewAnalysisResult() Metadata should be initialized, not nil")
			}

			// Test initialized slices are empty
			if len(result.Entities) != 0 {
				t.Errorf("NewAnalysisResult() Entities length = %d, want 0", len(result.Entities))
			}
			if len(result.Citations) != 0 {
				t.Errorf("NewAnalysisResult() Citations length = %d, want 0", len(result.Citations))
			}
			if len(result.Topics) != 0 {
				t.Errorf("NewAnalysisResult() Topics length = %d, want 0", len(result.Topics))
			}
			if len(result.Relationships) != 0 {
				t.Errorf("NewAnalysisResult() Relationships length = %d, want 0", len(result.Relationships))
			}
			if len(result.KeyPoints) != 0 {
				t.Errorf("NewAnalysisResult() KeyPoints length = %d, want 0", len(result.KeyPoints))
			}
			if len(result.Metadata) != 0 {
				t.Errorf("NewAnalysisResult() Metadata length = %d, want 0", len(result.Metadata))
			}

			// Test timestamp is reasonable
			if result.AnalyzedAt.Before(testStart) {
				t.Error("NewAnalysisResult() AnalyzedAt should be after test start")
			}
			if result.AnalyzedAt.After(time.Now()) {
				t.Error("NewAnalysisResult() AnalyzedAt should not be in the future")
			}
		})
	}
}

// TestNewError tests the creation of new error information
func TestNewError(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name    string
		code    string
		message string
		details string
	}{
		{
			name:    "basic error",
			code:    "TEST_ERROR",
			message: "Test error message",
			details: "Detailed test error information",
		},
		{
			name:    "empty details",
			code:    "EMPTY_DETAILS",
			message: "Error with no details",
			details: "",
		},
		{
			name:    "empty message",
			code:    "EMPTY_MESSAGE",
			message: "",
			details: "Error with empty message",
		},
		{
			name:    "empty code",
			code:    "",
			message: "Error with empty code",
			details: "This error has no code",
		},
		{
			name:    "all empty",
			code:    "",
			message: "",
			details: "",
		},
		{
			name:    "long message",
			code:    "LONG_MESSAGE",
			message: "This is a very long error message that contains lots of information about what went wrong in the system",
			details: "Even more detailed information about the error condition",
		},
		{
			name:    "special characters",
			code:    "SPECIAL_CHARS",
			message: "Error with üñíçødé characters: 世界",
			details: "Details with symbols: !@#$%^&*()[]{}",
		},
		{
			name:    "JSON-like message",
			code:    "JSON_ERROR",
			message: `{"error": "parsing failed", "line": 42}`,
			details: "JSON parsing error details",
		},
	}

	testStart := time.Now()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewError(tt.code, tt.message, tt.details)

			// Test required fields
			if result == nil {
				t.Fatal("NewError() returned nil")
			}

			// Test Code
			if result.Code != tt.code {
				t.Errorf("NewError() Code = %v, want %v", result.Code, tt.code)
			}

			// Test Message
			if result.Message != tt.message {
				t.Errorf("NewError() Message = %v, want %v", result.Message, tt.message)
			}

			// Test Details
			if result.Details != tt.details {
				t.Errorf("NewError() Details = %v, want %v", result.Details, tt.details)
			}

			// Test Context is initialized
			if result.Context == nil {
				t.Error("NewError() Context should be initialized, not nil")
			}

			// Test Context is empty
			if len(result.Context) != 0 {
				t.Errorf("NewError() Context length = %d, want 0", len(result.Context))
			}

			// Test timestamp is reasonable
			if result.Timestamp.Before(testStart) {
				t.Error("NewError() Timestamp should be after test start")
			}
			if result.Timestamp.After(time.Now()) {
				t.Error("NewError() Timestamp should not be in the future")
			}
		})
	}
}

// TestParseUUID tests UUID parsing functionality
func TestParseUUID(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid UUID v4",
			input:   "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID v1",
			input:   "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			wantErr: false,
		},
		{
			name:    "valid UUID v5",
			input:   "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
			wantErr: false,
		},
		{
			name:    "valid UUID uppercase",
			input:   "550E8400-E29B-41D4-A716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid UUID mixed case",
			input:   "550e8400-E29B-41d4-A716-446655440000",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "empty string should be invalid UUID",
		},
		{
			name:    "invalid format - too short",
			input:   "550e8400-e29b-41d4-a716",
			wantErr: true,
			errMsg:  "short UUID should be invalid",
		},
		{
			name:    "invalid format - too long",
			input:   "550e8400-e29b-41d4-a716-446655440000-extra",
			wantErr: true,
			errMsg:  "long UUID should be invalid",
		},
		{
			name:    "invalid format - missing hyphens (actually valid in Go)",
			input:   "550e8400e29b41d4a716446655440000",
			wantErr: false, // Go UUID parser accepts this format
		},
		{
			name:    "invalid format - wrong hyphen positions",
			input:   "550e8-400e29b-41d4a-716446655440000",
			wantErr: true,
			errMsg:  "UUID with wrong hyphen positions should be invalid",
		},
		{
			name:    "invalid characters - non-hex",
			input:   "550g8400-e29b-41d4-a716-446655440000",
			wantErr: true,
			errMsg:  "UUID with non-hex characters should be invalid",
		},
		{
			name:    "invalid characters - symbols",
			input:   "550e8400-e29b-41d4-a716-44665544000@",
			wantErr: true,
			errMsg:  "UUID with symbols should be invalid",
		},
		{
			name:    "random string",
			input:   "not-a-uuid-at-all",
			wantErr: true,
			errMsg:  "random string should be invalid UUID",
		},
		{
			name:    "32 hex digits (valid in Go)",
			input:   "12345678901234567890123456789012",
			wantErr: false, // Go UUID parser accepts 32 hex digits
		},
		{
			name:    "nil UUID string",
			input:   "00000000-0000-0000-0000-000000000000",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseUUID(tt.input)

			// Test error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUUID() error = %v, wantErr %v (%s)", err, tt.wantErr, tt.errMsg)
				return
			}

			// If no error expected, validate the result
			if !tt.wantErr {
				// Test that result is not nil UUID for non-nil inputs
				if result == uuid.Nil && tt.input != "00000000-0000-0000-0000-000000000000" {
					t.Error("ParseUUID() returned nil UUID for valid input")
				}

				// For valid UUIDs, just check that we got a valid UUID back
				// The parser normalizes the format, so we don't need to check exact string match
				if result.String() == "" {
					t.Error("ParseUUID() returned empty string for valid UUID")
				}
			}

			// If error expected, validate error is not nil
			if tt.wantErr && err == nil {
				t.Errorf("ParseUUID() expected error but got none (%s)", tt.errMsg)
			}
		})
	}
}
