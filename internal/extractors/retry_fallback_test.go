package extractors

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/example/idaes/internal/types"
)

// TestRetryStrategy tests the retry strategy implementation
func TestRetryStrategy(t *testing.T) {
	tests := []struct {
		name          string
		config        RetryConfig
		error         error
		attempt       int
		shouldRetry   bool
		expectedDelay time.Duration
	}{
		{
			name:        "successful extraction",
			config:      DefaultRetryConfig(),
			error:       nil,
			attempt:     1,
			shouldRetry: false,
		},
		{
			name:        "retryable error within max attempts",
			config:      DefaultRetryConfig(),
			error:       errors.New("json parse error occurred"),
			attempt:     1,
			shouldRetry: true,
		},
		{
			name:        "retryable error exceeding max attempts",
			config:      DefaultRetryConfig(),
			error:       errors.New("timeout error"),
			attempt:     4,
			shouldRetry: false,
		},
		{
			name:        "non-retryable error",
			config:      DefaultRetryConfig(),
			error:       errors.New("authentication failed"),
			attempt:     1,
			shouldRetry: false,
		},
		{
			name: "custom retry config",
			config: RetryConfig{
				MaxAttempts:     5,
				BaseDelay:       50 * time.Millisecond,
				MaxDelay:        2 * time.Second,
				BackoffFactor:   1.5,
				EnableJitter:    false,
				RetryableErrors: []string{"custom error"},
			},
			error:       errors.New("custom error occurred"),
			attempt:     2,
			shouldRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewRetryStrategy(tt.config)

			// Test ShouldRetry
			result := strategy.ShouldRetry(tt.error, tt.attempt)
			if result != tt.shouldRetry {
				t.Errorf("ShouldRetry() = %v, want %v", result, tt.shouldRetry)
			}

			// Test GetDelay
			if tt.shouldRetry {
				delay := strategy.GetDelay(tt.attempt - 1)
				if delay < 0 {
					t.Errorf("GetDelay() returned negative delay: %v", delay)
				}
				if delay > tt.config.MaxDelay {
					t.Errorf("GetDelay() = %v, exceeds max delay %v", delay, tt.config.MaxDelay)
				}
			}
		})
	}
}

// TestRetryableExtractor tests the retryable extractor functionality
func TestRetryableExtractor(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		extractorFunc  func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error)
		expectedResult bool
		expectedError  bool
		expectRetry    bool
	}{
		{
			name: "successful extraction on first attempt",
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return &types.EntityExtractionResult{
					ID:         "test-1",
					DocumentID: input.DocumentID,
					Entities:   []*types.ExtractedEntity{},
				}, nil
			},
			expectedResult: true,
			expectedError:  false,
			expectRetry:    false,
		},
		{
			name: "failure then success",
			extractorFunc: func() func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				callCount := 0
				return func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
					callCount++
					if callCount == 1 {
						return nil, errors.New("json parse error")
					}
					return &types.EntityExtractionResult{
						ID:         "test-2",
						DocumentID: input.DocumentID,
						Entities:   []*types.ExtractedEntity{},
					}, nil
				}
			}(),
			expectedResult: true,
			expectedError:  false,
			expectRetry:    true,
		},
		{
			name: "persistent failure",
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return nil, errors.New("json parse error")
			},
			expectedResult: false,
			expectedError:  true,
			expectRetry:    true,
		},
		{
			name: "non-retryable error",
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return nil, errors.New("authentication failed")
			},
			expectedResult: false,
			expectedError:  true,
			expectRetry:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultRetryConfig()
			config.BaseDelay = 1 * time.Millisecond // Speed up tests
			strategy := NewRetryStrategy(config)
			extractor := NewRetryableExtractor(strategy, tt.extractorFunc)

			input := &types.ExtractionInput{
				DocumentID:    "test-doc",
				DocumentName:  "test.txt",
				Content:       "test content",
				DocumentType:  types.DocumentTypeOther,
				AnalysisDepth: types.AnalysisDepthBasic,
			}

			result, err := extractor.Extract(ctx, input)

			if tt.expectedResult && result == nil {
				t.Error("Expected result but got nil")
			}
			if !tt.expectedResult && result != nil {
				t.Error("Expected nil result but got result")
			}
			if tt.expectedError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check retry metadata
			if result != nil && tt.expectRetry {
				if result.ExtractionMetadata == nil {
					t.Error("Expected extraction metadata but got nil")
				} else {
					if attemptCount, ok := result.ExtractionMetadata["attempt_count"]; !ok {
						t.Error("Expected attempt_count in metadata")
					} else if count, ok := attemptCount.(int); !ok || count <= 1 {
						t.Errorf("Expected attempt_count > 1, got %v", count)
					}
				}
			}
		})
	}
}

// TestFallbackExtractor tests the fallback extraction strategies
func TestFallbackExtractor(t *testing.T) {
	ctx := t.Context()
	mockPromptBuilder := &PromptBuilder{}
	mockResponseProcessor := &ResponseProcessor{}

	tests := []struct {
		name           string
		config         FallbackConfig
		extractorFunc  func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error)
		expectedResult bool
		expectedError  bool
		expectFallback bool
	}{
		{
			name:   "successful primary extraction",
			config: DefaultFallbackConfig(),
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return &types.EntityExtractionResult{
					ID:         "test-1",
					DocumentID: input.DocumentID,
					Entities: []*types.ExtractedEntity{
						{
							ID:   "entity-1",
							Type: types.EntityTypePerson,
							Text: "John Doe",
						},
						{
							ID:   "entity-2",
							Type: types.EntityTypeConcept,
							Text: "machine learning",
						},
					},
					QualityMetrics: &types.QualityMetrics{
						OverallScore: 0.8,
					},
				}, nil
			},
			expectedResult: true,
			expectedError:  false,
			expectFallback: false,
		},
		{
			name:   "primary extraction fails, fallback succeeds",
			config: DefaultFallbackConfig(),
			extractorFunc: func() func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				callCount := 0
				return func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
					callCount++
					if callCount == 1 {
						// Primary extraction fails
						return nil, errors.New("extraction failed")
					}
					// Fallback succeeds
					return &types.EntityExtractionResult{
						ID:         "fallback-1",
						DocumentID: input.DocumentID,
						Entities: []*types.ExtractedEntity{
							{
								ID:   "entity-1",
								Type: types.EntityTypePerson,
								Text: "Jane Doe",
							},
						},
						QualityMetrics: &types.QualityMetrics{
							OverallScore: 0.6,
						},
					}, nil
				}
			}(),
			expectedResult: true,
			expectedError:  false,
			expectFallback: true,
		},
		{
			name: "keyword extraction fallback",
			config: FallbackConfig{
				Strategy:                 FallbackKeywordExtraction,
				SimplifiedPromptEnabled:  false,
				KeywordExtractionEnabled: true,
				MinEntityThreshold:       1,
			},
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return nil, errors.New("extraction failed")
			},
			expectedResult: true,
			expectedError:  false,
			expectFallback: true,
		},
		{
			name: "all fallbacks disabled",
			config: FallbackConfig{
				Strategy:                 FallbackNone,
				SimplifiedPromptEnabled:  false,
				KeywordExtractionEnabled: false,
				MinEntityThreshold:       1,
			},
			extractorFunc: func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
				return nil, errors.New("extraction failed")
			},
			expectedResult: false,
			expectedError:  true,
			expectFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewFallbackExtractor(
				tt.config,
				mockPromptBuilder,
				mockResponseProcessor,
				tt.extractorFunc,
			)

			input := &types.ExtractionInput{
				DocumentID:    "test-doc",
				DocumentName:  "test.txt",
				Content:       "This is a research study about data analysis and results.",
				DocumentType:  types.DocumentTypeResearch,
				AnalysisDepth: types.AnalysisDepthDetailed,
			}

			result, err := extractor.ExtractWithFallback(ctx, input)

			if tt.expectedResult && result == nil {
				t.Error("Expected result but got nil")
			}
			if !tt.expectedResult && result != nil {
				t.Error("Expected nil result but got result")
			}
			if tt.expectedError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check fallback metadata
			if result != nil {
				if result.ExtractionMetadata == nil {
					t.Error("Expected extraction metadata but got nil")
				} else {
					fallbackUsed, ok := result.ExtractionMetadata["fallback_used"]
					if !ok {
						t.Error("Expected fallback_used in metadata")
					} else if used, ok := fallbackUsed.(bool); !ok {
						t.Error("Expected fallback_used to be boolean")
					} else if used != tt.expectFallback {
						t.Errorf("Expected fallback_used=%v, got %v", tt.expectFallback, used)
					} else if tt.expectFallback && used {
						if _, ok := result.ExtractionMetadata["fallback_strategy"]; !ok {
							t.Error("Expected fallback_strategy in metadata when fallback is used")
						}
					}
				}
			}
		})
	}
}

// TestKeywordExtraction tests the keyword-based fallback extraction
func TestKeywordExtraction(t *testing.T) {
	config := FallbackConfig{
		Strategy:                 FallbackKeywordExtraction,
		KeywordExtractionEnabled: true,
		MinEntityThreshold:       0,
	}

	extractor := NewFallbackExtractor(
		config,
		&PromptBuilder{},
		&ResponseProcessor{},
		func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
			return nil, errors.New("primary extraction failed")
		},
	)

	tests := []struct {
		name             string
		content          string
		expectedCount    int
		expectedKeywords []string
	}{
		{
			name:             "content with multiple keywords",
			content:          "This research study provides comprehensive data analysis and shows significant results.",
			expectedCount:    5, // research, study, data, analysis, results
			expectedKeywords: []string{"research", "study", "data", "analysis", "results"},
		},
		{
			name:             "content with no keywords",
			content:          "Hello world, this is a simple text without any special terms.",
			expectedCount:    0,
			expectedKeywords: []string{},
		},
		{
			name:             "content with repeated keywords",
			content:          "The research team conducted research on data analysis using research methods.",
			expectedCount:    3, // research (counted once), data, analysis
			expectedKeywords: []string{"research", "data", "analysis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &types.ExtractionInput{
				DocumentID:    "test-doc",
				DocumentName:  "test.txt",
				Content:       tt.content,
				DocumentType:  types.DocumentTypeResearch,
				AnalysisDepth: types.AnalysisDepthBasic,
			}

			ctx := context.Background()
			result, err := extractor.ExtractWithFallback(ctx, input)

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if result == nil {
				t.Fatal("Expected result but got nil")
			}

			if len(result.Entities) != tt.expectedCount {
				t.Errorf("Expected %d entities, got %d", tt.expectedCount, len(result.Entities))
			}

			// Check that expected keywords are found
			foundKeywords := make(map[string]bool)
			for _, entity := range result.Entities {
				foundKeywords[entity.Text] = true
			}

			for _, expectedKeyword := range tt.expectedKeywords {
				if !foundKeywords[expectedKeyword] {
					t.Errorf("Expected keyword '%s' not found", expectedKeyword)
				}
			}

			// Verify fallback metadata
			if result.ExtractionMetadata == nil {
				t.Error("Expected extraction metadata but got nil")
			} else {
				if strategy, ok := result.ExtractionMetadata["fallback_strategy"]; !ok {
					t.Error("Expected fallback_strategy in metadata")
				} else if strategy != "keyword_extraction" {
					t.Errorf("Expected fallback_strategy 'keyword_extraction', got '%v'", strategy)
				}
			}
		})
	}
}

// TestRetryWithTimeout tests retry behavior with context timeouts
func TestRetryWithTimeout(t *testing.T) {
	ctx := t.Context()
	config := RetryConfig{
		MaxAttempts:     5,
		BaseDelay:       10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		BackoffFactor:   2.0,
		EnableJitter:    false,
		RetryableErrors: []string{"timeout"},
	}

	strategy := NewRetryStrategy(config)

	slowExtractor := func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
		time.Sleep(50 * time.Millisecond) // Simulate slow operation
		return nil, errors.New("timeout error")
	}

	extractor := NewRetryableExtractor(strategy, slowExtractor)

	input := &types.ExtractionInput{
		DocumentID:    "test-doc",
		DocumentName:  "test.txt",
		Content:       "test content",
		DocumentType:  types.DocumentTypeOther,
		AnalysisDepth: types.AnalysisDepthBasic,
	}

	// Test with short overall timeout
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := extractor.Extract(ctx, input)
	duration := time.Since(start)

	if result != nil {
		t.Error("Expected nil result due to timeout")
	}
	if err == nil {
		t.Error("Expected error due to timeout")
	}
	if err != nil && !strings.Contains(err.Error(), "extraction failed after") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected retry or timeout error message, got: %v", err)
	}
	if duration > 300*time.Millisecond {
		t.Errorf("Extraction took too long: %v", duration)
	}
}

// BenchmarkRetryableExtractor benchmarks the retry extractor performance
func BenchmarkRetryableExtractor(b *testing.B) {
	config := DefaultRetryConfig()
	config.BaseDelay = 1 * time.Microsecond // Minimal delay for benchmarking
	strategy := NewRetryStrategy(config)

	successfulExtractor := func(ctx context.Context, input *types.ExtractionInput) (*types.EntityExtractionResult, error) {
		return &types.EntityExtractionResult{
			ID:         "bench-result",
			DocumentID: input.DocumentID,
			Entities:   []*types.ExtractedEntity{},
		}, nil
	}

	extractor := NewRetryableExtractor(strategy, successfulExtractor)

	input := &types.ExtractionInput{
		DocumentID:    "bench-doc",
		DocumentName:  "bench.txt",
		Content:       "benchmark content",
		DocumentType:  types.DocumentTypeOther,
		AnalysisDepth: types.AnalysisDepthBasic,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := extractor.Extract(ctx, input)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
