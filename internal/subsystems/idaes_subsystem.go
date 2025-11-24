package subsystems

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/clients"
	"github.com/example/idaes/internal/types"
)

// AnalysisRequest represents a request for document analysis
type AnalysisRequest struct {
	RequestID string           `json:"request_id"`
	Document  *types.Document  `json:"document"`
	Options   *AnalysisOptions `json:"options,omitempty"`
	Callback  ResponseCallback `json:"-"` // Not serialized - for internal use
}

// AnalysisResponse represents the result of document analysis
type AnalysisResponse struct {
	RequestID      string                `json:"request_id"`
	Result         *types.AnalysisResult `json:"result,omitempty"`
	Error          error                 `json:"error,omitempty"`
	ProcessingTime time.Duration         `json:"processing_time"`
	ExtractorUsed  map[string]any        `json:"extractor_used,omitempty"`
	Status         AnalysisStatus        `json:"status"`
}

// AnalysisOptions contains optional configuration for analysis
type AnalysisOptions struct {
	EnableEntities  bool          `json:"enable_entities"`
	EnableCitations bool          `json:"enable_citations"`
	EnableTopics    bool          `json:"enable_topics"`
	Timeout         time.Duration `json:"timeout,omitempty"`
}

// AnalysisStatus represents the status of an analysis request
type AnalysisStatus string

const (
	StatusPending    AnalysisStatus = "pending"
	StatusProcessing AnalysisStatus = "processing"
	StatusCompleted  AnalysisStatus = "completed"
	StatusFailed     AnalysisStatus = "failed"
	StatusCancelled  AnalysisStatus = "cancelled"
)

// ResponseCallback is a function type for handling analysis responses
type ResponseCallback func(response *AnalysisResponse)

// HealthStatus represents the health status of the IDAES subsystem
type HealthStatus struct {
	Status                string            `json:"status"`
	Timestamp             time.Time         `json:"timestamp"`
	RequestsProcessed     int64             `json:"requests_processed"`
	RequestsInProgress    int               `json:"requests_in_progress"`
	AverageProcessingTime time.Duration     `json:"average_processing_time"`
	LastError             string            `json:"last_error,omitempty"`
	ComponentHealth       map[string]string `json:"component_health"`
}

// IDaesSubsystem represents the intelligent document analysis engine as a standalone subsystem
type IDaesSubsystem struct {
	// Channel-based communication
	requestChan  chan *AnalysisRequest
	responseChan chan *AnalysisResponse
	healthChan   chan chan *HealthStatus
	shutdownChan chan struct{}

	// Core analysis system (exposed for adapter access)
	AnalysisSystem *analyzers.AnalysisSystem

	// Subscribers and callbacks
	subscribers   map[string]ResponseCallback
	subscribersMu sync.RWMutex

	// State tracking
	requestsProcessed  int64
	requestsInProgress int
	processingTimes    []time.Duration
	lastError          string
	stateMu            sync.RWMutex

	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// IDaesConfig contains configuration for the IDAES subsystem
type IDaesConfig struct {
	AnalysisConfig    *analyzers.AnalysisSystemConfig `json:"analysis_config"`
	RequestBuffer     int                             `json:"request_buffer"`
	ResponseBuffer    int                             `json:"response_buffer"`
	WorkerCount       int                             `json:"worker_count"`
	RequestTimeout    time.Duration                   `json:"request_timeout"`
	ShutdownTimeout   time.Duration                   `json:"shutdown_timeout"`
	ChromaURL         string                          `json:"chroma_url"`
	ChromaTenant      string                          `json:"chroma_tenant"`
	ChromaDatabase    string                          `json:"chroma_database"`
	OllamaURL         string                          `json:"ollama_url"`
	IntelligenceModel string                          `json:"intelligence_model"`
	EmbeddingModel    string                          `json:"embedding_model"`
	LLMTimeout        time.Duration                   `json:"llm_timeout"`
	LLMRetryAttempts  int                             `json:"llm_retry_attempts"`
}

// NewIDaesSubsystem creates a new IDAES subsystem
func NewIDaesSubsystem(ctx context.Context, config *IDaesConfig) (*IDaesSubsystem, error) {
	if config == nil {
		config = DefaultIDaesConfig()
	}

	// Update analysis config with service URLs if provided
	if config.AnalysisConfig == nil {
		config.AnalysisConfig = analyzers.DefaultAnalysisSystemConfig()
	}

	// Override URLs and models if provided in config
	if config.ChromaURL != "" {
		config.AnalysisConfig.ChromaDBURL = config.ChromaURL
	}
	if config.ChromaTenant != "" {
		config.AnalysisConfig.ChromaDBTenant = config.ChromaTenant
	}
	if config.ChromaDatabase != "" {
		config.AnalysisConfig.ChromaDBDatabase = config.ChromaDatabase
	}
	if config.OllamaURL != "" {
		config.AnalysisConfig.LLMConfig.BaseURL = config.OllamaURL
	}
	if config.IntelligenceModel != "" {
		config.AnalysisConfig.LLMConfig.Model = config.IntelligenceModel
	}
	if config.EmbeddingModel != "" {
		config.AnalysisConfig.LLMConfig.EmbeddingModel = config.EmbeddingModel
	}
	if config.LLMTimeout > 0 {
		config.AnalysisConfig.LLMConfig.Timeout = config.LLMTimeout
	}
	if config.LLMRetryAttempts > 0 {
		config.AnalysisConfig.LLMConfig.RetryAttempts = config.LLMRetryAttempts
	}

	// CRITICAL FIX: Override orchestrator configuration with IDaes subsystem settings
	if config.WorkerCount > 0 {
		config.AnalysisConfig.OrchestratorConfig.MaxWorkers = config.WorkerCount
		slog.InfoContext(ctx, "Overriding orchestrator MaxWorkers",
			"max_workers", config.WorkerCount,
			"idaes_worker_count", config.WorkerCount)
	}

	// Create analysis system
	factory := analyzers.NewAnalysisSystemFactory()
	analysisSystem, err := factory.CreateSystem(ctx, config.AnalysisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create analysis system: %w", err)
	}

	// Initialize meta collections on startup
	if err := initializeMetaCollections(ctx, config.AnalysisConfig); err != nil {
		slog.WarnContext(ctx, "Failed to initialize meta collections, will create on demand", "error", err)
	}

	// Create subsystem context
	subsystemCtx, cancel := context.WithCancel(ctx)

	subsystem := &IDaesSubsystem{
		requestChan:     make(chan *AnalysisRequest, config.RequestBuffer),
		responseChan:    make(chan *AnalysisResponse, config.ResponseBuffer),
		healthChan:      make(chan chan *HealthStatus),
		shutdownChan:    make(chan struct{}),
		AnalysisSystem:  analysisSystem,
		subscribers:     make(map[string]ResponseCallback),
		processingTimes: make([]time.Duration, 0, 100), // Keep last 100 times
		ctx:             subsystemCtx,
		cancel:          cancel,
	}

	return subsystem, nil
}

// Start begins the IDAES subsystem processing
func (i *IDaesSubsystem) Start() error {
	slog.InfoContext(i.ctx, "Starting IDAES subsystem")

	// Start the analysis system first
	if err := i.AnalysisSystem.Start(i.ctx); err != nil {
		return fmt.Errorf("failed to start analysis system: %w", err)
	}

	// Start request processor
	i.wg.Add(1)
	go i.requestProcessor()

	// Start response broadcaster
	i.wg.Add(1)
	go i.responseBroadcaster()

	// Start health monitor
	i.wg.Add(1)
	go i.healthMonitor()

	slog.InfoContext(i.ctx, "IDAES subsystem started")
	return nil
}

// Stop gracefully shuts down the IDAES subsystem
func (i *IDaesSubsystem) Stop() error {
	slog.InfoContext(i.ctx, "Stopping IDAES subsystem")

	// Signal shutdown
	close(i.shutdownChan)

	// Cancel context
	i.cancel()

	// Wait for all goroutines to finish
	i.wg.Wait()

	// Stop analysis system
	if i.AnalysisSystem != nil {
		if err := i.AnalysisSystem.Stop(i.ctx); err != nil {
			slog.WarnContext(i.ctx, "Analysis system stop warning", "error", err)
		} else {
			slog.InfoContext(i.ctx, "Analysis system stopped successfully")
		}
	}

	slog.InfoContext(i.ctx, "IDAES subsystem stopped")
	return nil
}

// SubmitRequest submits an analysis request to the IDAES subsystem
func (i *IDaesSubsystem) SubmitRequest(request *AnalysisRequest) error {
	select {
	case i.requestChan <- request:
		return nil
	case <-i.ctx.Done():
		return fmt.Errorf("subsystem is shutting down")
	default:
		return fmt.Errorf("request buffer full")
	}
}

// Subscribe registers a callback function to receive analysis responses
func (i *IDaesSubsystem) Subscribe(subscriberID string, callback ResponseCallback) {
	i.subscribersMu.Lock()
	defer i.subscribersMu.Unlock()
	i.subscribers[subscriberID] = callback
	slog.InfoContext(i.ctx, "Subscriber registered", "subscriber_id", subscriberID)
}

// Unsubscribe removes a subscriber
func (i *IDaesSubsystem) Unsubscribe(subscriberID string) {
	i.subscribersMu.Lock()
	defer i.subscribersMu.Unlock()
	delete(i.subscribers, subscriberID)
	slog.InfoContext(i.ctx, "Subscriber unregistered", "subscriber_id", subscriberID)
}

// GetHealth returns the current health status of the IDAES subsystem
func (i *IDaesSubsystem) GetHealth() *HealthStatus {
	healthResponseChan := make(chan *HealthStatus, 1)

	select {
	case i.healthChan <- healthResponseChan:
		return <-healthResponseChan
	case <-i.ctx.Done():
		return &HealthStatus{
			Status:    "shutting_down",
			Timestamp: time.Now(),
		}
	case <-time.After(time.Second):
		return &HealthStatus{
			Status:    "timeout",
			Timestamp: time.Now(),
		}
	}
}

// requestProcessor processes incoming analysis requests
func (i *IDaesSubsystem) requestProcessor() {
	defer i.wg.Done()

	for {
		select {
		case request := <-i.requestChan:
			i.processRequest(request)
		case <-i.shutdownChan:
			slog.InfoContext(i.ctx, "Request processor shutting down")
			return
		case <-i.ctx.Done():
			slog.InfoContext(i.ctx, "Request processor context cancelled")
			return
		}
	}
}

// processRequest processes a single analysis request
func (i *IDaesSubsystem) processRequest(request *AnalysisRequest) {
	startTime := time.Now()
	slog.InfoContext(i.ctx, "Processing analysis request", "request_id", request.RequestID)

	// Update state
	i.stateMu.Lock()
	i.requestsInProgress++
	i.stateMu.Unlock()

	// Create response
	response := &AnalysisResponse{
		RequestID: request.RequestID,
		Status:    StatusProcessing,
	}

	// Process the document with a timeout that accommodates LLM processing
	// Use a longer timeout since LLM operations can take 30+ seconds
	analysisCtx, cancel := context.WithTimeout(i.ctx, 3*time.Minute)
	defer cancel()

	slog.InfoContext(i.ctx, "Starting document analysis", "request_id", request.RequestID)
	analysisResult, err := i.AnalysisSystem.AnalyzeDocumentWithRequestID(analysisCtx, request.Document, request.RequestID)
	processingTime := time.Since(startTime)
	slog.InfoContext(i.ctx, "Document analysis completed", "request_id", request.RequestID, "duration", processingTime, "error", err)

	// Update response
	if err != nil {
		response.Error = err
		response.Status = StatusFailed
		i.stateMu.Lock()
		i.lastError = err.Error()
		i.stateMu.Unlock()
		slog.ErrorContext(i.ctx, "Analysis failed", "request_id", request.RequestID, "error", err)
	} else if analysisResult == nil {
		response.Error = fmt.Errorf("analysis service returned nil result")
		response.Status = StatusFailed
		i.stateMu.Lock()
		i.lastError = "analysis service returned nil result"
		i.stateMu.Unlock()
		slog.ErrorContext(i.ctx, "Analysis returned nil result", "request_id", request.RequestID)
	} else {
		response.Result = analysisResult.Result
		// Convert ExtractorUsed to interface{} map
		extractorUsed := make(map[string]any)
		for k, v := range analysisResult.ExtractorUsed {
			extractorUsed[k] = v
		}
		response.ExtractorUsed = extractorUsed
		response.Status = StatusCompleted
		slog.InfoContext(i.ctx, "Analysis completed", "request_id", request.RequestID, "processing_time", processingTime)
	}

	response.ProcessingTime = processingTime

	// Update state
	i.stateMu.Lock()
	i.requestsInProgress--
	i.requestsProcessed++
	i.processingTimes = append(i.processingTimes, processingTime)
	if len(i.processingTimes) > 100 {
		i.processingTimes = i.processingTimes[1:] // Keep only last 100
	}
	i.stateMu.Unlock()

	// Send response
	slog.InfoContext(i.ctx, "Sending analysis response", "request_id", request.RequestID, "status", response.Status)
	select {
	case i.responseChan <- response:
		slog.InfoContext(i.ctx, "Analysis response sent successfully", "request_id", request.RequestID, "status", response.Status)
	case <-i.ctx.Done():
		slog.WarnContext(i.ctx, "Context cancelled while sending response", "request_id", request.RequestID)
	}
}

// responseBroadcaster broadcasts responses to all subscribers
func (i *IDaesSubsystem) responseBroadcaster() {
	defer i.wg.Done()

	for {
		select {
		case response := <-i.responseChan:
			i.broadcastResponse(response)
		case <-i.shutdownChan:
			slog.InfoContext(i.ctx, "Response broadcaster shutting down")
			return
		case <-i.ctx.Done():
			slog.InfoContext(i.ctx, "Response broadcaster context cancelled")
			return
		}
	}
}

// broadcastResponse sends a response to all subscribers
func (i *IDaesSubsystem) broadcastResponse(response *AnalysisResponse) {
	i.subscribersMu.RLock()
	defer i.subscribersMu.RUnlock()

	for subscriberID, callback := range i.subscribers {
		go func(id string, cb ResponseCallback) {
			defer func() {
				if r := recover(); r != nil {
					slog.ErrorContext(i.ctx, "Subscriber callback panicked", "subscriber_id", id, "panic", r)
				}
			}()
			cb(response)
		}(subscriberID, callback)
	}
}

// healthMonitor handles health status requests
func (i *IDaesSubsystem) healthMonitor() {
	defer i.wg.Done()

	for {
		select {
		case responseChan := <-i.healthChan:
			health := i.generateHealthStatus()
			select {
			case responseChan <- health:
			case <-i.ctx.Done():
				return
			}
		case <-i.shutdownChan:
			slog.InfoContext(i.ctx, "Health monitor shutting down")
			return
		case <-i.ctx.Done():
			slog.InfoContext(i.ctx, "Health monitor context cancelled")
			return
		}
	}
}

// generateHealthStatus creates a current health status
func (i *IDaesSubsystem) generateHealthStatus() *HealthStatus {
	i.stateMu.RLock()
	defer i.stateMu.RUnlock()

	status := "healthy"
	if i.lastError != "" {
		status = "degraded"
	}

	var avgProcessingTime time.Duration
	if len(i.processingTimes) > 0 {
		total := time.Duration(0)
		for _, pt := range i.processingTimes {
			total += pt
		}
		avgProcessingTime = total / time.Duration(len(i.processingTimes))
	}

	// Check component health
	componentHealth := make(map[string]string)
	if i.AnalysisSystem != nil {
		// TODO: Add actual health checks for analysis system components
		componentHealth["analysis_system"] = "healthy"
		componentHealth["storage_manager"] = "healthy"
		componentHealth["llm_client"] = "healthy"
	}

	return &HealthStatus{
		Status:                status,
		Timestamp:             time.Now(),
		RequestsProcessed:     i.requestsProcessed,
		RequestsInProgress:    i.requestsInProgress,
		AverageProcessingTime: avgProcessingTime,
		LastError:             i.lastError,
		ComponentHealth:       componentHealth,
	}
}

// DefaultIDaesConfig returns a default configuration for the IDAES subsystem
func DefaultIDaesConfig() *IDaesConfig {
	return &IDaesConfig{
		AnalysisConfig:    analyzers.DefaultAnalysisSystemConfig(),
		RequestBuffer:     100,
		ResponseBuffer:    100,
		WorkerCount:       4,
		RequestTimeout:    time.Minute * 5,
		ShutdownTimeout:   time.Second * 30,
		ChromaURL:         "http://localhost:8000",
		ChromaTenant:      "default_tenant",
		ChromaDatabase:    "default_database",
		OllamaURL:         "http://localhost:11434",
		IntelligenceModel: "llama3.2:1b",
		EmbeddingModel:    "nomic-embed-text",
		LLMTimeout:        time.Second * 60,
		LLMRetryAttempts:  3,
	}
}

// initializeMetaCollections creates the essential meta collections in ChromaDB on startup
// These collections are part of the core IDAES design for organizing different types of data:
// - documents: Main document chunks with embeddings
// - entities: Extracted entities and their relationships
// - citations: Academic citations and cross-references
// - topics: Topic analysis and clustering results
// - metadata: Document metadata and classifications
func initializeMetaCollections(ctx context.Context, config *analyzers.AnalysisSystemConfig) error {
	slog.InfoContext(ctx, "Initializing meta collections",
		"url", config.ChromaDBURL,
		"tenant", config.ChromaDBTenant,
		"database", config.ChromaDBDatabase)

	// Create ChromaDB client
	chromaClient := clients.NewChromaDBClient(
		config.ChromaDBURL,
		30*time.Second,
		3,
		config.ChromaDBTenant,
		config.ChromaDBDatabase,
	)

	// Test connection first
	if err := chromaClient.Health(ctx); err != nil {
		return fmt.Errorf("cannot connect to ChromaDB: %w", err)
	}

	// Define meta collections according to IDAES design
	metaCollections := []struct {
		name        string
		description string
		metadata    map[string]any
	}{
		{
			name:        "documents",
			description: "Main document chunks with embeddings for semantic search",
			metadata: map[string]any{
				"type":       "document_chunks",
				"purpose":    "semantic_search_and_storage",
				"embedding":  true,
				"created_by": "idaes_subsystem",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name:        "entities",
			description: "Extracted entities and their relationships across documents",
			metadata: map[string]any{
				"type":       "entity_relationships",
				"purpose":    "entity_extraction_and_linking",
				"embedding":  true,
				"created_by": "idaes_subsystem",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name:        "citations",
			description: "Academic citations and cross-references between documents",
			metadata: map[string]any{
				"type":       "citation_network",
				"purpose":    "citation_analysis_and_linking",
				"embedding":  true,
				"created_by": "idaes_subsystem",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name:        "topics",
			description: "Topic analysis and clustering results for content organization",
			metadata: map[string]any{
				"type":       "topic_clusters",
				"purpose":    "topic_modeling_and_clustering",
				"embedding":  true,
				"created_by": "idaes_subsystem",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name:        "metadata",
			description: "Document metadata and classifications for content management",
			metadata: map[string]any{
				"type":       "document_metadata",
				"purpose":    "classification_and_management",
				"embedding":  false,
				"created_by": "idaes_subsystem",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	// Create each collection if it doesn't exist
	for _, collection := range metaCollections {
		slog.InfoContext(ctx, "Ensuring meta collection exists", "collection", collection.name)

		// Try to get the collection first
		_, err := chromaClient.GetCollection(ctx, collection.name)
		if err != nil {
			// Collection doesn't exist, create it
			slog.InfoContext(ctx, "Creating meta collection", "collection", collection.name, "description", collection.description)
			_, err = chromaClient.CreateCollection(ctx, collection.name, collection.metadata)
			if err != nil {
				return fmt.Errorf("failed to create meta collection '%s': %w", collection.name, err)
			}
			slog.InfoContext(ctx, "Successfully created meta collection", "collection", collection.name)
		} else {
			slog.InfoContext(ctx, "Meta collection already exists", "collection", collection.name)
		}
	}

	slog.InfoContext(ctx, "Meta collections initialization completed successfully")
	return nil
}
