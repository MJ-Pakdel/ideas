package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/example/idaes/internal/clients"
	"github.com/example/idaes/internal/extractors"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/storage"
	"github.com/example/idaes/internal/types"
)

// AnalysisSystemFactory creates and configures a complete analysis system
type AnalysisSystemFactory struct {
}

// AnalysisSystemConfig contains configuration for the entire analysis system
type AnalysisSystemConfig struct {
	// LLM configuration
	LLMConfig *interfaces.LLMConfig `json:"llm_config" yaml:"llm_config"`

	// ChromaDB configuration
	ChromaDBURL      string `json:"chromadb_url" yaml:"chromadb_url"`
	ChromaDBTenant   string `json:"chromadb_tenant" yaml:"chromadb_tenant"`
	ChromaDBDatabase string `json:"chromadb_database" yaml:"chromadb_database"`

	// Analysis pipeline configuration
	PipelineConfig *AnalysisConfig `json:"pipeline_config" yaml:"pipeline_config"`

	// Orchestrator configuration
	OrchestratorConfig *OrchestratorConfig `json:"orchestrator_config" yaml:"orchestrator_config"`

	// Extractor configurations
	EntityConfig   *interfaces.EntityExtractionConfig   `json:"entity_config" yaml:"entity_config"`
	CitationConfig *interfaces.CitationExtractionConfig `json:"citation_config" yaml:"citation_config"`
	TopicConfig    *interfaces.TopicExtractionConfig    `json:"topic_config" yaml:"topic_config"`

	// Unified extraction configuration
	UnifiedConfig *interfaces.UnifiedExtractionConfig `json:"unified_config" yaml:"unified_config"`
}

// AnalysisSystem represents a complete configured analysis system
type AnalysisSystem struct {
	LLMClient      interfaces.LLMClient
	StorageManager interfaces.StorageManager
	Pipeline       *AnalysisPipeline
	Orchestrator   *AnalysisOrchestrator
	Extractors     *ExtractorSet
	Config         *AnalysisSystemConfig

	// Phase 6: System-wide graceful shutdown integration
	cancellationManager *SystemCancellationManager
	systemContext       context.Context
	systemCancel        context.CancelFunc
}

// ExtractorSet contains all configured extractors
type ExtractorSet struct {
	EntityExtractors   map[types.ExtractorMethod]interfaces.EntityExtractor
	CitationExtractors map[types.ExtractorMethod]interfaces.CitationExtractor
	TopicExtractors    map[types.ExtractorMethod]interfaces.TopicExtractor

	// Unified extractor for comprehensive analysis in a single LLM call
	UnifiedExtractor interfaces.UnifiedExtractor
}

// NewAnalysisSystemFactory creates a new analysis system factory
func NewAnalysisSystemFactory() *AnalysisSystemFactory {

	return &AnalysisSystemFactory{}
}

// CreateSystem creates a complete analysis system with the given configuration
func (f *AnalysisSystemFactory) CreateSystem(ctx context.Context, config *AnalysisSystemConfig) (*AnalysisSystem, error) {
	if config == nil {
		config = DefaultAnalysisSystemConfig()
	}

	slog.InfoContext(ctx, "Creating analysis system")

	// Create LLM client
	llmClient, err := f.createLLMClient(config.LLMConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// CRITICAL: Fail fast if Ollama is not accessible
	// This prevents the application from running with broken LLM connectivity
	slog.InfoContext(ctx, "Testing Ollama connectivity", "url", config.LLMConfig.BaseURL, "model", config.LLMConfig.Model, "embedding_model", config.LLMConfig.EmbeddingModel)
	if err := llmClient.Health(ctx); err != nil {
		return nil, fmt.Errorf("CRITICAL: Ollama is not accessible - failing fast to prevent broken application state: %w", err)
	}
	slog.InfoContext(ctx, "Ollama connectivity confirmed successfully")

	// Create storage manager
	storageManager, err := f.createStorageManager(config.ChromaDBURL, config.ChromaDBTenant, config.ChromaDBDatabase, llmClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage manager: %w", err)
	}

	// CRITICAL: Fail fast if ChromaDB is not accessible
	// This prevents the application from running with broken storage
	slog.InfoContext(ctx, "Testing ChromaDB connectivity", "url", config.ChromaDBURL, "tenant", config.ChromaDBTenant, "database", config.ChromaDBDatabase)
	if err := storageManager.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("CRITICAL: ChromaDB is not accessible - failing fast to prevent broken application state: %w", err)
	}
	slog.InfoContext(ctx, "ChromaDB connectivity confirmed successfully")

	// Create extractors
	extractors, err := f.createExtractors(ctx, llmClient, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create extractors: %w", err)
	}

	// Create analysis pipeline
	pipeline := NewAnalysisPipeline(ctx, storageManager, llmClient, config.PipelineConfig)
	pipeline.RegisterExtractors(
		extractors.EntityExtractors,
		extractors.CitationExtractors,
		extractors.TopicExtractors,
	)

	// Register unified extractor if available
	if extractors.UnifiedExtractor != nil {
		pipeline.RegisterUnifiedExtractor(extractors.UnifiedExtractor)
		slog.InfoContext(ctx, "Unified extractor registered with main pipeline")
	} else {
		slog.InfoContext(ctx, "No unified extractor available - using individual extractors only")
	}

	// Create orchestrator
	orchestrator := NewAnalysisOrchestrator(config.OrchestratorConfig)

	// CRITICAL FIX: Create separate pipeline instances for each worker to prevent shared resultChan issues
	// Each worker needs its own pipeline with its own resultChan to avoid race conditions
	numWorkers := config.OrchestratorConfig.MaxWorkers
	slog.InfoContext(ctx, "Creating separate pipeline instances for each worker",
		"num_workers", numWorkers,
		"total_pipelines", numWorkers)

	for i := 0; i < numWorkers; i++ {
		// Create a separate pipeline instance for each worker
		workerPipeline := NewAnalysisPipeline(ctx, storageManager, llmClient, config.PipelineConfig)
		workerPipeline.RegisterExtractors(
			extractors.EntityExtractors,
			extractors.CitationExtractors,
			extractors.TopicExtractors,
		)
		orchestrator.AddPipeline(workerPipeline)
		slog.InfoContext(ctx, "Added dedicated pipeline for worker",
			"worker_index", i,
			"pipeline_ptr", fmt.Sprintf("%p", workerPipeline))
	}

	// Phase 6: Initialize system-wide graceful shutdown
	cancellationManager := NewSystemCancellationManager(ctx)
	systemContext, systemCancel := context.WithCancel(ctx)

	system := &AnalysisSystem{
		LLMClient:      llmClient,
		StorageManager: storageManager,
		Pipeline:       pipeline,
		Orchestrator:   orchestrator,
		Extractors:     extractors,
		Config:         config,

		// Phase 6: Graceful shutdown integration
		cancellationManager: cancellationManager,
		systemContext:       systemContext,
		systemCancel:        systemCancel,
	}

	slog.InfoContext(ctx, "Analysis system created successfully")
	return system, nil
}

// componentWrapper adapts existing components to the CancellableComponent interface
type componentWrapper struct {
	name      string
	component interface {
		Stop(context.Context) error
	}
}

func (cw *componentWrapper) Shutdown(ctx context.Context) error {
	return cw.component.Stop(ctx)
}

func (cw *componentWrapper) Name() string {
	return cw.name
}

func (cw *componentWrapper) IsHealthy(ctx context.Context) bool {
	// Basic health check - component exists and is accessible
	return cw.component != nil
}

// pipelineWrapper adapts AnalysisPipeline to CancellableComponent interface
type pipelineWrapper struct {
	name     string
	pipeline *AnalysisPipeline
}

func (pw *pipelineWrapper) Shutdown(ctx context.Context) error {
	return pw.pipeline.Shutdown(ctx)
}

func (pw *pipelineWrapper) Name() string {
	return pw.name
}

func (pw *pipelineWrapper) IsHealthy(ctx context.Context) bool {
	return pw.pipeline != nil
}

// createDefaultPipelineConfig creates a pipeline config with environment variable support
func createDefaultPipelineConfig() *AnalysisConfig {
	config := DefaultAnalysisConfig()

	// Allow environment variable override for unified extraction
	config.UseUnifiedExtraction = getBoolFromEnv("IDAES_USE_UNIFIED_EXTRACTION", config.UseUnifiedExtraction)
	config.UnifiedFallbackEnabled = getBoolFromEnv("IDAES_UNIFIED_FALLBACK_ENABLED", config.UnifiedFallbackEnabled)

	return config
}

// getBoolFromEnv gets a boolean value from an environment variable with a default fallback
func getBoolFromEnv(envVar string, defaultValue bool) bool {
	if value := os.Getenv(envVar); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// DefaultAnalysisSystemConfig returns a default configuration for the analysis system
func DefaultAnalysisSystemConfig() *AnalysisSystemConfig {
	return &AnalysisSystemConfig{
		LLMConfig: &interfaces.LLMConfig{
			Provider:       "ollama",
			BaseURL:        "http://localhost:11434",
			Model:          "llama3.2:3b",
			EmbeddingModel: "nomic-embed-text",
			Temperature:    0.1,
			MaxTokens:      4096,
			Timeout:        60 * time.Second,
			RetryAttempts:  3,
		},
		ChromaDBURL:        "http://localhost:8000",
		ChromaDBTenant:     "default_tenant",
		ChromaDBDatabase:   "idaes",
		PipelineConfig:     createDefaultPipelineConfig(), // Use env-aware pipeline config
		OrchestratorConfig: DefaultOrchestratorConfig(),
		EntityConfig: &interfaces.EntityExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM, // IDAES uses intelligent LLM-only approach
			MinConfidence: 0.7,
			MaxEntities:   100,
			EntityTypes:   []types.EntityType{}, // empty means all types
		},
		CitationConfig: &interfaces.CitationExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM, // IDAES uses intelligent LLM-only entity extraction
			MinConfidence: 0.8,
			MaxCitations:  50,
			Formats:       []types.CitationFormat{}, // empty means all formats
		},
		TopicConfig: &interfaces.TopicExtractionConfig{
			Enabled:       true,
			Method:        "regex", // Current implementation uses regex - should be upgraded to LLM
			NumTopics:     10,
			MinConfidence: 0.6,
			MaxKeywords:   20,
		},
		UnifiedConfig: &interfaces.UnifiedExtractionConfig{
			Temperature:         0.1,
			MaxTokens:           4096,
			MinConfidence:       0.7,
			MaxEntities:         100,
			EntityTypes:         []types.EntityType{}, // empty means all types
			MaxCitations:        50,
			CitationFormats:     []types.CitationFormat{}, // empty means all formats
			MaxTopics:           10,
			TopicMinKeywords:    3,
			ClassificationTypes: []types.DocumentType{}, // empty means all types
			IncludeMetadata:     true,
			AnalysisDepth:       "comprehensive",
			CacheEnabled:        true,
			CacheTTL:            30 * time.Minute,
		},
	}
}

// createLLMClient creates and configures an LLM client
func (f *AnalysisSystemFactory) createLLMClient(config *interfaces.LLMConfig) (interfaces.LLMClient, error) {
	if config == nil {
		return nil, fmt.Errorf("LLM configuration is required")
	}

	switch config.Provider {
	case "ollama":
		client := clients.NewOllamaClient(
			config.BaseURL,
			config.Model,
			config.EmbeddingModel,
			config.Timeout,
			config.RetryAttempts,
		)
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}

// createStorageManager creates and configures a storage manager
func (f *AnalysisSystemFactory) createStorageManager(chromaDBURL, chromaDBTenant, database string, llmClient interfaces.LLMClient) (interfaces.StorageManager, error) {
	chromaClient := clients.NewChromaDBClient(chromaDBURL, 30*time.Second, 3, chromaDBTenant, database)
	storageManager := storage.NewChromaStorageManager(
		chromaClient,
		llmClient,
		"documents", // defaultCollection
		"entities",  // entityCollection
		"citations", // citationCollection
		"topics",    // topicCollection
		768,         // embeddingDimension (common for text embeddings)
		3,           // maxRetries
		time.Second, // retryDelay
	)
	return storageManager, nil
}

// createExtractors creates all configured extractors
func (f *AnalysisSystemFactory) createExtractors(ctx context.Context, llmClient interfaces.LLMClient, config *AnalysisSystemConfig) (*ExtractorSet, error) {
	// Create extractor factory
	factory := extractors.NewExtractorFactory()

	// Create entity extractors - IDAES requires LLM for intelligent analysis
	entityExtractors := make(map[types.ExtractorMethod]interfaces.EntityExtractor)

	// CRITICAL: LLM entity extractor is required for IDAES intelligent analysis
	llmEntityExtractor, err := factory.CreateEntityExtractor(ctx, types.ExtractorMethodLLM, config.EntityConfig, llmClient)
	if err != nil {
		return nil, fmt.Errorf("CRITICAL: LLM entity extractor is required for IDAES intelligent analysis - failing fast: %w", err)
	}
	entityExtractors[types.ExtractorMethodLLM] = llmEntityExtractor

	slog.InfoContext(ctx, "Created intelligent entity extractor",
		"llm_extractor", "enabled",
		"note", "Regex and hybrid extractors removed - IDAES requires intelligent analysis only")

	// Create citation extractors - Regex is appropriate for citations due to standardized formats
	citationExtractors := make(map[types.ExtractorMethod]interfaces.CitationExtractor)

	// Use LLM for intelligent citation extraction
	llmCitationExtractor, err := factory.CreateCitationExtractor(ctx, types.ExtractorMethodLLM, config.CitationConfig, llmClient)
	if err != nil {
		// Citation extraction failure is not critical - log warning and continue
		slog.Warn("Failed to create LLM citation extractor - citation analysis will be disabled", "error", err)
	} else {
		citationExtractors[types.ExtractorMethodLLM] = llmCitationExtractor
	}

	// Create topic extractors - Use LLM for intelligent topic extraction
	topicExtractors := make(map[types.ExtractorMethod]interfaces.TopicExtractor)

	llmTopicExtractor, err := factory.CreateTopicExtractor(ctx, "llm", config.TopicConfig, llmClient)
	if err != nil {
		// Topic extraction failure is not critical - log warning and continue
		slog.Warn("Failed to create LLM topic extractor - topic analysis will be disabled", "error", err)
	} else {
		topicExtractors[types.ExtractorMethodLLM] = llmTopicExtractor
	}

	// Create unified extractor for comprehensive analysis in a single LLM call
	var unifiedExtractor interfaces.UnifiedExtractor
	if config.UnifiedConfig != nil {
		unifiedExtractor, err = factory.CreateUnifiedExtractor(ctx, config.UnifiedConfig, llmClient)
		if err != nil {
			// Unified extraction failure is not critical for backward compatibility - log warning
			slog.Warn("Failed to create unified extractor - unified analysis will be disabled", "error", err)
		} else {
			slog.InfoContext(ctx, "Created unified LLM extractor",
				"unified_extractor", "enabled",
				"note", "Unified extraction enables comprehensive analysis in a single LLM call")
		}
	} else {
		slog.InfoContext(ctx, "Unified extraction config not provided - unified analysis disabled")
	}

	extractorSet := &ExtractorSet{
		EntityExtractors:   entityExtractors,
		CitationExtractors: citationExtractors,
		TopicExtractors:    topicExtractors,
		UnifiedExtractor:   unifiedExtractor,
	}
	slog.InfoContext(ctx, "createExtractors",
		slog.Int("entity_extractors", len(extractorSet.EntityExtractors)),
		slog.Int("citation_extractors", len(extractorSet.CitationExtractors)),
		slog.Int("topic_extractors", len(extractorSet.TopicExtractors)),
		slog.Bool("unified_extractor", extractorSet.UnifiedExtractor != nil))

	return extractorSet, nil
}

// Start starts the analysis system
func (s *AnalysisSystem) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting analysis system with Phase 6 graceful shutdown integration")

	// Integrate components with cancellation manager for coordinated shutdown
	if err := s.cancellationManager.IntegrateOrchestrator(&componentWrapper{
		name:      "orchestrator",
		component: s.Orchestrator,
	}); err != nil {
		return fmt.Errorf("failed to integrate orchestrator with cancellation manager: %w", err)
	}

	if err := s.cancellationManager.IntegratePipelineStages(&pipelineWrapper{
		name:     "pipeline",
		pipeline: s.Pipeline,
	}); err != nil {
		return fmt.Errorf("failed to integrate pipeline with cancellation manager: %w", err)
	}

	if err := s.cancellationManager.IntegrateStorageManager(s.StorageManager); err != nil {
		return fmt.Errorf("failed to integrate storage manager with cancellation manager: %w", err)
	}

	// Start orchestrator with system context
	if err := s.Orchestrator.Start(s.systemContext); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	slog.InfoContext(ctx, "Analysis system started successfully with graceful shutdown")
	return nil
}

// Stop gracefully stops the analysis system using Phase 6 patterns
func (s *AnalysisSystem) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Initiating graceful shutdown of IDAES Analysis System")

	// Use the cancellation manager to coordinate system shutdown
	if err := s.cancellationManager.InitiateGracefulShutdown(ctx); err != nil {
		slog.ErrorContext(ctx, "Error during graceful shutdown initiation", slog.Any("error", err))
	}

	// Wait for graceful shutdown completion with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.cancellationManager.WaitForShutdownCompletion(shutdownCtx); err != nil {
		slog.WarnContext(ctx, "Shutdown timeout exceeded, forcing shutdown", slog.Any("error", err))

		// Force shutdown if graceful shutdown takes too long
		if forceErr := s.cancellationManager.ForceShutdown(ctx); forceErr != nil {
			slog.ErrorContext(ctx, "Force shutdown failed", slog.Any("error", forceErr))
		}
	}

	// Cancel system context
	s.systemCancel()

	// Close remaining components manually as fallback
	s.closeRemainingComponents(ctx)

	// Get final system health status
	healthStatus := s.cancellationManager.GetSystemHealth(ctx)
	slog.InfoContext(ctx, "IDAES Analysis System shutdown complete",
		slog.Bool("graceful_shutdown", healthStatus.Overall),
		slog.Any("component_status", healthStatus.Components),
		slog.Any("active_goroutines", healthStatus.ActiveGoroutines))

	return nil
}

// closeRemainingComponents provides fallback cleanup for components
func (s *AnalysisSystem) closeRemainingComponents(ctx context.Context) {
	// Close LLM client
	if err := s.LLMClient.Close(); err != nil {
		slog.WarnContext(ctx, "Error closing LLM client", "error", err)
	}

	// Close storage manager
	if err := s.StorageManager.Close(); err != nil {
		slog.WarnContext(ctx, "Error closing storage manager", "error", err)
	}

	// Close extractors
	s.closeExtractors(ctx)
}

// closeExtractors closes all extractors
func (s *AnalysisSystem) closeExtractors(ctx context.Context) {
	for method, extractor := range s.Extractors.EntityExtractors {
		if err := extractor.Close(); err != nil {
			slog.WarnContext(ctx, "error closing entity extractors", "method", method, "error", err)
		}
	}

	for method, extractor := range s.Extractors.CitationExtractors {
		if err := extractor.Close(); err != nil {
			slog.WarnContext(ctx, "error closing citation extractors", "method", method, "error", err)
		}
	}

	for method, extractor := range s.Extractors.TopicExtractors {
		if err := extractor.Close(); err != nil {
			slog.WarnContext(ctx, "error closing topic extractor", "method", method, "error", err)

		}
	}
}

// AnalyzeDocument is a convenience method to analyze a document through the system
func (s *AnalysisSystem) AnalyzeDocument(ctx context.Context, document *types.Document) (*AnalysisResponse, error) {
	// Generate request ID based on document ID for backward compatibility
	requestID := fmt.Sprintf("doc_%s", document.ID)
	return s.AnalyzeDocumentWithRequestID(ctx, document, requestID)
}

// AnalyzeDocumentWithRequestID analyzes a document with a specific request ID (preserves original request ID)
func (s *AnalysisSystem) AnalyzeDocumentWithRequestID(ctx context.Context, document *types.Document, requestID string) (*AnalysisResponse, error) {
	request := &AnalysisRequest{
		Document:  document,
		RequestID: requestID,
		Priority:  0,
	}

	slog.InfoContext(ctx, "System submitting request to orchestrator", "request_id", request.RequestID, "document_id", document.ID)
	// Submit to orchestrator
	if err := s.Orchestrator.SubmitRequest(request); err != nil {
		return nil, fmt.Errorf("failed to submit request: %w", err)
	}

	slog.InfoContext(ctx, "System waiting for response from orchestrator", "request_id", request.RequestID)
	// Get response - now no conflict since we disabled the response handler
	response, ok := s.Orchestrator.GetResponse()
	if !ok {
		slog.ErrorContext(ctx, "System failed to get response from orchestrator", "request_id", request.RequestID)
		return nil, fmt.Errorf("failed to get response")
	}

	if response == nil {
		slog.ErrorContext(ctx, "System received nil response from orchestrator", "request_id", request.RequestID)
		return nil, fmt.Errorf("received nil response from orchestrator")
	}

	slog.InfoContext(ctx, "System received response from orchestrator", "request_id", request.RequestID, "response_id", response.RequestID, "has_error", response.Error != nil)
	return response, nil
}

// GetSystemStatus returns the current status of the analysis system
func (s *AnalysisSystem) GetSystemStatus() map[string]any {
	orchestratorMetrics := s.Orchestrator.GetMetrics()
	workerStatus := s.Orchestrator.GetWorkerStatus()
	pipelineMetrics := s.Pipeline.GetMetrics()

	return map[string]any{
		"orchestrator": map[string]any{
			"metrics":       orchestratorMetrics,
			"worker_status": workerStatus,
		},
		"pipeline": map[string]any{
			"metrics": pipelineMetrics,
		},
		"extractors": map[string]any{
			"entity_count":   len(s.Extractors.EntityExtractors),
			"citation_count": len(s.Extractors.CitationExtractors),
			"topic_count":    len(s.Extractors.TopicExtractors),
		},
	}
}

// GetSystemContext returns the system-wide cancellation context
func (s *AnalysisSystem) GetSystemContext() context.Context {
	return s.systemContext
}

// IsShutdownInitiated returns true if system shutdown has been initiated
func (s *AnalysisSystem) IsShutdownInitiated() bool {
	return s.cancellationManager.IsShutdownInitiated()
}
