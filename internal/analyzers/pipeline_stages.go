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

// PipelineStagesAnalyzer implements the Go pipeline pattern for document analysis
// Following GO_CONCURRENCY_PATTERNS.md: Pipeline, Fan-out/Fan-in, Bounded Parallelism
// Phase 6: Enhanced with comprehensive cancellation and timeout management
type PipelineStagesAnalyzer struct {
	// Configuration and dependencies (immutable after creation)
	config             *AnalysisConfig
	entityExtractors   map[types.ExtractorMethod]interfaces.EntityExtractor
	citationExtractors map[types.ExtractorMethod]interfaces.CitationExtractor
	topicExtractors    map[types.ExtractorMethod]interfaces.TopicExtractor
	storageManager     interfaces.StorageManager
	llmClient          interfaces.LLMClient

	// Phase 6: Enhanced cancellation coordination
	cancellationCoordinator *CancellationCoordinator
	timeoutManager          *TimeoutManager

	// Pipeline control (legacy - replaced by cancellation coordinator)
	shutdownChan chan struct{}
	doneChan     chan struct{}

	// Metrics broadcasting
	metricsBroadcaster *MetricsBroadcaster
}

// Pipeline stage data types following the pipeline pattern
type DocumentStage struct {
	Request *AnalysisRequest
	Context context.Context
}

type ExtractionStage struct {
	Document  *types.Document
	Config    *AnalysisConfig
	RequestID string
	Context   context.Context
}

type ExtractionResult struct {
	Type      string // "entity", "citation", "topic"
	RequestID string
	Data      any // []*types.Entity, []*types.Citation, or []*types.Topic
	Method    types.ExtractorMethod
	Error     error
	Duration  time.Duration
}

type AggregationStage struct {
	Request        *AnalysisRequest
	EntityResult   *ExtractionResult
	CitationResult *ExtractionResult
	TopicResult    *ExtractionResult
	Context        context.Context
}

type StorageStage struct {
	Document *types.Document
	Result   *types.AnalysisResult
	Context  context.Context
}

// NewPipelineStagesAnalyzer creates a new pipeline stages analyzer
func NewPipelineStagesAnalyzer(
	ctx context.Context,
	storageManager interfaces.StorageManager,
	llmClient interfaces.LLMClient,
	config *AnalysisConfig,
) *PipelineStagesAnalyzer {
	if config == nil {
		config = DefaultAnalysisConfig()
	}

	// Create cancellation coordinator for system-wide shutdown coordination
	timeoutConfig := DefaultTimeoutConfig()
	cancellationCoordinator := NewCancellationCoordinator(
		ctx,
		timeoutConfig.ShutdownTimeout,
		timeoutConfig.ComponentShutdown,
		timeoutConfig.GracefulShutdown,
	)

	// Create timeout manager
	timeoutManager := NewTimeoutManager(timeoutConfig, cancellationCoordinator)

	analyzer := &PipelineStagesAnalyzer{
		config:             config,
		entityExtractors:   make(map[types.ExtractorMethod]interfaces.EntityExtractor),
		citationExtractors: make(map[types.ExtractorMethod]interfaces.CitationExtractor),
		topicExtractors:    make(map[types.ExtractorMethod]interfaces.TopicExtractor),
		storageManager:     storageManager,
		llmClient:          llmClient,

		// Phase 6: Enhanced cancellation system
		cancellationCoordinator: cancellationCoordinator,
		timeoutManager:          timeoutManager,

		// Legacy channels (maintained for backward compatibility)
		shutdownChan: make(chan struct{}),
		doneChan:     make(chan struct{}),
		// Note: metricsBroadcaster will be set up when needed
		metricsBroadcaster: nil,
	}

	// Register this pipeline as a cancellable component
	cancellationCoordinator.RegisterComponent("pipeline_stages_analyzer", analyzer)

	return analyzer
}

// AnalyzeDocumentPipeline processes a document through pipeline stages
// Phase 6: Enhanced with comprehensive cancellation and timeout management
func (p *PipelineStagesAnalyzer) AnalyzeDocumentPipeline(ctx context.Context, request *AnalysisRequest) (*AnalysisResponse, error) {
	startTime := time.Now()

	// Use the timeout manager to create a proper pipeline context
	pipelineCtx, cancel := p.timeoutManager.CreatePipelineContext(ctx)
	defer cancel()

	// Check if system shutdown has been initiated
	if p.cancellationCoordinator.IsShutdownInitiated() {
		return nil, fmt.Errorf("system shutdown in progress - rejecting new requests")
	}

	// Track this operation with the cancellation coordinator
	operationName := fmt.Sprintf("pipeline-analysis-%s", request.RequestID)
	p.cancellationCoordinator.TrackGoroutine(operationName)
	defer p.cancellationCoordinator.UntrackGoroutine(operationName)

	response := &AnalysisResponse{
		RequestID:     request.RequestID,
		Document:      request.Document,
		Timestamp:     startTime,
		ExtractorUsed: make(map[string]types.ExtractorMethod),
	}

	// Stage 1: Document preparation stage (implicit - just validate and prepare context)

	// Stage 2: Fan-out to extraction stages (parallel processing)
	extractionStage := &ExtractionStage{
		Document:  request.Document,
		Config:    request.CustomConfig,
		RequestID: request.RequestID,
		Context:   pipelineCtx, // Use the managed context
	}

	if extractionStage.Config == nil {
		extractionStage.Config = p.config
	} // Create bounded channels for extraction results
	entityResultChan := make(chan *ExtractionResult, 1)
	citationResultChan := make(chan *ExtractionResult, 1)
	topicResultChan := make(chan *ExtractionResult, 1)

	// Fan-out: Start extraction pipeline stages concurrently
	var extractionWG sync.WaitGroup

	// Entity extraction stage
	if extractionStage.Config.EnableEntityExtraction {
		extractionWG.Add(1)
		go p.entityExtractionStage(extractionStage, entityResultChan, &extractionWG)
	} else {
		// Send empty result if disabled
		go func() {
			entityResultChan <- &ExtractionResult{
				Type:      "entity",
				RequestID: request.RequestID,
				Data:      []*types.Entity{},
				Method:    "",
				Error:     nil,
				Duration:  0,
			}
		}()
	}

	// Citation extraction stage
	if extractionStage.Config.EnableCitationExtraction {
		extractionWG.Add(1)
		go p.citationExtractionStage(extractionStage, citationResultChan, &extractionWG)
	} else {
		// Send empty result if disabled
		go func() {
			citationResultChan <- &ExtractionResult{
				Type:      "citation",
				RequestID: request.RequestID,
				Data:      []*types.Citation{},
				Method:    "",
				Error:     nil,
				Duration:  0,
			}
		}()
	}

	// Topic extraction stage
	if extractionStage.Config.EnableTopicExtraction {
		extractionWG.Add(1)
		go p.topicExtractionStage(extractionStage, topicResultChan, &extractionWG)
	} else {
		// Send empty result if disabled
		go func() {
			topicResultChan <- &ExtractionResult{
				Type:      "topic",
				RequestID: request.RequestID,
				Data:      []*types.Topic{},
				Method:    "",
				Error:     nil,
				Duration:  0,
			}
		}()
	}

	// Fan-in: Collect results from all extraction stages
	var entityResult, citationResult, topicResult *ExtractionResult
	var collectionError error

	// Wait for all extraction results with proper channel coordination
	done := make(chan struct{})
	go func() {
		extractionWG.Wait()
		close(done)
	}()

	// Collect results or handle timeout/cancellation
	resultCount := 0
	expectedResults := 3 // entity, citation, topic

	for resultCount < expectedResults {
		select {
		case result := <-entityResultChan:
			entityResult = result
			if result.Error != nil {
				collectionError = fmt.Errorf("entity extraction failed: %w", result.Error)
			} else {
				response.ExtractorUsed["entities"] = result.Method
			}
			resultCount++

		case result := <-citationResultChan:
			citationResult = result
			if result.Error != nil {
				collectionError = fmt.Errorf("citation extraction failed: %w", result.Error)
			} else {
				response.ExtractorUsed["citations"] = result.Method
			}
			resultCount++

		case result := <-topicResultChan:
			topicResult = result
			if result.Error != nil {
				collectionError = fmt.Errorf("topic extraction failed: %w", result.Error)
			} else {
				response.ExtractorUsed["topics"] = result.Method
			}
			resultCount++

		case <-pipelineCtx.Done():
			response.Error = p.timeoutManager.HandleTimeoutError(pipelineCtx.Err(), "pipeline", time.Since(startTime))
			response.ProcessingTime = time.Since(startTime)
			return response, response.Error

		case <-p.cancellationCoordinator.GetDoneChannel():
			response.Error = fmt.Errorf("system shutdown initiated during pipeline processing")
			response.ProcessingTime = time.Since(startTime)
			return response, response.Error
		}
	}

	// Check for collection errors
	if collectionError != nil {
		response.Error = collectionError
		response.ProcessingTime = time.Since(startTime)
		return response, collectionError
	}

	// Stage 3: Aggregation stage (fan-in results)
	aggregationStage := &AggregationStage{
		Request:        request,
		EntityResult:   entityResult,
		CitationResult: citationResult,
		TopicResult:    topicResult,
		Context:        ctx,
	}

	result, err := p.aggregationStage(aggregationStage)
	if err != nil {
		response.Error = fmt.Errorf("aggregation stage failed: %w", err)
		response.ProcessingTime = time.Since(startTime)
		return response, err
	}

	// Stage 4: Storage stage (if enabled)
	if extractionStage.Config.StoreResults && p.storageManager != nil {
		storageStage := &StorageStage{
			Document: request.Document,
			Result:   result,
			Context:  ctx,
		}

		if err := p.storageStage(storageStage); err != nil {
			slog.ErrorContext(ctx, "storage stage failed", slog.Any("error", err))
			// Don't fail the entire pipeline for storage errors
		}
	}

	// Generate embeddings if enabled (post-processing stage)
	if extractionStage.Config.GenerateEmbeddings && p.llmClient != nil {
		if embedding, err := p.llmClient.Embed(ctx, request.Document.Content); err == nil {
			result.Metadata["embedding"] = embedding
		} else {
			slog.Warn("Failed to generate embedding", "error", err)
		}
	}

	// Complete response
	response.Result = result
	response.ProcessingTime = time.Since(startTime)

	// Broadcast completion metrics (simplified for now)
	slog.InfoContext(ctx, "pipeline analysis complete",
		slog.String("request_id", request.RequestID),
		slog.Duration("processing_time", response.ProcessingTime),
		slog.Int("entities", len(result.Entities)),
		slog.Int("citations", len(result.Citations)),
		slog.Int("topics", len(result.Topics)))

	return response, nil
}

// RegisterEntityExtractor registers an entity extractor
func (p *PipelineStagesAnalyzer) RegisterEntityExtractor(method types.ExtractorMethod, extractor interfaces.EntityExtractor) {
	p.entityExtractors[method] = extractor
}

// RegisterCitationExtractor registers a citation extractor
func (p *PipelineStagesAnalyzer) RegisterCitationExtractor(method types.ExtractorMethod, extractor interfaces.CitationExtractor) {
	p.citationExtractors[method] = extractor
}

// RegisterTopicExtractor registers a topic extractor
func (p *PipelineStagesAnalyzer) RegisterTopicExtractor(method types.ExtractorMethod, extractor interfaces.TopicExtractor) {
	p.topicExtractors[method] = extractor
}

// Shutdown gracefully stops the pipeline (implements CancellableComponent)
func (p *PipelineStagesAnalyzer) Shutdown(ctx context.Context) error {
	// Use the legacy shutdown mechanism for now, but coordinate with cancellation system
	close(p.shutdownChan)

	select {
	case <-p.doneChan:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	}
}

// Name returns the component name (implements CancellableComponent)
func (p *PipelineStagesAnalyzer) Name() string {
	return "pipeline_stages_analyzer"
}

// IsHealthy returns true if the component is functioning properly (implements CancellableComponent)
func (p *PipelineStagesAnalyzer) IsHealthy(ctx context.Context) bool {
	// Check if shutdown has been initiated
	select {
	case <-p.shutdownChan:
		return false
	default:
		// Component is healthy if not shutting down
		return true
	}
}

// entityExtractionStage implements the entity extraction pipeline stage
// Phase 6: Enhanced with timeout management and cancellation coordination
func (p *PipelineStagesAnalyzer) entityExtractionStage(stage *ExtractionStage, resultChan chan<- *ExtractionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// Track this extraction operation
	extractionName := fmt.Sprintf("entity-extraction-%s", stage.RequestID)
	p.cancellationCoordinator.TrackGoroutine(extractionName)
	defer p.cancellationCoordinator.UntrackGoroutine(extractionName)

	startTime := time.Now()
	result := &ExtractionResult{
		Type:      "entity",
		RequestID: stage.RequestID,
		Duration:  0,
	}

	// Create extraction context with timeout
	extractionCtx, cancel := p.timeoutManager.CreateExtractionContext(stage.Context)
	defer cancel()

	// Check for shutdown before proceeding
	select {
	case <-p.cancellationCoordinator.GetDoneChannel():
		result.Error = fmt.Errorf("system shutdown during entity extraction")
		result.Duration = time.Since(startTime)
		resultChan <- result
		return
	default:
		// Continue with extraction
	}

	// Get extractor
	extractor, exists := p.entityExtractors[stage.Config.PreferredEntityMethod]
	if !exists || extractor == nil {
		if stage.Config.FallbackOnFailure {
			// Try to find any available entity extractor
			for method, e := range p.entityExtractors {
				if e != nil {
					extractor = e
					result.Method = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			result.Error = fmt.Errorf("no entity extractor available")
			result.Duration = time.Since(startTime)
			resultChan <- result
			return
		}
	} else {
		result.Method = stage.Config.PreferredEntityMethod
	}

	// Perform extraction
	extractionConfig := &interfaces.EntityExtractionConfig{
		Enabled:       true,
		Method:        stage.Config.PreferredEntityMethod,
		MinConfidence: stage.Config.MinConfidenceScore,
		MaxEntities:   100, // Default limit
	}

	extractionResult, err := extractor.Extract(extractionCtx, stage.Document, extractionConfig)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(startTime)
		resultChan <- result
		return
	}

	result.Data = extractionResult.Entities
	result.Error = extractionResult.Error
	result.Duration = time.Since(startTime)

	resultChan <- result
}

// citationExtractionStage implements the citation extraction pipeline stage
func (p *PipelineStagesAnalyzer) citationExtractionStage(stage *ExtractionStage, resultChan chan<- *ExtractionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	startTime := time.Now()
	result := &ExtractionResult{
		Type:      "citation",
		RequestID: stage.RequestID,
		Duration:  0,
	}

	// Get extractor
	extractor, exists := p.citationExtractors[stage.Config.PreferredCitationMethod]
	if !exists || extractor == nil {
		if stage.Config.FallbackOnFailure {
			// Try to find any available citation extractor
			for method, e := range p.citationExtractors {
				if e != nil {
					extractor = e
					result.Method = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			result.Error = fmt.Errorf("no citation extractor available")
			result.Duration = time.Since(startTime)
			resultChan <- result
			return
		}
	} else {
		result.Method = stage.Config.PreferredCitationMethod
	}

	// Perform extraction
	extractionConfig := &interfaces.CitationExtractionConfig{
		Enabled:       true,
		Method:        stage.Config.PreferredCitationMethod,
		MinConfidence: stage.Config.MinConfidenceScore,
		MaxCitations:  100, // Default limit
	}

	extractionResult, err := extractor.Extract(stage.Context, stage.Document, extractionConfig)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(startTime)
		resultChan <- result
		return
	}

	result.Data = extractionResult.Citations
	result.Error = extractionResult.Error
	result.Duration = time.Since(startTime)

	resultChan <- result
}

// topicExtractionStage implements the topic extraction pipeline stage
func (p *PipelineStagesAnalyzer) topicExtractionStage(stage *ExtractionStage, resultChan chan<- *ExtractionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	startTime := time.Now()
	result := &ExtractionResult{
		Type:      "topic",
		RequestID: stage.RequestID,
		Duration:  0,
	}

	// Get extractor
	extractor, exists := p.topicExtractors[stage.Config.PreferredTopicMethod]
	if !exists || extractor == nil {
		if stage.Config.FallbackOnFailure {
			// Try to find any available topic extractor
			for method, e := range p.topicExtractors {
				if e != nil {
					extractor = e
					result.Method = method
					exists = true
					break
				}
			}
		}

		if !exists || extractor == nil {
			result.Error = fmt.Errorf("no topic extractor available")
			result.Duration = time.Since(startTime)
			resultChan <- result
			return
		}
	} else {
		result.Method = stage.Config.PreferredTopicMethod
	}

	// Perform extraction
	extractionConfig := &interfaces.TopicExtractionConfig{
		Enabled:       true,
		Method:        string(stage.Config.PreferredTopicMethod), // Convert ExtractorMethod to string
		MinConfidence: stage.Config.MinConfidenceScore,
		NumTopics:     10, // Default number of topics
		MaxKeywords:   20, // Default max keywords per topic
	}

	extractionResult, err := extractor.Extract(stage.Context, stage.Document, extractionConfig)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(startTime)
		resultChan <- result
		return
	}

	result.Data = extractionResult.Topics
	result.Error = extractionResult.Error
	result.Duration = time.Since(startTime)

	resultChan <- result
}

// aggregationStage implements the result aggregation pipeline stage (fan-in)
func (p *PipelineStagesAnalyzer) aggregationStage(stage *AggregationStage) (*types.AnalysisResult, error) {
	// Create analysis result
	result := types.NewAnalysisResult(stage.Request.Document.ID)

	// Aggregate entity results
	if stage.EntityResult != nil && stage.EntityResult.Error == nil {
		if entities, ok := stage.EntityResult.Data.([]*types.Entity); ok {
			result.Entities = entities
		}
	}

	// Aggregate citation results
	if stage.CitationResult != nil && stage.CitationResult.Error == nil {
		if citations, ok := stage.CitationResult.Data.([]*types.Citation); ok {
			result.Citations = citations
		}
	}

	// Aggregate topic results
	if stage.TopicResult != nil && stage.TopicResult.Error == nil {
		if topics, ok := stage.TopicResult.Data.([]*types.Topic); ok {
			result.Topics = topics
		}
	}

	return result, nil
}

// storageStage implements the storage pipeline stage
func (p *PipelineStagesAnalyzer) storageStage(stage *StorageStage) error {
	if p.storageManager == nil {
		return fmt.Errorf("storage manager not available")
	}

	// Store the document first
	if err := p.storageManager.StoreDocument(stage.Context, stage.Document); err != nil {
		slog.ErrorContext(stage.Context, "failed to store document", slog.Any("error", err))
		return fmt.Errorf("failed to store document: %w", err)
	}

	// Store individual components
	for _, entity := range stage.Result.Entities {
		if err := p.storageManager.StoreEntity(stage.Context, entity); err != nil {
			slog.ErrorContext(stage.Context, "failed to store entity", slog.Any("error", err))
			return fmt.Errorf("failed to store entity: %w", err)
		}
	}

	for _, citation := range stage.Result.Citations {
		if err := p.storageManager.StoreCitation(stage.Context, citation); err != nil {
			slog.ErrorContext(stage.Context, "failed to store citation", slog.Any("error", err))
			return fmt.Errorf("failed to store citation: %w", err)
		}
	}

	for _, topic := range stage.Result.Topics {
		if err := p.storageManager.StoreTopic(stage.Context, topic); err != nil {
			slog.ErrorContext(stage.Context, "failed to store topic", slog.Any("error", err))
			return fmt.Errorf("failed to store topic: %w", err)
		}
	}

	return nil
}
