package extractors

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// ExtractorFactory implements the factory pattern for creating different types of extractors
type ExtractorFactory struct {
}

// NewExtractorFactory creates a new extractor factory
func NewExtractorFactory() *ExtractorFactory {
	return &ExtractorFactory{}
}

// CreateEntityExtractor creates an entity extractor based on the specified method
func (f *ExtractorFactory) CreateEntityExtractor(ctx context.Context, method types.ExtractorMethod, config *interfaces.EntityExtractionConfig, llmClient interfaces.LLMClient) (interfaces.EntityExtractor, error) {
	slog.InfoContext(ctx, "Creating entity extractor", "method", method)

	switch method {
	case types.ExtractorMethodLLM:
		if llmClient == nil {
			return nil, fmt.Errorf("LLM client is required for LLM entity extraction")
		}
		return NewLLMEntityExtractor(config, llmClient), nil
	default:
		return nil, fmt.Errorf("unsupported entity extraction method: %s", method)
	}
}

// CreateCitationExtractor creates a citation extractor based on the specified method
func (f *ExtractorFactory) CreateCitationExtractor(ctx context.Context, method types.ExtractorMethod, config *interfaces.CitationExtractionConfig, llmClient interfaces.LLMClient) (interfaces.CitationExtractor, error) {
	slog.InfoContext(ctx, "Creating citation extractor", "method", method)

	switch method {
	case types.ExtractorMethodLLM:
		return NewLLMCitationExtractor(config, llmClient), nil
	default:
		return nil, fmt.Errorf("unsupported citation extraction method: %s", method)
	}
}

// CreateTopicExtractor creates a topic extractor based on the specified method
func (f *ExtractorFactory) CreateTopicExtractor(ctx context.Context, method string, config *interfaces.TopicExtractionConfig, llmClient interfaces.LLMClient) (interfaces.TopicExtractor, error) {
	slog.InfoContext(ctx, "Creating topic extractor", "method", method)

	switch method {
	case "llm":
		return NewLLMTopicExtractor(config, llmClient), nil
	default:
		return nil, fmt.Errorf("unsupported topic extraction method: %s", method)
	}
}

// CreateUnifiedExtractor creates a unified extractor for comprehensive analysis
func (f *ExtractorFactory) CreateUnifiedExtractor(ctx context.Context, config *interfaces.UnifiedExtractionConfig, llmClient interfaces.LLMClient) (interfaces.UnifiedExtractor, error) {
	slog.InfoContext(ctx, "Creating unified extractor")

	if llmClient == nil {
		return nil, fmt.Errorf("LLM client is required for unified extraction")
	}

	return NewLLMUnifiedExtractor(config, llmClient), nil
}

// Close cleans up all managed extractors (future implementation)
func (f *ExtractorFactory) Close(ctx context.Context) error {
	slog.InfoContext(ctx, "Closing extractor factory")
	return nil
}
