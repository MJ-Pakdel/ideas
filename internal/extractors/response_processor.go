package extractors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/example/idaes/internal/types"
)

// ResponseProcessor handles the complete pipeline of processing LLM responses
type ResponseProcessor struct {
	JSONCleaner     *JSONCleaner
	EntityValidator *EntityValidator
	MaxRetries      int
	StrictMode      bool
	EnableCaching   bool
	TimeoutDuration time.Duration
	Logger          *slog.Logger
}

// ProcessingConfig contains configuration for response processing
type ProcessingConfig struct {
	MaxRetries      int
	StrictMode      bool
	EnableCaching   bool
	TimeoutDuration time.Duration
	MinConfidence   float64
	MaxEntities     int
	AllowedTypes    []types.EntityType
	ValidateOffsets bool
}

// ProcessingResult contains the complete result of response processing
type ProcessingResult struct {
	Success          bool
	Result           *types.EntityExtractionResult
	ProcessingTime   time.Duration
	AttemptsUsed     int
	ValidationResult *ValidationResult
	CleaningStats    *CleaningStats
	Errors           []string
	Warnings         []string
}

// NewResponseProcessor creates a new response processor with default settings
func NewResponseProcessor() *ResponseProcessor {
	return &ResponseProcessor{
		JSONCleaner:     NewJSONCleaner(),
		EntityValidator: NewEntityValidator(),
		MaxRetries:      3,
		StrictMode:      false,
		EnableCaching:   true,
		TimeoutDuration: 30 * time.Second,
		Logger:          slog.Default(),
	}
}

// WithConfig configures the response processor
func (rp *ResponseProcessor) WithConfig(config *ProcessingConfig) *ResponseProcessor {
	if config != nil {
		rp.MaxRetries = config.MaxRetries
		rp.StrictMode = config.StrictMode
		rp.EnableCaching = config.EnableCaching
		rp.TimeoutDuration = config.TimeoutDuration

		// Configure sub-components
		rp.EntityValidator = rp.EntityValidator.
			WithStrictMode(config.StrictMode).
			WithMinConfidence(config.MinConfidence).
			WithMaxEntities(config.MaxEntities)

		if len(config.AllowedTypes) > 0 {
			rp.EntityValidator = rp.EntityValidator.WithAllowedTypes(config.AllowedTypes)
		}
	}
	return rp
}

// WithLogger sets the logger for the processor
func (rp *ResponseProcessor) WithLogger(logger *slog.Logger) *ResponseProcessor {
	rp.Logger = logger
	return rp
}

// ProcessResponse processes a raw LLM response and returns structured results
func (rp *ResponseProcessor) ProcessResponse(ctx context.Context, rawResponse string, documentContent string, request *types.ExtractionRequest) (*ProcessingResult, error) {
	startTime := time.Now()

	result := &ProcessingResult{
		Success:      false,
		AttemptsUsed: 1,
		Errors:       make([]string, 0),
		Warnings:     make([]string, 0),
	}

	// Check context cancellation
	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	// Step 1: Clean the JSON response
	rp.Logger.Debug("Starting JSON cleaning", "response_length", len(rawResponse))
	cleanedJSON, cleaningStats := rp.JSONCleaner.CleanResponseWithStats(rawResponse)
	result.CleaningStats = cleaningStats

	if cleanedJSON == "" {
		result.Errors = append(result.Errors, "failed to extract valid JSON from response")
		return result, fmt.Errorf("no valid JSON found in response")
	}

	// Step 2: Parse JSON into extraction result
	rp.Logger.Debug("Parsing JSON response", "cleaned_length", len(cleanedJSON))
	extractionResult, parseErr := rp.parseJSONResponse(cleanedJSON, request)
	if parseErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("JSON parsing failed: %v", parseErr))

		// Try alternative parsing strategies
		if altResult, altErr := rp.tryAlternativeParsing(cleanedJSON, request); altErr == nil {
			extractionResult = altResult
			result.Warnings = append(result.Warnings, "used alternative JSON parsing")
		} else {
			return result, fmt.Errorf("failed to parse JSON: %v", parseErr)
		}
	}

	// Step 3: Validate and filter entities
	rp.Logger.Debug("Validating entities", "entity_count", len(extractionResult.Entities))
	validatedEntities, validationResult := rp.EntityValidator.ValidateAndFilterEntities(extractionResult.Entities, documentContent)
	result.ValidationResult = validationResult

	// Update extraction result with validated entities
	extractionResult.Entities = validatedEntities
	extractionResult.TotalEntities = len(validatedEntities)

	// Step 4: Update quality metrics
	rp.updateQualityMetrics(extractionResult, validationResult, cleaningStats)

	// Step 5: Final validation
	if validationResult.HasCriticalErrors() {
		result.Errors = append(result.Errors, validationResult.Errors...)
		if rp.StrictMode {
			return result, fmt.Errorf("validation failed: %s", strings.Join(validationResult.Errors, "; "))
		}
	}

	if validationResult.HasWarnings() {
		result.Warnings = append(result.Warnings, validationResult.Warnings...)
	}

	// Success
	result.Success = true
	result.Result = extractionResult
	result.ProcessingTime = time.Since(startTime)

	rp.Logger.Info("Response processing completed successfully",
		"entities_extracted", len(validatedEntities),
		"processing_time", result.ProcessingTime,
		"validation_warnings", len(result.Warnings))

	return result, nil
}

// parseJSONResponse parses the cleaned JSON into an EntityExtractionResult
func (rp *ResponseProcessor) parseJSONResponse(jsonStr string, request *types.ExtractionRequest) (*types.EntityExtractionResult, error) {
	// First try to parse as complete EntityExtractionResult
	var fullResult types.EntityExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &fullResult); err == nil {
		rp.enrichExtractionResult(&fullResult, request)
		return &fullResult, nil
	}

	// Try to parse as simplified format (just entities array)
	var simpleFormat struct {
		Entities       []*types.ExtractedEntity `json:"entities"`
		DocumentType   string                   `json:"document_type,omitempty"`
		TotalEntities  int                      `json:"total_entities,omitempty"`
		ProcessingTime string                   `json:"processing_time,omitempty"`
		QualityMetrics *types.QualityMetrics    `json:"quality_metrics,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &simpleFormat); err == nil {
		result := types.NewEntityExtractionResult(request.DocumentID, request.DocumentType)
		result.Entities = simpleFormat.Entities
		result.TotalEntities = len(simpleFormat.Entities)

		if simpleFormat.DocumentType != "" {
			result.DocumentType = types.DocumentType(simpleFormat.DocumentType)
		}

		if simpleFormat.QualityMetrics != nil {
			result.QualityMetrics = simpleFormat.QualityMetrics
		}

		rp.enrichExtractionResult(result, request)
		return result, nil
	}

	return nil, fmt.Errorf("unable to parse JSON response")
}

// tryAlternativeParsing attempts alternative parsing strategies
func (rp *ResponseProcessor) tryAlternativeParsing(jsonStr string, request *types.ExtractionRequest) (*types.EntityExtractionResult, error) {
	// Strategy 1: Try to parse individual entities
	var entitiesOnly struct {
		Entities []*types.ExtractedEntity `json:"entities"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &entitiesOnly); err == nil {
		result := types.NewEntityExtractionResult(request.DocumentID, request.DocumentType)
		result.Entities = entitiesOnly.Entities
		result.TotalEntities = len(entitiesOnly.Entities)
		rp.enrichExtractionResult(result, request)
		return result, nil
	}

	return nil, fmt.Errorf("all alternative parsing strategies failed")
}

// enrichExtractionResult enriches the extraction result with additional metadata
func (rp *ResponseProcessor) enrichExtractionResult(result *types.EntityExtractionResult, request *types.ExtractionRequest) {
	if result == nil {
		return
	}

	// Set basic metadata
	if result.DocumentID == "" {
		result.DocumentID = request.DocumentID
	}

	if result.DocumentType == "" {
		result.DocumentType = request.DocumentType
	}

	if result.ProcessingTime == "" {
		result.ProcessingTime = time.Now().Format(time.RFC3339)
	}

	// Enrich individual entities
	for _, entity := range result.Entities {
		if entity != nil {
			rp.enrichEntity(entity, request)
		}
	}

	// Initialize quality metrics if not present
	if result.QualityMetrics == nil {
		result.QualityMetrics = &types.QualityMetrics{}
	}

	// Set processing metadata
	if result.ExtractionMetadata == nil {
		result.ExtractionMetadata = make(map[string]any)
	}

	result.ExtractionMetadata["processor_version"] = "1.0.0"
	result.ExtractionMetadata["processed_at"] = time.Now().Format(time.RFC3339)
}

// enrichEntity enriches individual entities with additional metadata
func (rp *ResponseProcessor) enrichEntity(entity *types.ExtractedEntity, request *types.ExtractionRequest) {
	if entity == nil {
		return
	}

	// Set basic metadata
	if entity.DocumentID == "" {
		entity.DocumentID = request.DocumentID
	}

	if entity.ExtractedAt.IsZero() {
		entity.ExtractedAt = time.Now()
	}

	if entity.ExtractionID == "" {
		entity.ExtractionID = request.ID
	}

	// Set normalized text if empty
	if entity.NormalizedText == "" {
		entity.NormalizedText = strings.TrimSpace(entity.Text)
	}

	// Initialize metadata if needed
	if entity.EntityMetadata == nil {
		entity.EntityMetadata = &types.EntityExtractionMeta{}
	}

	if entity.ValidationFlags == nil {
		entity.ValidationFlags = &types.ValidationFlags{
			IsValid: true,
		}
	}
}

// updateQualityMetrics updates the quality metrics based on validation results
func (rp *ResponseProcessor) updateQualityMetrics(result *types.EntityExtractionResult, validation *ValidationResult, cleaning *CleaningStats) {
	if result.QualityMetrics == nil {
		result.QualityMetrics = &types.QualityMetrics{}
	}

	metrics := result.QualityMetrics

	// Calculate confidence score
	if len(result.Entities) > 0 {
		totalConfidence := 0.0
		for _, entity := range result.Entities {
			if entity != nil {
				totalConfidence += entity.Confidence
			}
		}
		metrics.ConfidenceScore = totalConfidence / float64(len(result.Entities))
	}

	// Calculate completeness score
	if validation != nil {
		totalProcessed := validation.EntityCount + validation.InvalidEntities
		if totalProcessed > 0 {
			metrics.CompletenessScore = float64(validation.EntityCount) / float64(totalProcessed)
		}
	}

	// Calculate reliability score based on validation results
	if validation != nil {
		errorPenalty := float64(len(validation.Errors)) * 0.1
		warningPenalty := float64(len(validation.Warnings)) * 0.05
		metrics.ReliabilityScore = 1.0 - errorPenalty - warningPenalty
		if metrics.ReliabilityScore < 0 {
			metrics.ReliabilityScore = 0
		}
	}

	// Overall score is weighted average
	metrics.OverallScore = (metrics.ConfidenceScore*0.4 + metrics.CompletenessScore*0.3 + metrics.ReliabilityScore*0.3)

	// Set processing time
	if cleaning != nil {
		metrics.ProcessingTimeMs = int64(time.Since(time.Now()).Milliseconds()) // This would be calculated from actual processing
	}

	// Initialize performance metrics
	if metrics.PerformanceMetrics == nil {
		metrics.PerformanceMetrics = &types.PerformanceMetrics{}
	}

	// Initialize validation results
	if metrics.ValidationResults == nil && validation != nil {
		metrics.ValidationResults = &types.ValidationResults{
			TotalEntities:     validation.EntityCount + validation.InvalidEntities,
			ValidEntities:     validation.EntityCount,
			InvalidEntities:   validation.InvalidEntities,
			DuplicateEntities: validation.DuplicatesFound,
			ErrorCount:        len(validation.Errors),
			WarningCount:      len(validation.Warnings),
		}

		if metrics.ValidationResults.TotalEntities > 0 {
			metrics.ValidationResults.ValidationRate = float64(metrics.ValidationResults.ValidEntities) / float64(metrics.ValidationResults.TotalEntities)
		}
	}
}

// ProcessResponseWithRetry processes a response with retry logic
func (rp *ResponseProcessor) ProcessResponseWithRetry(ctx context.Context, rawResponse string, documentContent string, request *types.ExtractionRequest) (*ProcessingResult, error) {
	var lastErr error
	var result *ProcessingResult

	for attempt := 1; attempt <= rp.MaxRetries; attempt++ {
		rp.Logger.Debug("Processing attempt", "attempt", attempt, "max_retries", rp.MaxRetries)

		result, lastErr = rp.ProcessResponse(ctx, rawResponse, documentContent, request)
		if lastErr == nil && result.Success {
			result.AttemptsUsed = attempt
			return result, nil
		}

		// Check if we should retry
		if attempt < rp.MaxRetries && rp.shouldRetry(lastErr, result) {
			rp.Logger.Debug("Retrying processing", "attempt", attempt, "error", lastErr)
			continue
		}

		break
	}

	// All attempts failed
	if result == nil {
		result = &ProcessingResult{
			Success:      false,
			AttemptsUsed: rp.MaxRetries,
			Errors:       []string{fmt.Sprintf("all %d attempts failed: %v", rp.MaxRetries, lastErr)},
		}
	} else {
		result.AttemptsUsed = rp.MaxRetries
	}

	return result, lastErr
}

// shouldRetry determines whether processing should be retried
func (rp *ResponseProcessor) shouldRetry(err error, result *ProcessingResult) bool {
	if err == nil {
		return false
	}

	// Don't retry on context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Retry on parsing errors but not on validation errors in strict mode
	if rp.StrictMode && result != nil && result.ValidationResult != nil && result.ValidationResult.HasCriticalErrors() {
		return false
	}

	// Retry on JSON parsing errors
	if strings.Contains(err.Error(), "JSON") || strings.Contains(err.Error(), "parsing") {
		return true
	}

	return false
}

// GetProcessingStats returns statistics about the processing
func (pr *ProcessingResult) GetProcessingStats() map[string]any {
	stats := map[string]any{
		"success":         pr.Success,
		"processing_time": pr.ProcessingTime.String(),
		"attempts_used":   pr.AttemptsUsed,
		"error_count":     len(pr.Errors),
		"warning_count":   len(pr.Warnings),
	}

	if pr.Result != nil {
		stats["entities_extracted"] = pr.Result.TotalEntities
		if pr.Result.QualityMetrics != nil {
			stats["overall_score"] = pr.Result.QualityMetrics.OverallScore
			stats["confidence_score"] = pr.Result.QualityMetrics.ConfidenceScore
		}
	}

	if pr.CleaningStats != nil {
		stats["original_length"] = pr.CleaningStats.OriginalLength
		stats["cleaned_length"] = pr.CleaningStats.CleanedLength
		stats["markdown_removed"] = pr.CleaningStats.MarkdownRemoved
	}

	if pr.ValidationResult != nil {
		stats["duplicates_found"] = pr.ValidationResult.DuplicatesFound
		stats["invalid_entities"] = pr.ValidationResult.InvalidEntities
	}

	return stats
}
