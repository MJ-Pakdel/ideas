package extractors

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/example/idaes/internal/types"
)

// RetryConfig defines configuration for retry behavior
type RetryConfig struct {
	MaxAttempts     int           `json:"max_attempts"`
	BaseDelay       time.Duration `json:"base_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	EnableJitter    bool          `json:"enable_jitter"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// DefaultRetryConfig returns sensible defaults for retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		EnableJitter:  true,
		RetryableErrors: []string{
			"json parse error",
			"empty response",
			"malformed response",
			"timeout",
			"connection error",
		},
	}
}

// RetryStrategy implements exponential backoff with jitter
type RetryStrategy struct {
	config RetryConfig
}

// NewRetryStrategy creates a new retry strategy with the given configuration
func NewRetryStrategy(config RetryConfig) *RetryStrategy {
	return &RetryStrategy{
		config: config,
	}
}

// ShouldRetry determines if an error is retryable based on configuration
func (rs *RetryStrategy) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}

	if attempt >= rs.config.MaxAttempts {
		return false
	}

	errorMsg := err.Error()
	for _, retryableError := range rs.config.RetryableErrors {
		if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(retryableError)) {
			return true
		}
	}

	return false
}

// GetDelay calculates the delay for the next retry attempt
func (rs *RetryStrategy) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return rs.config.BaseDelay
	}

	// Exponential backoff: base_delay * (backoff_factor ^ attempt)
	delay := float64(rs.config.BaseDelay) * math.Pow(rs.config.BackoffFactor, float64(attempt))

	// Cap at max delay
	if delay > float64(rs.config.MaxDelay) {
		delay = float64(rs.config.MaxDelay)
	}

	result := time.Duration(delay)

	// Add jitter to prevent thundering herd
	if rs.config.EnableJitter {
		jitter := time.Duration(float64(result) * 0.1 * (2*rand.Float64() - 1)) // ±10% jitter
		result += jitter
	}

	return result
}

// RetryableExtractor wraps any extraction function with retry logic
type RetryableExtractor struct {
	strategy    *RetryStrategy
	baseExtract func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error)
}

// NewRetryableExtractor creates a new retryable extractor
func NewRetryableExtractor(
	strategy *RetryStrategy,
	baseExtract func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error),
) *RetryableExtractor {
	return &RetryableExtractor{
		strategy:    strategy,
		baseExtract: baseExtract,
	}
}

// Extract performs extraction with retry logic
func (re *RetryableExtractor) Extract(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
	var lastErr error
	var lastResult *types.EntityExtractionResult

	for attempt := 0; attempt < re.strategy.config.MaxAttempts; attempt++ {
		// Create a new context with timeout for this attempt
		// Increased timeout to 120 seconds for LLM operations (citations can be complex)
		attemptCtx, cancel := context.WithTimeout(ctx, 120*time.Second)

		result, err := re.baseExtract(attemptCtx, input)
		cancel()

		if err == nil && result != nil {
			// Success - record attempt metadata
			if result.ExtractionMetadata == nil {
				result.ExtractionMetadata = make(map[string]any)
			}
			result.ExtractionMetadata["attempt_count"] = attempt + 1
			result.ExtractionMetadata["retry_reason"] = ""
			if lastErr != nil {
				result.ExtractionMetadata["retry_reason"] = fmt.Sprintf("Recovered from: %v", lastErr)
			}
			return result, nil
		}

		lastErr = err
		lastResult = result

		// Check if we should retry
		if !re.strategy.ShouldRetry(err, attempt+1) {
			break
		}

		// Wait before retrying (unless this is the last attempt)
		if attempt < re.strategy.config.MaxAttempts-1 {
			delay := re.strategy.GetDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	// All attempts failed - return enriched error information
	if lastResult != nil {
		if lastResult.ExtractionMetadata == nil {
			lastResult.ExtractionMetadata = make(map[string]any)
		}
		lastResult.ExtractionMetadata["attempt_count"] = re.strategy.config.MaxAttempts
		lastResult.ExtractionMetadata["retry_reason"] = fmt.Sprintf("Failed after %d attempts: %v", re.strategy.config.MaxAttempts, lastErr)
		return lastResult, fmt.Errorf("extraction failed after %d attempts: %w", re.strategy.config.MaxAttempts, lastErr)
	}

	return nil, fmt.Errorf("extraction failed after %d attempts: %w", re.strategy.config.MaxAttempts, lastErr)
}

// FallbackStrategy defines different fallback approaches
type FallbackStrategy int

const (
	FallbackNone FallbackStrategy = iota
	FallbackSimplified
	FallbackBasicEntities
	FallbackKeywordExtraction
)

// FallbackConfig defines configuration for fallback behavior
type FallbackConfig struct {
	Strategy                 FallbackStrategy `json:"strategy"`
	SimplifiedPromptEnabled  bool             `json:"simplified_prompt_enabled"`
	KeywordExtractionEnabled bool             `json:"keyword_extraction_enabled"`
	MinEntityThreshold       int              `json:"min_entity_threshold"`
}

// DefaultFallbackConfig returns sensible defaults for fallback behavior
func DefaultFallbackConfig() FallbackConfig {
	return FallbackConfig{
		Strategy:                 FallbackSimplified,
		SimplifiedPromptEnabled:  true,
		KeywordExtractionEnabled: true,
		MinEntityThreshold:       1,
	}
}

// FallbackExtractor implements fallback strategies for failed extractions
type FallbackExtractor struct {
	config            FallbackConfig
	promptBuilder     *PromptBuilder
	responseProcessor *ResponseProcessor
	baseExtract       func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error)
}

// NewFallbackExtractor creates a new fallback extractor
func NewFallbackExtractor(
	config FallbackConfig,
	promptBuilder *PromptBuilder,
	responseProcessor *ResponseProcessor,
	baseExtract func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error),
) *FallbackExtractor {
	return &FallbackExtractor{
		config:            config,
		promptBuilder:     promptBuilder,
		responseProcessor: responseProcessor,
		baseExtract:       baseExtract,
	}
}

// ExtractWithFallback attempts extraction with fallback strategies
func (fe *FallbackExtractor) ExtractWithFallback(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
	// Try primary extraction first
	result, err := fe.baseExtract(ctx, input)
	if err == nil && fe.isResultAcceptable(result) {
		if result.ExtractionMetadata == nil {
			result.ExtractionMetadata = make(map[string]any)
		}
		result.ExtractionMetadata["fallback_used"] = false
		return result, nil
	}

	// Primary extraction failed or result unacceptable - try fallbacks
	switch fe.config.Strategy {
	case FallbackSimplified:
		return fe.trySimplifiedExtraction(ctx, input)
	case FallbackBasicEntities:
		return fe.tryBasicEntityExtraction(ctx, input)
	case FallbackKeywordExtraction:
		return fe.tryKeywordExtraction(ctx, input)
	default:
		return result, err // Return original result if no fallback
	}
}

// trySimplifiedExtraction uses a simplified prompt for extraction
func (fe *FallbackExtractor) trySimplifiedExtraction(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
	if !fe.config.SimplifiedPromptEnabled {
		return nil, errors.New("simplified prompt fallback disabled")
	}

	// Create simplified input with basic analysis depth
	fallbackInput := &types.ExtractionInput{
		DocumentID:    input.DocumentID,
		DocumentName:  input.DocumentName,
		Content:       input.Content,
		DocumentType:  input.DocumentType,
		AnalysisDepth: types.AnalysisDepthBasic, // Force basic analysis
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
			types.EntityTypeOrganization,
			types.EntityTypeLocation,
		},
		Options: input.Options,
	}

	result, err := fe.baseExtract(ctx, fallbackInput)
	if err == nil {
		if result.ExtractionMetadata == nil {
			result.ExtractionMetadata = make(map[string]any)
		}
		result.ExtractionMetadata["fallback_used"] = true
		result.ExtractionMetadata["fallback_strategy"] = "simplified_prompt"
	}
	return result, err
}

// tryBasicEntityExtraction focuses only on basic entity types
func (fe *FallbackExtractor) tryBasicEntityExtraction(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
	// Create basic entity extraction input
	fallbackInput := &types.ExtractionInput{
		DocumentID:    input.DocumentID,
		DocumentName:  input.DocumentName,
		Content:       input.Content,
		DocumentType:  types.DocumentTypeOther, // Force generic processing
		AnalysisDepth: types.AnalysisDepthBasic,
		RequiredEntityTypes: []types.EntityType{
			types.EntityTypePerson,
			types.EntityTypeOrganization,
		},
		Options: input.Options,
	}

	result, err := fe.baseExtract(ctx, fallbackInput)
	if err == nil {
		if result.ExtractionMetadata == nil {
			result.ExtractionMetadata = make(map[string]any)
		}
		result.ExtractionMetadata["fallback_used"] = true
		result.ExtractionMetadata["fallback_strategy"] = "basic_entities"
	}
	return result, err
}

// tryKeywordExtraction uses simple keyword-based extraction as last resort
func (fe *FallbackExtractor) tryKeywordExtraction(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
	if !fe.config.KeywordExtractionEnabled {
		return nil, errors.New("keyword extraction fallback disabled")
	}

	// Simple keyword-based extraction (placeholder implementation)
	entities := fe.extractKeywordBasedEntities(input.Content)

	result := &types.EntityExtractionResult{
		ID:               fmt.Sprintf("fallback_%d", time.Now().Unix()),
		DocumentID:       input.DocumentID,
		DocumentType:     input.DocumentType,
		Entities:         entities,
		TotalEntities:    len(entities),
		ProcessingTime:   "10ms", // Approximate
		ProcessingTimeMs: 10,
		QualityMetrics: &types.QualityMetrics{
			OverallScore:      0.3, // Low confidence for keyword extraction
			ConfidenceScore:   0.3,
			CompletenessScore: 0.2,
			ReliabilityScore:  0.3,
			ConsistencyScore:  0.3,
			ProcessingTimeMs:  10,
		},
		ExtractionMetadata: map[string]any{
			"fallback_used":     true,
			"fallback_strategy": "keyword_extraction",
			"attempt_count":     1,
		},
		ExtractorVersion: "1.0.0",
		ModelUsed:        "keyword_extractor",
		PromptVersion:    "fallback",
		CreatedAt:        time.Now(),
		Method:           types.ExtractorMethodLLM,
	}

	return result, nil
}

// isResultAcceptable checks if extraction result meets minimum quality thresholds
func (fe *FallbackExtractor) isResultAcceptable(result *types.EntityExtractionResult) bool {
	if result == nil {
		return false
	}

	// Check minimum entity threshold
	if len(result.Entities) < fe.config.MinEntityThreshold {
		return false
	}

	// Check quality score if available
	if result.QualityMetrics != nil && result.QualityMetrics.OverallScore < 0.5 {
		return false
	}

	return true
}

// extractKeywordBasedEntities performs simple keyword-based entity extraction
func (fe *FallbackExtractor) extractKeywordBasedEntities(content string) []*types.ExtractedEntity {
	// Simple placeholder implementation
	// In a real implementation, this would use NLP libraries or regex patterns
	var entities []*types.ExtractedEntity

	// Example: Look for common patterns
	// This is a very basic implementation - real keyword extraction would be more sophisticated
	keywords := []string{"research", "study", "analysis", "data", "results"}

	for i, keyword := range keywords {
		if strings.Contains(strings.ToLower(content), strings.ToLower(keyword)) {
			entities = append(entities, &types.ExtractedEntity{
				ID:             fmt.Sprintf("keyword_%d", i),
				Type:           types.EntityTypeConcept,
				Text:           keyword,
				NormalizedText: strings.ToLower(keyword),
				Confidence:     0.3,
				StartOffset:    0,
				EndOffset:      len(keyword),
				Context:        fmt.Sprintf("Found keyword: %s", keyword),
				DocumentID:     "",
				ExtractedAt:    time.Now(),
				ExtractionID:   fmt.Sprintf("fallback_%d", time.Now().Unix()),
			})
		}
	}

	return entities
}
