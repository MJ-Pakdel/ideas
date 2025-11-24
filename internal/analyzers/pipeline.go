package analyzers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// pipelineDocument represents a document flowing through the pipeline stages
type pipelineDocument struct {
	request    *AnalysisRequest
	response   *AnalysisResponse
	result     *types.AnalysisResult
	stage      pipelineStage
	startTime  time.Time
	stageStart time.Time
	errors     []error
}

// pipelineStage represents the current stage of document processing
type pipelineStage int

const (
	stageDocument pipelineStage = iota
	stageExtraction
	stageProcessing
	stageStorage
	stageComplete
)

func (s pipelineStage) String() string {
	switch s {
	case stageDocument:
		return "document"
	case stageExtraction:
		return "extraction"
	case stageProcessing:
		return "processing"
	case stageStorage:
		return "storage"
	case stageComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// AnalysisPipeline orchestrates document analysis using channel-based pipeline stages (Phase 5 pattern)
type AnalysisPipeline struct {
	entityExtractors   map[types.ExtractorMethod]interfaces.EntityExtractor
	citationExtractors map[types.ExtractorMethod]interfaces.CitationExtractor
	topicExtractors    map[types.ExtractorMethod]interfaces.TopicExtractor
	storageManager     interfaces.StorageManager
	llmClient          interfaces.LLMClient
	config             *AnalysisConfig
	metrics            *PipelineMetrics

	// Phase 2: Unified LLM extraction support
	unifiedExtractor interfaces.UnifiedExtractor
	analysisAdapter  interfaces.UnifiedAnalysisAdapter

	// Channel-based pipeline stages (Phase 5 pattern)
	documentChan   chan *pipelineDocument
	extractionChan chan *pipelineDocument
	processingChan chan *pipelineDocument
	storageChan    chan *pipelineDocument
	resultChan     chan *AnalysisResponse

	// Stage coordination
	stageDoneChan chan struct{}
	stageWG       sync.WaitGroup // Only for pipeline shutdown coordination

	// Bounded parallelism controls
	extractionWorkers int
	processingWorkers int
	storageWorkers    int
}

// AnalysisConfig configures the analysis pipeline behavior
type AnalysisConfig struct {
	// Strategy selection
	PreferredEntityMethod   types.ExtractorMethod
	PreferredCitationMethod types.ExtractorMethod
	PreferredTopicMethod    types.ExtractorMethod

	// Phase 2: Unified extraction settings
	UseUnifiedExtraction   bool
	UnifiedFallbackEnabled bool

	// Performance settings
	MaxConcurrentAnalyses int
	AnalysisTimeout       time.Duration
	RetryAttempts         int
	RetryDelay            time.Duration

	// Quality thresholds
	MinConfidenceScore float64
	RequireValidation  bool
	FallbackOnFailure  bool

	// Processing options
	EnableEntityExtraction   bool
	EnableCitationExtraction bool
	EnableTopicExtraction    bool
	EnableSemanticAnalysis   bool
	StoreResults             bool
	GenerateEmbeddings       bool
}

// PipelineMetrics tracks analysis performance and usage statistics
type PipelineMetrics struct {
	TotalAnalyses         int64
	SuccessfulAnalyses    int64
	FailedAnalyses        int64
	AverageProcessingTime time.Duration
	ExtractorUsageCount   map[types.ExtractorMethod]int64
	ErrorCounts           map[string]int64
	mu                    sync.RWMutex
}

// AnalysisRequest represents a request for document analysis
type AnalysisRequest struct {
	Document       *types.Document
	RequestID      string
	Priority       int
	CustomConfig   *AnalysisConfig
	ResultCallback func(*AnalysisResponse)
}

// AnalysisResponse contains the results of document analysis
type AnalysisResponse struct {
	RequestID      string
	Document       *types.Document
	Result         *types.AnalysisResult
	ProcessingTime time.Duration
	ExtractorUsed  map[string]types.ExtractorMethod
	Error          error
	Timestamp      time.Time
}

// NewAnalysisPipeline creates a new analysis pipeline with channel-based stages (Phase 5 pattern)
func NewAnalysisPipeline(
	ctx context.Context,
	storageManager interfaces.StorageManager,
	llmClient interfaces.LLMClient,
	config *AnalysisConfig,
) *AnalysisPipeline {
	if config == nil {
		config = DefaultAnalysisConfig()
	}

	// Calculate worker counts based on config
	extractionWorkers := min(config.MaxConcurrentAnalyses, 5) // Bounded parallelism
	processingWorkers := min(config.MaxConcurrentAnalyses/2, 3)
	storageWorkers := min(config.MaxConcurrentAnalyses/3, 2)

	pipeline := &AnalysisPipeline{
		entityExtractors:   make(map[types.ExtractorMethod]interfaces.EntityExtractor),
		citationExtractors: make(map[types.ExtractorMethod]interfaces.CitationExtractor),
		topicExtractors:    make(map[types.ExtractorMethod]interfaces.TopicExtractor),
		storageManager:     storageManager,
		llmClient:          llmClient,
		config:             config,
		metrics:            NewPipelineMetrics(),

		// Phase 2: Initialize unified extraction support
		unifiedExtractor: nil,                                    // Will be set via RegisterUnifiedExtractor
		analysisAdapter:  interfaces.NewUnifiedAnalysisAdapter(), // Default adapter for backward compatibility

		// Channel-based pipeline stages with bounded buffers
		documentChan:   make(chan *pipelineDocument, 10),
		extractionChan: make(chan *pipelineDocument, 20),
		processingChan: make(chan *pipelineDocument, 15),
		storageChan:    make(chan *pipelineDocument, 10),
		resultChan:     make(chan *AnalysisResponse, 30),

		// Stage coordination
		stageDoneChan: make(chan struct{}),

		// Worker counts for bounded parallelism
		extractionWorkers: extractionWorkers,
		processingWorkers: processingWorkers,
		storageWorkers:    storageWorkers,
	}

	// Start pipeline stages
	pipeline.startPipelineStages(ctx)

	slog.InfoContext(ctx, "Created new AnalysisPipeline instance",
		"pipeline_ptr", fmt.Sprintf("%p", pipeline),
		"resultChan_ptr", fmt.Sprintf("%p", pipeline.resultChan),
		"extraction_workers", extractionWorkers,
		"processing_workers", processingWorkers,
		"storage_workers", storageWorkers)

	return pipeline
}

// startPipelineStages initializes and starts all pipeline stage workers (Phase 5 pattern)
func (p *AnalysisPipeline) startPipelineStages(ctx context.Context) {
	// Start extraction stage workers (fan-out pattern)
	for i := 0; i < p.extractionWorkers; i++ {
		p.stageWG.Add(1)
		go func(workerID int) {
			defer p.stageWG.Done()
			p.runExtractionStage(ctx, workerID)
		}(i)
	}

	// Start processing stage workers
	for i := 0; i < p.processingWorkers; i++ {
		p.stageWG.Add(1)
		go func(workerID int) {
			defer p.stageWG.Done()
			p.runProcessingStage(ctx, workerID)
		}(i)
	}

	// Start storage stage workers
	for i := 0; i < p.storageWorkers; i++ {
		p.stageWG.Add(1)
		go func(workerID int) {
			defer p.stageWG.Done()
			p.runStorageStage(ctx, workerID)
		}(i)
	}

	// NOTE: Removed runResultCollector() as it was consuming results meant for AnalyzeDocument()
	// The storage stage already logs completion when sending results to resultChan
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DefaultAnalysisConfig returns a default analysis configuration
func DefaultAnalysisConfig() *AnalysisConfig {
	return &AnalysisConfig{
		PreferredEntityMethod:   types.ExtractorMethodLLM,
		PreferredCitationMethod: types.ExtractorMethodLLM, // LLM for all extraction types
		PreferredTopicMethod:    "llm",                    // LLM for all extraction types

		// Phase 2: Unified extraction defaults - disabled by default for backward compatibility
		UseUnifiedExtraction:   false, // Enable manually for unified mode
		UnifiedFallbackEnabled: true,  // Allow fallback to individual extractors if unified fails

		MaxConcurrentAnalyses:    10,
		AnalysisTimeout:          5 * time.Minute,
		RetryAttempts:            3,
		RetryDelay:               time.Second,
		MinConfidenceScore:       0.7,
		RequireValidation:        true,
		FallbackOnFailure:        true,
		EnableEntityExtraction:   true,
		EnableCitationExtraction: true,
		EnableTopicExtraction:    true,
		EnableSemanticAnalysis:   true,
		StoreResults:             true,
		GenerateEmbeddings:       true,
	}
}

// NewPipelineMetrics creates a new metrics tracker
func NewPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		ExtractorUsageCount: make(map[types.ExtractorMethod]int64),
		ErrorCounts:         make(map[string]int64),
	}
}

// RegisterExtractors registers all available extractors with the pipeline (channel-safe)
func (p *AnalysisPipeline) RegisterExtractors(
	entityExtractors map[types.ExtractorMethod]interfaces.EntityExtractor,
	citationExtractors map[types.ExtractorMethod]interfaces.CitationExtractor,
	topicExtractors map[types.ExtractorMethod]interfaces.TopicExtractor,
) {
	// Note: Registration should happen before pipeline starts, so no synchronization needed
	// In a production system, this would be during initialization phase

	for method, extractor := range entityExtractors {
		p.entityExtractors[method] = extractor
	}

	for method, extractor := range citationExtractors {
		p.citationExtractors[method] = extractor
	}

	for method, extractor := range topicExtractors {
		p.topicExtractors[method] = extractor
	}
	slog.Info("Registered extractors with analysis pipeline",
		"entity_extractors", len(p.entityExtractors),
		"citation_extractors", len(p.citationExtractors),
		"topic_extractors", len(p.topicExtractors))
}

// RegisterUnifiedExtractor registers a unified extractor for comprehensive document analysis
func (p *AnalysisPipeline) RegisterUnifiedExtractor(extractor interfaces.UnifiedExtractor) {
	p.unifiedExtractor = extractor

	// Don't auto-enable to avoid breaking existing tests and explicit configuration
	// Users need to explicitly enable via environment variable or configuration
	if extractor != nil && !p.config.UseUnifiedExtraction {
		slog.Info("Unified extractor registered but not enabled",
			"unified_extraction_enabled", p.config.UseUnifiedExtraction,
			"message", "Set IDAES_USE_UNIFIED_EXTRACTION=true to enable unified extraction")
	}

	slog.Info("Registered unified extractor with analysis pipeline",
		"extractor_type", fmt.Sprintf("%T", extractor),
		"unified_extraction_enabled", p.config.UseUnifiedExtraction)
}

// GetConfig returns a copy of the current pipeline configuration
func (p *AnalysisPipeline) GetConfig() *AnalysisConfig {
	if p.config == nil {
		return nil
	}

	// Return a copy to prevent external modification
	configCopy := *p.config
	return &configCopy
}

// UpdateConfig updates the pipeline configuration
func (p *AnalysisPipeline) UpdateConfig(newConfig *AnalysisConfig) error {
	if newConfig == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Make a copy to prevent external modification
	configCopy := *newConfig
	p.config = &configCopy

	slog.Info("Pipeline configuration updated",
		"use_unified_extraction", p.config.UseUnifiedExtraction,
		"fallback_enabled", p.config.UnifiedFallbackEnabled,
		"timeout", p.config.AnalysisTimeout)

	return nil
}

// HasUnifiedExtractor returns true if a unified extractor is registered
func (p *AnalysisPipeline) HasUnifiedExtractor() bool {
	return p.unifiedExtractor != nil
}

// AnalyzeDocument performs comprehensive analysis using channel-based pipeline stages (Phase 5 pattern)
func (p *AnalysisPipeline) AnalyzeDocument(ctx context.Context, request *AnalysisRequest) *AnalysisResponse {
	startTime := time.Now()

	slog.InfoContext(ctx, "Pipeline AnalyzeDocument called",
		"request_id", request.RequestID,
		"pipeline_ptr", fmt.Sprintf("%p", p),
		"resultChan_ptr", fmt.Sprintf("%p", p.resultChan))

	// Create pipeline document
	pipeDoc := &pipelineDocument{
		request: request,
		response: &AnalysisResponse{
			RequestID:     request.RequestID,
			Document:      request.Document,
			Timestamp:     startTime,
			ExtractorUsed: make(map[string]types.ExtractorMethod),
		},
		result:     types.NewAnalysisResult(request.Document.ID),
		stage:      stageDocument,
		startTime:  startTime,
		stageStart: startTime,
		errors:     make([]error, 0),
	}

	// Use custom config if provided, otherwise use pipeline default
	if request.CustomConfig != nil {
		// For pipeline pattern, we'll store config in the pipeline document
		// Since the stages will need access to it
	}

	// Submit to pipeline
	select {
	case p.documentChan <- pipeDoc:
		// Document submitted to pipeline
	case <-ctx.Done():
		return &AnalysisResponse{
			RequestID:      request.RequestID,
			Document:       request.Document,
			Error:          fmt.Errorf("pipeline submission timeout: %w", ctx.Err()),
			Timestamp:      startTime,
			ProcessingTime: time.Since(startTime),
		}
	}

	// Wait for result from pipeline
	slog.InfoContext(ctx, "Pipeline AnalyzeDocument starting to wait for result", "request_id", request.RequestID, "resultChan_ptr", fmt.Sprintf("%p", p.resultChan))
	select {
	case response := <-p.resultChan:
		slog.InfoContext(ctx, "Pipeline received result from resultChan",
			"request_id", response.RequestID,
			"expected_id", request.RequestID,
			"resultChan_ptr", fmt.Sprintf("%p", p.resultChan))
		if response.RequestID == request.RequestID {
			slog.InfoContext(ctx, "Pipeline AnalyzeDocument returning success", "request_id", request.RequestID, "processing_time", time.Since(startTime))
			return response
		}
		// If we get a different response, this is a problem with our pipeline
		slog.ErrorContext(ctx, "Pipeline received wrong response ID", "got_id", response.RequestID, "expected_id", request.RequestID)
		return &AnalysisResponse{
			RequestID:      request.RequestID,
			Document:       request.Document,
			Error:          fmt.Errorf("pipeline returned wrong response"),
			Timestamp:      startTime,
			ProcessingTime: time.Since(startTime),
		}
	case <-ctx.Done():
		slog.WarnContext(ctx, "Pipeline AnalyzeDocument timeout due to context done", "request_id", request.RequestID, "timeout", time.Since(startTime), "ctx_err", ctx.Err())
		return &AnalysisResponse{
			RequestID:      request.RequestID,
			Document:       request.Document,
			Error:          fmt.Errorf("pipeline processing timeout: %w", ctx.Err()),
			Timestamp:      startTime,
			ProcessingTime: time.Since(startTime),
		}
	}
}

// runExtractionStage processes documents through the extraction stage (Phase 5 pipeline pattern)
func (p *AnalysisPipeline) runExtractionStage(ctx context.Context, workerID int) {
	slog.DebugContext(ctx, "starting runExtractionStage", slog.Int("worker_id", workerID))
	defer slog.DebugContext(ctx, "ending runExtractionStage", slog.Int("worker_id", workerID))

	for {
		select {
		case <-p.stageDoneChan:
			return
		case pipeDoc := <-p.documentChan:
			if pipeDoc == nil {
				return // Channel closed
			}

			// Update stage
			pipeDoc.stage = stageExtraction
			pipeDoc.stageStart = time.Now()

			// Get config (use custom if provided, otherwise pipeline default)
			config := p.config
			if pipeDoc.request.CustomConfig != nil {
				config = pipeDoc.request.CustomConfig
			}

			// Set timeout context
			if config.AnalysisTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, config.AnalysisTimeout)
				defer cancel()
			}

			// Perform extractions (fan-out pattern within this stage)
			var extractionErrors []error

			// Phase 2: Try unified extraction first if enabled and available
			if config.UseUnifiedExtraction && p.unifiedExtractor != nil {
				slog.InfoContext(ctx, "Using unified LLM extraction",
					"document_id", pipeDoc.request.Document.ID,
					"fallback_enabled", config.UnifiedFallbackEnabled)

				unifiedResult, err := p.extractUnified(ctx, pipeDoc.request.Document, config)
				if err != nil {
					extractionErrors = append(extractionErrors, fmt.Errorf("unified extraction failed: %w", err))

					// If unified extraction fails and fallback is disabled, skip individual extractions
					if !config.UnifiedFallbackEnabled {
						slog.WarnContext(ctx, "Unified extraction failed and fallback disabled", "error", err)
						pipeDoc.errors = append(pipeDoc.errors, extractionErrors...)
						// Send to processing stage with empty results
						select {
						case p.processingChan <- pipeDoc:
						case <-p.stageDoneChan:
							return
						}
						continue
					}

					slog.InfoContext(ctx, "Unified extraction failed, falling back to individual extractors", "error", err)
				} else {
					// Unified extraction succeeded - convert results using adapter
					if config.EnableEntityExtraction && len(unifiedResult.Entities) > 0 {
						pipeDoc.result.Entities = unifiedResult.Entities
						pipeDoc.response.ExtractorUsed["entities"] = types.ExtractorMethodLLM
					}

					if config.EnableCitationExtraction && len(unifiedResult.Citations) > 0 {
						pipeDoc.result.Citations = unifiedResult.Citations
						pipeDoc.response.ExtractorUsed["citations"] = types.ExtractorMethodLLM
					}

					if config.EnableTopicExtraction && len(unifiedResult.Topics) > 0 {
						pipeDoc.result.Topics = unifiedResult.Topics
						pipeDoc.response.ExtractorUsed["topics"] = types.ExtractorMethodLLM
					}

					slog.InfoContext(ctx, "Unified extraction completed successfully",
						"entities", len(unifiedResult.Entities),
						"citations", len(unifiedResult.Citations),
						"topics", len(unifiedResult.Topics))

					// Record unified extractor usage for metrics
					p.recordExtractorUsage(types.ExtractorMethodLLM)

					// Skip individual extractions - unified extraction succeeded
					// Send to processing stage
					select {
					case p.processingChan <- pipeDoc:
					case <-p.stageDoneChan:
						return
					}
					continue
				}
			}

			// Individual extractions (fallback or when unified is disabled)
			// Entity extraction
			if config.EnableEntityExtraction {
				entities, method, err := p.extractEntities(ctx, pipeDoc.request.Document, config)
				if err != nil {
					extractionErrors = append(extractionErrors, fmt.Errorf("entity extraction failed: %w", err))
				} else {
					pipeDoc.result.Entities = entities
					pipeDoc.response.ExtractorUsed["entities"] = method
				}
			}

			// Citation extraction
			if config.EnableCitationExtraction {
				citations, method, err := p.extractCitations(ctx, pipeDoc.request.Document, config)
				if err != nil {
					extractionErrors = append(extractionErrors, fmt.Errorf("citation extraction failed: %w", err))
				} else {
					pipeDoc.result.Citations = citations
					pipeDoc.response.ExtractorUsed["citations"] = method
				}
			}

			// Topic extraction
			if config.EnableTopicExtraction {
				topics, method, err := p.extractTopics(ctx, pipeDoc.request.Document, config)
				if err != nil {
					extractionErrors = append(extractionErrors, fmt.Errorf("topic extraction failed: %w", err))
				} else {
					pipeDoc.result.Topics = topics
					pipeDoc.response.ExtractorUsed["topics"] = method
				}
			}

			// Handle extraction errors
			if len(extractionErrors) > 0 {
				pipeDoc.errors = append(pipeDoc.errors, extractionErrors...)
				// Still continue to next stage for partial results
			}

			// Send to processing stage
			select {
			case p.processingChan <- pipeDoc:
			case <-p.stageDoneChan:
				return
			}
		}
	}
}

// runProcessingStage handles document processing (embeddings, semantic analysis)
func (p *AnalysisPipeline) runProcessingStage(ctx context.Context, workerID int) {
	slog.DebugContext(ctx, "starting runProcessingStage", slog.Int("worker_id", workerID))
	defer slog.DebugContext(ctx, "ending runProcessingStage", slog.Int("worker_id", workerID))

	for {
		select {
		case <-p.stageDoneChan:
			return
		case pipeDoc := <-p.processingChan:
			if pipeDoc == nil {
				return // Channel closed
			}

			// Update stage
			pipeDoc.stage = stageProcessing
			pipeDoc.stageStart = time.Now()

			// Get config
			config := p.config
			if pipeDoc.request.CustomConfig != nil {
				config = pipeDoc.request.CustomConfig
			}

			if config.AnalysisTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, config.AnalysisTimeout)
				defer cancel()
			}

			// Generate embeddings if enabled
			if config.GenerateEmbeddings && p.llmClient != nil {
				if embedding, err := p.llmClient.Embed(ctx, pipeDoc.request.Document.Content); err == nil {
					// Store embedding in metadata
					pipeDoc.result.Metadata["embedding"] = embedding
				} else {
					slog.Warn("Failed to generate embedding", "error", err)
					pipeDoc.errors = append(pipeDoc.errors, fmt.Errorf("embedding generation failed: %w", err))
				}
			}

			// Semantic analysis could be added here
			if config.EnableSemanticAnalysis {
				// Placeholder for semantic analysis
				// This could involve additional LLM calls, entity linking, etc.
			}

			// Send to storage stage
			select {
			case p.storageChan <- pipeDoc:
			case <-p.stageDoneChan:
				return
			}
		}
	}
}

// runStorageStage handles result storage
func (p *AnalysisPipeline) runStorageStage(ctx context.Context, workerID int) {
	slog.DebugContext(ctx, "starting runStorageStage", slog.Int("worker_id", workerID))
	defer slog.DebugContext(ctx, "ending runStorageStage", slog.Int("worker_id", workerID))
	for {
		select {
		case <-p.stageDoneChan:
			return
		case pipeDoc := <-p.storageChan:
			if pipeDoc == nil {
				return // Channel closed
			}

			// Update stage
			pipeDoc.stage = stageStorage
			pipeDoc.stageStart = time.Now()

			// Get config
			config := p.config
			if pipeDoc.request.CustomConfig != nil {
				config = pipeDoc.request.CustomConfig
			}

			if config.AnalysisTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, config.AnalysisTimeout)
				defer cancel()
			}

			// Store results if enabled
			if config.StoreResults && p.storageManager != nil {
				// Store the document first
				if err := p.storageManager.StoreDocument(ctx, pipeDoc.request.Document); err != nil {
					slog.ErrorContext(ctx, "failed to store document", slog.Any("error", err))
					pipeDoc.errors = append(pipeDoc.errors, fmt.Errorf("document storage failed: %w", err))
				}

				// Store individual components
				for _, entity := range pipeDoc.result.Entities {
					if err := p.storageManager.StoreEntity(ctx, entity); err != nil {
						slog.ErrorContext(ctx, "failed to store entity", slog.Any("error", err))
						pipeDoc.errors = append(pipeDoc.errors, fmt.Errorf("entity storage failed: %w", err))
					}
				}
				for _, citation := range pipeDoc.result.Citations {
					if err := p.storageManager.StoreCitation(ctx, citation); err != nil {
						slog.ErrorContext(ctx, "failed to store citation", slog.Any("error", err))
						pipeDoc.errors = append(pipeDoc.errors, fmt.Errorf("citation storage failed: %w", err))
					}
				}
				for _, topic := range pipeDoc.result.Topics {
					if err := p.storageManager.StoreTopic(ctx, topic); err != nil {
						slog.ErrorContext(ctx, "failed to store topic", slog.Any("error", err))
						pipeDoc.errors = append(pipeDoc.errors, fmt.Errorf("topic storage failed: %w", err))
					}
				}
			}

			// Finalize response
			pipeDoc.stage = stageComplete
			pipeDoc.response.Result = pipeDoc.result
			pipeDoc.response.ProcessingTime = time.Since(pipeDoc.startTime)

			// Handle errors
			if len(pipeDoc.errors) > 0 {
				// Combine all errors
				errorMsg := "processing errors: "
				for i, err := range pipeDoc.errors {
					if i > 0 {
						errorMsg += "; "
					}
					errorMsg += err.Error()
				}
				pipeDoc.response.Error = fmt.Errorf("%s", errorMsg)
				p.recordError("pipeline_processing")
				p.metrics.FailedAnalyses++
			} else {
				p.metrics.SuccessfulAnalyses++
				p.updateProcessingTime(pipeDoc.response.ProcessingTime)
			}

			// Send result
			select {
			case p.resultChan <- pipeDoc.response:
				slog.InfoContext(ctx, "Pipeline result sent to resultChan",
					"request_id", pipeDoc.response.RequestID,
					"processing_time", pipeDoc.response.ProcessingTime,
					"resultChan_ptr", fmt.Sprintf("%p", p.resultChan),
					"resultChan_len", len(p.resultChan),
					"resultChan_cap", cap(p.resultChan))

				// Log pipeline completion (moved from runResultCollector)
				if pipeDoc.response.Error == nil {
					slog.Info("pipeline analysis complete",
						"request_id", pipeDoc.response.RequestID,
						"document_id", pipeDoc.response.Document.ID,
						"processing_time", pipeDoc.response.ProcessingTime,
						"entities", len(pipeDoc.response.Result.Entities),
						"citations", len(pipeDoc.response.Result.Citations),
						"topics", len(pipeDoc.response.Result.Topics))
				} else {
					slog.Error("pipeline analysis failed",
						"request_id", pipeDoc.response.RequestID,
						"document_id", pipeDoc.response.Document.ID,
						"processing_time", pipeDoc.response.ProcessingTime,
						"error", pipeDoc.response.Error.Error())
				}
			case <-p.stageDoneChan:
				slog.WarnContext(ctx, "Pipeline stage done while sending result", "request_id", pipeDoc.response.RequestID)
				return
			}
		}
	}
}

// runResultCollector collects and logs pipeline results (fan-in pattern)
func (p *AnalysisPipeline) runResultCollector() {
	for {
		select {
		case <-p.stageDoneChan:
			return
		case response := <-p.resultChan:
			if response == nil {
				return // Channel closed
			}

			// Log completion
			if response.Error == nil {
				slog.Info("pipeline analysis complete",
					slog.String("request_id", response.RequestID),
					slog.String("document_id", response.Document.ID),
					slog.Duration("processing_time", response.ProcessingTime),
					slog.Int("entities", len(response.Result.Entities)),
					slog.Int("citations", len(response.Result.Citations)),
					slog.Int("topics", len(response.Result.Topics)))
			} else {
				slog.Error("pipeline analysis failed",
					slog.String("request_id", response.RequestID),
					slog.String("document_id", response.Document.ID),
					slog.Duration("processing_time", response.ProcessingTime),
					slog.String("error", response.Error.Error()))
			}

			// Results are already in the channel for the caller to receive
		}
	}
}

// extractUnified performs unified document analysis using a single LLM call
func (p *AnalysisPipeline) extractUnified(ctx context.Context, doc *types.Document, config *AnalysisConfig) (*interfaces.UnifiedExtractionResult, error) {
	if p.unifiedExtractor == nil {
		return nil, fmt.Errorf("unified extractor not available")
	}

	// Use default unified extraction config for now
	// TODO: Make this configurable through AnalysisConfig
	unifiedConfig := &interfaces.UnifiedExtractionConfig{
		Temperature:     0.1,
		MaxTokens:       4000,
		MinConfidence:   config.MinConfidenceScore,
		MaxEntities:     20,
		MaxCitations:    10,
		MaxTopics:       8,
		AnalysisDepth:   "detailed",
		IncludeMetadata: true,
	}

	// Perform unified extraction
	result, err := p.unifiedExtractor.ExtractAll(ctx, doc, unifiedConfig)
	if err != nil {
		return nil, fmt.Errorf("unified extraction failed: %w", err)
	}

	return result, nil
}

// extractEntities performs entity extraction using the configured strategy (channel-safe)
func (p *AnalysisPipeline) extractEntities(ctx context.Context, doc *types.Document, config *AnalysisConfig) ([]*types.Entity, types.ExtractorMethod, error) {
	extractor, exists := p.entityExtractors[config.PreferredEntityMethod]

	if !exists || extractor == nil {
		if config.FallbackOnFailure {
			// Try to find any available entity extractor
			for method, e := range p.entityExtractors {
				if e != nil {
					extractor = e
					config.PreferredEntityMethod = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			return nil, "", fmt.Errorf("no entity extractor available")
		}
	}

	// Create extraction config
	extractionConfig := &interfaces.EntityExtractionConfig{
		Enabled:       true,
		Method:        config.PreferredEntityMethod,
		MinConfidence: config.MinConfidenceScore,
		MaxEntities:   1000,                 // reasonable default
		EntityTypes:   []types.EntityType{}, // empty means all types
	}

	result, err := extractor.Extract(ctx, doc, extractionConfig)
	if err != nil {
		p.recordError(fmt.Sprintf("entity_extraction_%s", config.PreferredEntityMethod))
		return nil, config.PreferredEntityMethod, err
	}

	p.recordExtractorUsage(config.PreferredEntityMethod)
	return result.Entities, config.PreferredEntityMethod, nil
}

// extractCitations performs citation extraction using the configured strategy (channel-safe)
func (p *AnalysisPipeline) extractCitations(ctx context.Context, doc *types.Document, config *AnalysisConfig) ([]*types.Citation, types.ExtractorMethod, error) {
	extractor, exists := p.citationExtractors[config.PreferredCitationMethod]

	if !exists || extractor == nil {
		if config.FallbackOnFailure {
			// Try to find any available citation extractor
			for method, e := range p.citationExtractors {
				if e != nil {
					extractor = e
					config.PreferredCitationMethod = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			return nil, "", fmt.Errorf("no citation extractor available")
		}
	}

	// Create extraction config
	extractionConfig := &interfaces.CitationExtractionConfig{
		Enabled:       true,
		Method:        config.PreferredCitationMethod,
		MinConfidence: config.MinConfidenceScore,
		MaxCitations:  1000,                     // reasonable default
		Formats:       []types.CitationFormat{}, // empty means all formats
	}

	result, err := extractor.Extract(ctx, doc, extractionConfig)
	if err != nil {
		p.recordError(fmt.Sprintf("citation_extraction_%s", config.PreferredCitationMethod))
		return nil, config.PreferredCitationMethod, err
	}

	p.recordExtractorUsage(config.PreferredCitationMethod)
	return result.Citations, config.PreferredCitationMethod, nil
}

// extractTopics performs topic extraction using the configured strategy (channel-safe)
func (p *AnalysisPipeline) extractTopics(ctx context.Context, doc *types.Document, config *AnalysisConfig) ([]*types.Topic, types.ExtractorMethod, error) {
	extractor, exists := p.topicExtractors[config.PreferredTopicMethod]

	if !exists || extractor == nil {
		if config.FallbackOnFailure {
			// Try to find any available topic extractor
			for method, e := range p.topicExtractors {
				if e != nil {
					extractor = e
					config.PreferredTopicMethod = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			return nil, "", fmt.Errorf("no topic extractor available")
		}
	}

	// Create extraction config
	extractionConfig := &interfaces.TopicExtractionConfig{
		Enabled:       true,
		Method:        string(config.PreferredTopicMethod),
		NumTopics:     10, // reasonable default
		MinConfidence: config.MinConfidenceScore,
		MaxKeywords:   20, // reasonable default
	}

	result, err := extractor.Extract(ctx, doc, extractionConfig)
	if err != nil {
		p.recordError(fmt.Sprintf("topic_extraction_%s", config.PreferredTopicMethod))
		return nil, config.PreferredTopicMethod, err
	}

	p.recordExtractorUsage(config.PreferredTopicMethod)
	return result.Topics, config.PreferredTopicMethod, nil
}

// GetMetrics returns current pipeline metrics
func (p *AnalysisPipeline) GetMetrics() *PipelineMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &PipelineMetrics{
		TotalAnalyses:         p.metrics.TotalAnalyses,
		SuccessfulAnalyses:    p.metrics.SuccessfulAnalyses,
		FailedAnalyses:        p.metrics.FailedAnalyses,
		AverageProcessingTime: p.metrics.AverageProcessingTime,
		ExtractorUsageCount:   make(map[types.ExtractorMethod]int64),
		ErrorCounts:           make(map[string]int64),
	}

	for k, v := range p.metrics.ExtractorUsageCount {
		metrics.ExtractorUsageCount[k] = v
	}

	for k, v := range p.metrics.ErrorCounts {
		metrics.ErrorCounts[k] = v
	}

	return metrics
}

// recordExtractorUsage tracks extractor usage statistics
func (p *AnalysisPipeline) recordExtractorUsage(method types.ExtractorMethod) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.ExtractorUsageCount[method]++
	p.metrics.TotalAnalyses++
}

// recordError tracks error statistics
func (p *AnalysisPipeline) recordError(errorType string) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	p.metrics.ErrorCounts[errorType]++
}

// updateProcessingTime updates the average processing time
func (p *AnalysisPipeline) updateProcessingTime(duration time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	if p.metrics.TotalAnalyses == 1 {
		p.metrics.AverageProcessingTime = duration
	} else {
		// Calculate running average
		total := p.metrics.AverageProcessingTime * time.Duration(p.metrics.TotalAnalyses-1)
		p.metrics.AverageProcessingTime = (total + duration) / time.Duration(p.metrics.TotalAnalyses)
	}
}

// Shutdown gracefully shuts down the analysis pipeline
func (p *AnalysisPipeline) Shutdown(ctx context.Context) error {
	slog.InfoContext(ctx, "Shutting down analysis pipeline")

	// Log final metrics
	metrics := p.GetMetrics()
	slog.InfoContext(ctx, "Final pipeline metrics", slog.Int64("total_analyses", metrics.TotalAnalyses), slog.Int64("successful_analyses", metrics.SuccessfulAnalyses), slog.Int64("failed_analyses", metrics.FailedAnalyses), slog.Duration("average_processing_time", metrics.AverageProcessingTime))

	return nil
}
