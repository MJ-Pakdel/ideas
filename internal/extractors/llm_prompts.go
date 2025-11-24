package extractors

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// initializePromptTemplates creates the default prompt templates for different document types
func initializePromptTemplates() *PromptTemplates {
	return &PromptTemplates{
		ClassificationPrompt: getDocumentClassificationPrompt(),
		UnifiedPrompts: map[types.DocumentType]string{
			types.DocumentTypeResearch:     getResearchPaperUnifiedPrompt(),
			types.DocumentTypeBusiness:     getBusinessDocumentUnifiedPrompt(),
			types.DocumentTypePersonal:     getPersonalDocumentUnifiedPrompt(),
			types.DocumentTypeArticle:      getArticleUnifiedPrompt(),
			types.DocumentTypeReport:       getReportUnifiedPrompt(),
			types.DocumentTypeBook:         getBookUnifiedPrompt(),
			types.DocumentTypePresentation: getPresentationUnifiedPrompt(),
			types.DocumentTypeLegal:        getLegalDocumentUnifiedPrompt(),
			types.DocumentTypeOther:        getGenericUnifiedPrompt(),
		},
	}
}

// buildClassificationPrompt creates a prompt for document classification
func (e *LLMUnifiedExtractor) buildClassificationPrompt(document *types.Document) string {
	// Truncate content for classification to avoid token limits
	content := document.Content
	if len(content) > 2000 {
		content = content[:2000] + "..."
	}

	return fmt.Sprintf(e.prompts.ClassificationPrompt,
		document.Name,
		content)
}

// buildUnifiedAnalysisPrompt creates a comprehensive analysis prompt based on document type
func (e *LLMUnifiedExtractor) buildUnifiedAnalysisPrompt(document *types.Document, docType types.DocumentType, config *interfaces.UnifiedExtractionConfig) string {
	template, exists := e.prompts.UnifiedPrompts[docType]
	if !exists {
		template = e.prompts.UnifiedPrompts[types.DocumentTypeOther]
	}

	// Build entity types filter
	entityTypesFilter := ""
	if len(config.EntityTypes) > 0 {
		entityTypesFilter = fmt.Sprintf("Focus on these entity types: %v", config.EntityTypes)
	}

	// Build citation formats filter
	citationFormatsFilter := ""
	if len(config.CitationFormats) > 0 {
		citationFormatsFilter = fmt.Sprintf("Use these citation formats: %v", config.CitationFormats)
	}

	return fmt.Sprintf(template,
		document.Name,
		config.MaxEntities,
		config.MaxCitations,
		config.MaxTopics,
		config.MinConfidence,
		entityTypesFilter,
		citationFormatsFilter,
		config.AnalysisDepth,
		document.Content)
}

// parseClassificationResponse parses the LLM classification response
func (e *LLMUnifiedExtractor) parseClassificationResponse(response string) (*ClassificationResponse, error) {
	// Try to extract JSON from the response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in classification response")
	}

	var result ClassificationResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification JSON: %w", err)
	}

	return &result, nil
}

// parseUnifiedResponse parses the comprehensive LLM analysis response
func (e *LLMUnifiedExtractor) parseUnifiedResponse(response string) (*LLMAnalysisResponse, error) {
	// Try to extract JSON from the response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in unified analysis response")
	}

	var result LLMAnalysisResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Try to heal the JSON response for common issues
		healedJSON, healErr := healJSONResponse(jsonStr)
		if healErr != nil {
			return nil, fmt.Errorf("failed to parse unified analysis JSON: %w (original: %s)", err, jsonStr[:min(200, len(jsonStr))])
		}

		// Try parsing the healed JSON
		if err := json.Unmarshal([]byte(healedJSON), &result); err != nil {
			return nil, fmt.Errorf("failed to parse healed unified analysis JSON: %w", err)
		}
	}

	return &result, nil
}

// convertToUnifiedResult converts LLM response to UnifiedExtractionResult
func (e *LLMUnifiedExtractor) convertToUnifiedResult(documentID string, response *LLMAnalysisResponse, docType types.DocumentType, config *interfaces.UnifiedExtractionConfig) *interfaces.UnifiedExtractionResult {
	now := time.Now()

	result := &interfaces.UnifiedExtractionResult{
		Summary: response.Summary,
	}

	// Convert classification
	if response.DocumentClassification != nil {
		result.Classification = &types.DocumentClassification{
			ID:             uuid.New().String(),
			DocumentID:     documentID,
			PrimaryType:    convertStringToDocumentType(response.DocumentClassification.PrimaryType),
			SecondaryTypes: convertStringsToDocumentTypes(response.DocumentClassification.SecondaryTypes),
			Confidence:     response.DocumentClassification.Confidence,
			Features:       response.DocumentClassification.Features,
			Reasoning:      response.DocumentClassification.Reasoning,
			ClassifiedAt:   now,
			Method:         string(types.ExtractorMethodLLM),
		}
	}

	// Convert entities
	result.Entities = make([]*types.Entity, 0, len(response.Entities))
	for _, entity := range response.Entities {
		if entity.Confidence >= config.MinConfidence {
			result.Entities = append(result.Entities, &types.Entity{
				ID:          uuid.New().String(),
				Type:        convertStringToEntityType(entity.Type),
				Text:        entity.Text,
				Confidence:  entity.Confidence,
				StartOffset: entity.StartOffset,
				EndOffset:   entity.EndOffset,
				Context: &types.EntityContext{
					SurroundingText: entity.Context,
				},
				Metadata:    entity.Metadata,
				DocumentID:  documentID,
				ExtractedAt: now,
				Method:      types.ExtractorMethodLLM,
			})
		}
	}

	// Convert citations
	result.Citations = make([]*types.Citation, 0, len(response.Citations))
	for _, citation := range response.Citations {
		if citation.Confidence >= config.MinConfidence {
			result.Citations = append(result.Citations, &types.Citation{
				ID:          uuid.New().String(),
				Text:        citation.Text,
				Authors:     citation.Authors,
				Title:       stringPtr(citation.Title),
				Year:        intPtr(citation.Year),
				Journal:     stringPtr(citation.Journal),
				Volume:      stringPtr(citation.Volume),
				Issue:       stringPtr(citation.Issue),
				Pages:       stringPtr(citation.Pages),
				Publisher:   stringPtr(citation.Publisher),
				URL:         stringPtr(citation.URL),
				DOI:         stringPtr(citation.DOI),
				Format:      convertStringToCitationFormat(citation.Format),
				Confidence:  citation.Confidence,
				StartOffset: citation.StartOffset,
				EndOffset:   citation.EndOffset,
				Context: &types.CitationContext{
					SurroundingText: citation.Context,
				},
				Metadata:    citation.Metadata,
				DocumentID:  documentID,
				ExtractedAt: now,
				Method:      types.ExtractorMethodLLM,
			})
		}
	}

	// Convert topics
	result.Topics = make([]*types.Topic, 0, len(response.Topics))
	for _, topic := range response.Topics {
		if topic.Confidence >= config.MinConfidence {
			result.Topics = append(result.Topics, &types.Topic{
				ID:          uuid.New().String(),
				Name:        topic.Name,
				Keywords:    topic.Keywords,
				Confidence:  topic.Confidence,
				Weight:      topic.Weight,
				Description: topic.Description,
				Metadata:    topic.Metadata,
				DocumentID:  documentID,
				ExtractedAt: now,
				Method:      string(types.ExtractorMethodLLM),
			})
		}
	}

	// Convert quality metrics
	if response.QualityMetrics != nil {
		result.QualityMetrics = &interfaces.QualityMetrics{
			ConfidenceScore:   response.QualityMetrics.ConfidenceScore,
			CompletenessScore: response.QualityMetrics.CompletenessScore,
			ReliabilityScore:  response.QualityMetrics.ReliabilityScore,
		}
	}

	// Update metrics
	e.metrics.TotalEntities += int64(len(result.Entities))
	e.metrics.TotalCitations += int64(len(result.Citations))
	e.metrics.ExternalServiceCalls++

	// Calculate average confidence
	totalItems := len(result.Entities) + len(result.Citations) + len(result.Topics)
	if totalItems > 0 {
		totalConfidence := 0.0
		for _, entity := range result.Entities {
			totalConfidence += entity.Confidence
		}
		for _, citation := range result.Citations {
			totalConfidence += citation.Confidence
		}
		for _, topic := range result.Topics {
			totalConfidence += topic.Confidence
		}
		result.Confidence = totalConfidence / float64(totalItems)
		e.metrics.AverageConfidence = (e.metrics.AverageConfidence + result.Confidence) / 2.0
	}

	return result
}

// extractJSON extracts JSON content from LLM response (handling markdown code blocks, etc.)
func extractJSON(response string) string {
	// Remove markdown code block markers
	response = strings.ReplaceAll(response, "```json", "")
	response = strings.ReplaceAll(response, "```", "")

	// Trim whitespace
	response = strings.TrimSpace(response)

	// Find the first { and last } to extract JSON
	start := strings.Index(response, "{")
	if start == -1 {
		return ""
	}

	end := strings.LastIndex(response, "}")
	if end == -1 || end <= start {
		return ""
	}

	return response[start : end+1]
}

// healJSONResponse attempts to fix common JSON formatting issues from LLM responses
func healJSONResponse(jsonStr string) (string, error) {
	// First, try to fix truncated/incomplete JSON
	jsonStr = fixTruncatedJSON(jsonStr)

	// Parse to identify issues
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return "", fmt.Errorf("cannot parse as basic JSON: %w", err)
	}

	// Fix classification field if it's a string
	if classification, exists := raw["classification"]; exists {
		if classStr, ok := classification.(string); ok {
			// Convert string to object
			raw["classification"] = map[string]any{
				"primary_type": classStr,
				"confidence":   0.8, // Default confidence
				"reasoning":    "Inferred from string response",
			}
		}
	}

	// Fix topics field if it's a string or simple array
	if topics, exists := raw["topics"]; exists {
		switch topicsVal := topics.(type) {
		case string:
			// Convert single string to topic array
			raw["topics"] = []map[string]any{
				{
					"name":        topicsVal,
					"confidence":  0.8,
					"keywords":    []string{topicsVal},
					"weight":      1.0,
					"description": topicsVal,
				},
			}
		case []any:
			// Check if it's an array of strings and convert to objects
			var fixedTopics []map[string]any
			for _, topic := range topicsVal {
				if topicStr, ok := topic.(string); ok {
					fixedTopics = append(fixedTopics, map[string]any{
						"name":        topicStr,
						"confidence":  0.8,
						"keywords":    []string{topicStr},
						"weight":      1.0,
						"description": topicStr,
					})
				} else if topicMap, ok := topic.(map[string]any); ok {
					fixedTopics = append(fixedTopics, topicMap)
				}
			}
			if len(fixedTopics) > 0 {
				raw["topics"] = fixedTopics
			}
		}
	}

	// Fix citations field - handle if it's a number instead of array
	if citations, exists := raw["citations"]; exists {
		switch citationsVal := citations.(type) {
		case float64:
			// LLM returned a number instead of citations array - replace with empty array
			raw["citations"] = []any{}
		case []any:
			// Fix numeric fields within citation objects
			for _, citation := range citationsVal {
				if citationMap, ok := citation.(map[string]any); ok {
					// Convert numeric fields that should be strings
					for _, field := range []string{"issue", "volume", "pages", "year"} {
						if value, exists := citationMap[field]; exists {
							if numValue, ok := value.(float64); ok {
								citationMap[field] = fmt.Sprintf("%.0f", numValue)
							}
						}
					}
				}
			}
		}
	}

	// Re-marshal the fixed JSON
	healedBytes, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("failed to re-marshal healed JSON: %w", err)
	}

	return string(healedBytes), nil
}

// fixTruncatedJSON attempts to complete truncated JSON responses from LLM
func fixTruncatedJSON(jsonStr string) string {
	// Trim whitespace
	jsonStr = strings.TrimSpace(jsonStr)

	// Check if the response contains markdown mixed with JSON
	// Look for patterns like: {"something": "value"}\n* Item\n or similar
	if strings.Contains(jsonStr, "\n*") || strings.Contains(jsonStr, "\n#") || strings.Contains(jsonStr, "\n-") {
		// Find the end of the JSON object
		braceCount := 0
		jsonEnd := -1
		inString := false
		escaped := false

		for i, char := range jsonStr {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			if !inString {
				switch char {
				case '{':
					braceCount++
				case '}':
					braceCount--
					if braceCount == 0 {
						jsonEnd = i + 1
						break
					}
				}
			}
		}

		// If we found the end of JSON, extract only the JSON part
		if jsonEnd > 0 && jsonEnd < len(jsonStr) {
			jsonStr = jsonStr[:jsonEnd]
		}
	}

	// If JSON doesn't end properly, try to complete it
	if !strings.HasSuffix(jsonStr, "}") {
		// Count open/close braces and brackets
		braceCount := 0
		bracketCount := 0
		inString := false
		escaped := false

		for _, char := range jsonStr {
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' {
				escaped = true
				continue
			}

			if char == '"' {
				inString = !inString
				continue
			}

			if !inString {
				switch char {
				case '{':
					braceCount++
				case '}':
					braceCount--
				case '[':
					bracketCount++
				case ']':
					bracketCount--
				}
			}
		}

		// Close incomplete strings
		if inString {
			jsonStr += "\""
		}

		// Close incomplete arrays
		for bracketCount > 0 {
			jsonStr += "]"
			bracketCount--
		}

		// Close incomplete objects
		for braceCount > 0 {
			jsonStr += "}"
			braceCount--
		}
	}

	return jsonStr
} // Helper functions for type conversions
func convertStringToDocumentType(s string) types.DocumentType {
	switch strings.ToLower(s) {
	case "research_paper", "research":
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

func convertStringsToDocumentTypes(strings []string) []types.DocumentType {
	result := make([]types.DocumentType, len(strings))
	for i, s := range strings {
		result[i] = convertStringToDocumentType(s)
	}
	return result
}

func convertStringToEntityType(s string) types.EntityType {
	switch strings.ToUpper(s) {
	case "PERSON":
		return types.EntityTypePerson
	case "ORGANIZATION":
		return types.EntityTypeOrganization
	case "LOCATION":
		return types.EntityTypeLocation
	case "DATE":
		return types.EntityTypeDate
	case "EMAIL":
		return types.EntityTypeEmail
	case "CONCEPT":
		return types.EntityTypeConcept
	case "METRIC":
		return types.EntityTypeMetric
	case "TECHNOLOGY":
		return types.EntityTypeTechnology
	default:
		return types.EntityTypeConcept // Default fallback
	}
}

func convertStringToCitationFormat(s string) types.CitationFormat {
	switch strings.ToLower(s) {
	case "apa":
		return types.CitationFormatAPA
	case "mla":
		return types.CitationFormatMLA
	case "chicago":
		return types.CitationFormatChicago
	case "ieee":
		return types.CitationFormatIEEE
	default:
		return types.CitationFormatAuto
	}
}

// Helper functions for pointer conversions
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
