package extractors

import (
	"fmt"
	"strings"
	"time"

	"github.com/example/idaes/internal/types"
)

// PromptBuilder handles the construction of enhanced prompts for entity extraction
type PromptBuilder struct {
	DocumentType     types.DocumentType
	MaxEntities      int
	MinConfidence    float64
	ContextWindow    int
	ExampleMode      bool
	AnalysisDepth    types.AnalysisDepth
	EntityTypes      []types.EntityType
	IncludeMetadata  bool
	EnableStrictJSON bool
	PromptVersion    string
	ModelParameters  *types.ModelParameters
}

// PromptConfig contains configuration for prompt building
type PromptConfig struct {
	MaxPromptLength  int
	IncludeExamples  bool
	StrictJSONMode   bool
	ContextSize      int
	EnableValidation bool
}

// NewPromptBuilder creates a new prompt builder with default settings
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		DocumentType:     types.DocumentTypeOther,
		MaxEntities:      50,
		MinConfidence:    0.7,
		ContextWindow:    200,
		ExampleMode:      true,
		AnalysisDepth:    types.AnalysisDepthStandard,
		EntityTypes:      types.GetEntityTypesList(),
		IncludeMetadata:  true,
		EnableStrictJSON: true,
		PromptVersion:    "1.0.0",
		ModelParameters: &types.ModelParameters{
			Temperature: 0.1,
			MaxTokens:   2048,
		},
	}
}

// WithDocumentType sets the document type for the prompt
func (pb *PromptBuilder) WithDocumentType(docType types.DocumentType) *PromptBuilder {
	pb.DocumentType = docType
	return pb
}

// WithMaxEntities sets the maximum number of entities to extract
func (pb *PromptBuilder) WithMaxEntities(max int) *PromptBuilder {
	pb.MaxEntities = max
	return pb
}

// WithMinConfidence sets the minimum confidence threshold
func (pb *PromptBuilder) WithMinConfidence(min float64) *PromptBuilder {
	pb.MinConfidence = min
	return pb
}

// WithAnalysisDepth sets the depth of analysis
func (pb *PromptBuilder) WithAnalysisDepth(depth types.AnalysisDepth) *PromptBuilder {
	pb.AnalysisDepth = depth
	return pb
}

// WithEntityTypes sets the specific entity types to extract
func (pb *PromptBuilder) WithEntityTypes(entityTypes []types.EntityType) *PromptBuilder {
	pb.EntityTypes = entityTypes
	return pb
}

// WithContextWindow sets the context window size
func (pb *PromptBuilder) WithContextWindow(size int) *PromptBuilder {
	pb.ContextWindow = size
	return pb
}

// WithStrictJSON enables or disables strict JSON mode
func (pb *PromptBuilder) WithStrictJSON(enabled bool) *PromptBuilder {
	pb.EnableStrictJSON = enabled
	return pb
}

// BuildEntityExtractionPrompt creates a comprehensive entity extraction prompt
func (pb *PromptBuilder) BuildEntityExtractionPrompt(documentName, content string) string {
	var prompt strings.Builder

	// Header with clear instructions
	prompt.WriteString("You are an expert entity extraction system. ")
	prompt.WriteString("Analyze the following document and extract entities with high precision.\n\n")

	// JSON Schema Definition
	prompt.WriteString(pb.buildJSONSchema())

	// Document type specific instructions
	prompt.WriteString(pb.buildDocumentTypeInstructions())

	// Entity type definitions
	prompt.WriteString(pb.buildEntityTypeDefinitions())

	// Examples (if enabled)
	if pb.ExampleMode {
		prompt.WriteString(pb.buildFewShotExamples())
	}

	// Processing parameters
	prompt.WriteString(pb.buildProcessingParameters())

	// Document content
	prompt.WriteString(fmt.Sprintf("\nDocument Name: %s\n", documentName))
	prompt.WriteString(fmt.Sprintf("Document Type: %s\n", pb.DocumentType))
	prompt.WriteString(fmt.Sprintf("Analysis Depth: %s\n", pb.AnalysisDepth))
	prompt.WriteString("\nDocument Content:\n")
	prompt.WriteString(content)
	prompt.WriteString("\n\n")

	// Strict JSON output instructions
	if pb.EnableStrictJSON {
		prompt.WriteString(pb.buildStrictJSONInstructions())
	}

	return prompt.String()
}

// buildJSONSchema creates the JSON schema definition
func (pb *PromptBuilder) buildJSONSchema() string {
	schema := `REQUIRED JSON OUTPUT FORMAT:
{
  "entities": [
    {
      "type": "PERSON|ORGANIZATION|LOCATION|DATE|EMAIL|CONCEPT|METRIC|TECHNOLOGY|URL|CITATION",
      "text": "exact text from document",
      "confidence": 0.0-1.0,
      "start_offset": 0,
      "end_offset": 0,
      "context": "surrounding text for context",
      "normalized_text": "normalized version of text",
      "entity_metadata": {
        "category": "optional category",
        "subcategory": "optional subcategory",
        "tags": ["optional", "tags"],
        "role": "optional role for person/organization",
        "date_precision": "optional for dates (year|month|day|time)",
        "metric_value": "optional numeric value for metrics",
        "metric_unit": "optional unit for metrics"
      }
    }
  ],
  "document_type": "%s",
  "total_entities": 0,
  "processing_time": "%s",
  "quality_metrics": {
    "confidence_score": 0.0-1.0,
    "completeness_score": 0.0-1.0,
    "reliability_score": 0.0-1.0
  }
}

`
	return fmt.Sprintf(schema, pb.DocumentType, time.Now().Format(time.RFC3339))
}

// buildDocumentTypeInstructions creates document-type specific instructions
func (pb *PromptBuilder) buildDocumentTypeInstructions() string {
	var instructions strings.Builder

	instructions.WriteString("DOCUMENT TYPE SPECIFIC GUIDANCE:\n")

	switch pb.DocumentType {
	case types.DocumentTypeResearch:
		instructions.WriteString("- Focus on authors, institutions, methodologies, and technical concepts\n")
		instructions.WriteString("- Extract research domains, technologies, and academic affiliations\n")
		instructions.WriteString("- Pay attention to statistical measures and research metrics\n")

	case types.DocumentTypeBusiness:
		instructions.WriteString("- Focus on companies, executives, business metrics, and strategies\n")
		instructions.WriteString("- Extract KPIs, financial data, and business concepts\n")
		instructions.WriteString("- Identify stakeholders, partnerships, and business relationships\n")

	case types.DocumentTypeArticle:
		instructions.WriteString("- Focus on key people, organizations, and locations mentioned\n")
		instructions.WriteString("- Extract important dates, events, and factual information\n")
		instructions.WriteString("- Identify sources, quotes, and referenced materials\n")

	case types.DocumentTypeReport:
		instructions.WriteString("- Focus on data points, metrics, and analytical findings\n")
		instructions.WriteString("- Extract organizational entities and responsible parties\n")
		instructions.WriteString("- Identify time periods, locations, and scope of analysis\n")

	case types.DocumentTypeLegal:
		instructions.WriteString("- Focus on parties, legal entities, and jurisdictions\n")
		instructions.WriteString("- Extract dates, legal concepts, and regulatory references\n")
		instructions.WriteString("- Identify contract terms, obligations, and legal metrics\n")

	default:
		instructions.WriteString("- Extract all relevant entities based on content analysis\n")
		instructions.WriteString("- Focus on people, organizations, locations, and key concepts\n")
		instructions.WriteString("- Identify important dates, metrics, and technical terms\n")
	}

	instructions.WriteString("\n")
	return instructions.String()
}

// buildEntityTypeDefinitions creates detailed entity type definitions
func (pb *PromptBuilder) buildEntityTypeDefinitions() string {
	var definitions strings.Builder

	definitions.WriteString("ENTITY TYPE DEFINITIONS:\n")

	entityDefs := map[types.EntityType]string{
		types.EntityTypePerson:       "PERSON: Individual people, including names, titles, roles. Include authors, researchers, executives, etc.",
		types.EntityTypeOrganization: "ORGANIZATION: Companies, institutions, universities, government bodies, research groups, etc.",
		types.EntityTypeLocation:     "LOCATION: Geographic locations including cities, countries, regions, addresses, venues.",
		types.EntityTypeDate:         "DATE: Temporal expressions including specific dates, years, periods, deadlines, timeframes.",
		types.EntityTypeEmail:        "EMAIL: Email addresses and contact information.",
		types.EntityTypeConcept:      "CONCEPT: Abstract concepts, methodologies, theories, principles, ideas, frameworks.",
		types.EntityTypeMetric:       "METRIC: Quantitative measures, statistics, KPIs, percentages, scores, measurements.",
		types.EntityTypeTechnology:   "TECHNOLOGY: Software, hardware, platforms, tools, systems, programming languages, frameworks.",
		types.EntityTypeURL:          "URL: Web addresses, links, DOIs, online resources.",
		types.EntityTypeCitation:     "CITATION: References to other works, publications, studies, sources.",
	}

	for _, entityType := range pb.EntityTypes {
		if def, exists := entityDefs[entityType]; exists {
			definitions.WriteString(fmt.Sprintf("- %s\n", def))
		}
	}

	definitions.WriteString("\n")
	return definitions.String()
}

// buildFewShotExamples creates few-shot learning examples
func (pb *PromptBuilder) buildFewShotExamples() string {
	examples := `EXAMPLES:

Example 1 (Research Paper):
Text: "Dr. Sarah Johnson from MIT published a study on machine learning algorithms in 2023."
Entities:
- {"type": "PERSON", "text": "Dr. Sarah Johnson", "confidence": 0.95, "entity_metadata": {"role": "researcher", "title": "Dr."}}
- {"type": "ORGANIZATION", "text": "MIT", "confidence": 0.98, "entity_metadata": {"category": "university"}}
- {"type": "CONCEPT", "text": "machine learning algorithms", "confidence": 0.90, "entity_metadata": {"domain": "artificial intelligence"}}
- {"type": "DATE", "text": "2023", "confidence": 0.95, "entity_metadata": {"date_precision": "year"}}

Example 2 (Business Document):
Text: "Q3 revenue increased by 15% to $2.4M, with AWS contributing significantly to growth."
Entities:
- {"type": "METRIC", "text": "15%", "confidence": 0.98, "entity_metadata": {"metric_type": "percentage", "category": "growth"}}
- {"type": "METRIC", "text": "$2.4M", "confidence": 0.95, "entity_metadata": {"metric_value": 2400000, "metric_unit": "USD"}}
- {"type": "ORGANIZATION", "text": "AWS", "confidence": 0.90, "entity_metadata": {"category": "technology company"}}
- {"type": "DATE", "text": "Q3", "confidence": 0.85, "entity_metadata": {"date_precision": "quarter"}}

`
	return examples
}

// buildProcessingParameters creates processing parameter instructions
func (pb *PromptBuilder) buildProcessingParameters() string {
	var params strings.Builder

	params.WriteString("PROCESSING PARAMETERS:\n")
	params.WriteString(fmt.Sprintf("- Maximum entities to extract: %d\n", pb.MaxEntities))
	params.WriteString(fmt.Sprintf("- Minimum confidence threshold: %.2f\n", pb.MinConfidence))
	params.WriteString(fmt.Sprintf("- Context window size: %d characters\n", pb.ContextWindow))
	params.WriteString(fmt.Sprintf("- Analysis depth: %s\n", pb.AnalysisDepth))

	if len(pb.EntityTypes) < len(types.GetEntityTypesList()) {
		params.WriteString("- Focus on entity types: ")
		typeStrs := make([]string, len(pb.EntityTypes))
		for i, et := range pb.EntityTypes {
			typeStrs[i] = string(et)
		}
		params.WriteString(strings.Join(typeStrs, ", "))
		params.WriteString("\n")
	}

	params.WriteString("\n")
	return params.String()
}

// buildStrictJSONInstructions creates strict JSON output instructions
func (pb *PromptBuilder) buildStrictJSONInstructions() string {
	instructions := `CRITICAL INSTRUCTIONS:
1. Return ONLY the JSON object above
2. Do NOT include any explanatory text, markdown formatting, or code blocks
3. Do NOT prefix with "` + "```json" + `" or suffix with "` + "```" + `"
4. Ensure all JSON is valid and properly formatted
5. All confidence scores must be between 0.0 and 1.0
6. All offsets must be accurate to the source text
7. Include surrounding context for each entity (before/after text)
8. Use exact text from the document - do not paraphrase
9. Ensure entity types match exactly the defined categories
10. Calculate realistic confidence scores based on context clarity

QUALITY REQUIREMENTS:
- Prefer precision over recall (better to miss entities than extract incorrect ones)
- Confidence scores should reflect actual certainty about the extraction
- Context should provide enough information to validate the entity
- Normalized text should handle variations (e.g., "Dr." -> "Doctor")
- Metadata should enhance understanding of the entity

Return the JSON now:`

	return instructions
}

// ValidatePromptLength checks if the prompt exceeds maximum length
func (pb *PromptBuilder) ValidatePromptLength(prompt string, maxLength int) error {
	if len(prompt) > maxLength {
		return fmt.Errorf("prompt length (%d) exceeds maximum (%d)", len(prompt), maxLength)
	}
	return nil
}

// SetStrictJSONMode enables strict JSON output mode
func (pb *PromptBuilder) SetStrictJSONMode(enabled bool) {
	pb.EnableStrictJSON = enabled
}

// AddFewShotExamples enables few-shot learning examples
func (pb *PromptBuilder) AddFewShotExamples(enabled bool) {
	pb.ExampleMode = enabled
}

// GetPromptVersion returns the current prompt version
func (pb *PromptBuilder) GetPromptVersion() string {
	return pb.PromptVersion
}

// BuildClassificationPrompt creates a document classification prompt
func (pb *PromptBuilder) BuildClassificationPrompt(documentName, content string) string {
	return fmt.Sprintf(`Analyze the following document and classify it into one of these categories:
- research_paper: Academic research papers, studies, theses
- business_document: Business reports, memos, proposals, meeting notes
- personal_document: Personal notes, journals, letters, personal content
- article: News articles, blog posts, magazine articles
- report: Technical reports, status reports, analysis reports
- book: Books, chapters, extended literary works
- presentation: Slides, presentation materials
- legal_document: Legal contracts, agreements, court documents
- other: Any other type of document

Document Name: %s

Document Content (first 2000 characters):
%s

Respond with JSON in this exact format:
{
  "primary_type": "category_name",
  "confidence": 0.95,
  "reasoning": "Brief explanation of classification decision",
  "features": {
    "has_abstract": 0.0-1.0,
    "has_references": 0.0-1.0,
    "academic_language": 0.0-1.0,
    "business_terminology": 0.0-1.0,
    "formal_structure": 0.0-1.0
  }
}`, documentName, content)
}

// EstimateTokenCount estimates the token count for the prompt (rough approximation)
func (pb *PromptBuilder) EstimateTokenCount(prompt string) int {
	// Rough estimation: ~4 characters per token for English text
	return len(prompt) / 4
}

// OptimizeForModel adjusts the prompt builder settings for specific model constraints
func (pb *PromptBuilder) OptimizeForModel(modelName string, maxTokens int) {
	switch modelName {
	case "llama3.2:1b":
		// Optimize for smaller model
		pb.MaxEntities = 30
		pb.ContextWindow = 150
		pb.ExampleMode = true // Keep examples for better performance

	case "llama3.2:3b", "llama3.2:8b":
		// Standard optimization
		pb.MaxEntities = 50
		pb.ContextWindow = 200
		pb.ExampleMode = true

	default:
		// Conservative defaults
		pb.MaxEntities = 40
		pb.ContextWindow = 180
		pb.ExampleMode = true
	}

	if pb.ModelParameters != nil {
		pb.ModelParameters.MaxTokens = maxTokens
	}
}
