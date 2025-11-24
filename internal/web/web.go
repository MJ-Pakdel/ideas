package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/config"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// Server represents the main web server
type Server struct {
	mux             *http.ServeMux
	httpServer      *http.Server
	analysisSystem  *analyzers.AnalysisSystem // Legacy field for backward compatibility
	analysisService AnalysisService           // New field for pluggable analysis service
	config          *config.ServerConfig

	// WebSocket upgrade configuration
	upgrader websocket.Upgrader

	// Channel-based WebSocket connection management
	connectionManager *WebSocketConnectionManager
	broadcastCtx      context.Context
	broadcastCancel   context.CancelFunc
}

// WebSocketConnectionManager manages WebSocket connections using channels
type WebSocketConnectionManager struct {
	connections    map[*websocket.Conn]bool
	registerChan   chan *websocket.Conn
	unregisterChan chan *websocket.Conn
	broadcastChan  chan any
	statusChan     chan chan int // For querying connection count
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewWebSocketConnectionManager creates a new channel-based connection manager
func NewWebSocketConnectionManager(ctx context.Context) *WebSocketConnectionManager {
	managerCtx, cancel := context.WithCancel(ctx)

	manager := &WebSocketConnectionManager{
		connections:    make(map[*websocket.Conn]bool),
		registerChan:   make(chan *websocket.Conn, 10),
		unregisterChan: make(chan *websocket.Conn, 10),
		broadcastChan:  make(chan any, 100),
		statusChan:     make(chan chan int, 10),
		ctx:            managerCtx,
		cancel:         cancel,
	}

	go manager.run()
	return manager
}

// run is the main goroutine that manages WebSocket connections via channels
func (m *WebSocketConnectionManager) run() {
	defer func() {
		// Close all connections on shutdown
		for conn := range m.connections {
			conn.Close()
		}
		close(m.registerChan)
		close(m.unregisterChan)
		close(m.broadcastChan)
		close(m.statusChan)
	}()

	for {
		select {
		case conn := <-m.registerChan:
			m.connections[conn] = true

		case conn := <-m.unregisterChan:
			if _, exists := m.connections[conn]; exists {
				delete(m.connections, conn)
				conn.Close()
			}

		case message := <-m.broadcastChan:
			// Broadcast to all active connections
			for conn := range m.connections {
				select {
				case <-m.ctx.Done():
					return
				default:
					if err := conn.WriteJSON(message); err != nil {
						// Connection failed, remove it
						delete(m.connections, conn)
						conn.Close()
					}
				}
			}

		case responseChan := <-m.statusChan:
			responseChan <- len(m.connections)

		case <-m.ctx.Done():
			return
		}
	}
}

// RegisterConnection adds a new WebSocket connection
func (m *WebSocketConnectionManager) RegisterConnection(conn *websocket.Conn) {
	select {
	case m.registerChan <- conn:
	case <-m.ctx.Done():
	}
}

// UnregisterConnection removes a WebSocket connection
func (m *WebSocketConnectionManager) UnregisterConnection(conn *websocket.Conn) {
	select {
	case m.unregisterChan <- conn:
	case <-m.ctx.Done():
	}
}

// Broadcast sends a message to all connected clients
func (m *WebSocketConnectionManager) Broadcast(message any) {
	select {
	case m.broadcastChan <- message:
	case <-m.ctx.Done():
	}
}

// GetConnectionCount returns the current number of active connections
func (m *WebSocketConnectionManager) GetConnectionCount() int {
	responseChan := make(chan int, 1)
	select {
	case m.statusChan <- responseChan:
		return <-responseChan
	case <-m.ctx.Done():
		return 0
	}
}

// Stop gracefully shuts down the connection manager
func (m *WebSocketConnectionManager) Stop() {
	m.cancel()
}

// AnalysisService defines the interface for document analysis and search
// This allows the web server to work with either direct AnalysisSystem or IDAES subsystem
type AnalysisService interface {
	// AnalyzeDocument analyzes a document and returns the results
	AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error)

	// Health returns the health status of the analysis service
	Health() any

	// Close cleans up resources (optional)
	Close() error

	// Search methods
	SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error)
	SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error)
	SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error)
	SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error)
	SemanticSearch(ctx context.Context, query string, limit int) ([]*types.Document, error)

	// Document management
	DeleteDocument(ctx context.Context, documentID string) error
}

// NewServerWithAnalysisService creates a new web server instance with a provided analysis service
func NewServerWithAnalysisService(ctx context.Context, cfg *config.ServerConfig, analysisService AnalysisService) (*Server, error) {
	// Load configuration with defaults
	if cfg == nil {
		cfg = getDefaultServerConfig()
	}

	// Setup HTTP multiplexer
	mux := http.NewServeMux()

	// Create broadcast context for WebSocket management
	broadcastCtx, broadcastCancel := context.WithCancel(ctx)

	server := &Server{
		mux:             mux,
		analysisSystem:  nil, // Using external analysis service
		analysisService: analysisService,
		config:          cfg,

		// WebSocket configuration
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin for development
				// In production, this should be more restrictive
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},

		// Channel-based connection management
		connectionManager: NewWebSocketConnectionManager(broadcastCtx),
		broadcastCtx:      broadcastCtx,
		broadcastCancel:   broadcastCancel,
	}

	// Store the analysis service for use in handlers
	// We'll need to modify the Server struct to include this
	// For now, we'll use a simple approach

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// getDefaultServerConfig returns default server configuration
func getDefaultServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		EnableCORS:   true,
		StaticDir:    "./web/static",
		TemplateDir:  "./web/templates",
	}
}

// Start starts the IDAES server
func (s *Server) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "Starting IDAES server")

	// Start analysis system only if it exists (not using external service)
	if s.analysisSystem != nil {
		if err := s.analysisSystem.Start(ctx); err != nil {
			return fmt.Errorf("failed to start analysis system: %w", err)
		}
		// Start metrics broadcasting system using the analysis system context
		s.startMetricsBroadcast(s.analysisSystem.GetSystemContext())
	} else {
		// Using external analysis service, start metrics broadcasting with server context
		s.startMetricsBroadcast(ctx)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      s.middlewareChain(s.mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	slog.InfoContext(ctx, "Starting HTTP server", "host", s.config.Host, "port", s.config.Port)

	// Start server
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Stop gracefully stops the IDAES server
func (s *Server) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "Stopping IDAES server")

	// Stop broadcast system first
	if s.broadcastCancel != nil {
		s.broadcastCancel()
	}

	// Stop connection manager (closes all WebSocket connections)
	if s.connectionManager != nil {
		s.connectionManager.Stop()
	}

	// Stop HTTP server
	if s.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "Error shutting down HTTP server", "error", err)
		}
	}

	// Stop analysis system only if it exists (not using external service)
	if s.analysisSystem != nil {
		if err := s.analysisSystem.Stop(ctx); err != nil {
			slog.ErrorContext(ctx, "Error stopping analysis system", "error", err)
		}
	}

	slog.InfoContext(ctx, "IDAES server stopped")
	return nil
}

// createDualModelConfig creates analysis configuration with dual-model setup
func createDualModelConfig(cfg *config.ServerConfig) *analyzers.AnalysisSystemConfig {
	// Return default configuration if none provided
	return &analyzers.AnalysisSystemConfig{
		LLMConfig: &interfaces.LLMConfig{
			Provider:       "ollama",
			BaseURL:        "http://localhost:11434",
			Model:          "llama3.2:3b",      // Primary model for entity extraction
			EmbeddingModel: "nomic-embed-text", // Secondary model for embeddings
			Temperature:    0.1,
			MaxTokens:      4096,
			Timeout:        60 * time.Second,
			RetryAttempts:  3,
		},
		ChromaDBURL:        "http://localhost:8000",
		ChromaDBDatabase:   "idaes",
		PipelineConfig:     analyzers.DefaultAnalysisConfig(),
		OrchestratorConfig: analyzers.DefaultOrchestratorConfig(),
		EntityConfig: &interfaces.EntityExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM,
			MinConfidence: 0.7,
			MaxEntities:   100,
			EntityTypes:   []types.EntityType{},
		},
		CitationConfig: &interfaces.CitationExtractionConfig{
			Enabled:       true,
			Method:        types.ExtractorMethodLLM, // IDAES uses intelligent LLM-only entity extraction
			MinConfidence: 0.8,
			MaxCitations:  50,
			Formats:       []types.CitationFormat{},
		},
		TopicConfig: &interfaces.TopicExtractionConfig{
			Enabled:       true,
			Method:        "llm",
			NumTopics:     10,
			MinConfidence: 0.6,
			MaxKeywords:   20,
		},
	}
}

// middlewareChain applies middleware to the handler
func (s *Server) middlewareChain(next http.Handler) http.Handler {
	return s.loggingMiddleware(s.corsMiddleware(s.recoveryMiddleware(next)))
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)
		slog.InfoContext(r.Context(), "HTTP request", "method", r.Method, "path", r.URL.Path, "status", wrapped.statusCode, "duration", time.Since(start))
	})
}

// corsMiddleware handles CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware handles panics
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(r.Context(), "Panic recovered", "panic", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/v1/documents/upload", s.handleDocumentUpload)
	s.mux.HandleFunc("/api/v1/documents/analyze", s.handleDocumentAnalyze)
	s.mux.HandleFunc("/api/v1/documents/", s.handleDocumentOperations)

	// Analysis results endpoints
	s.mux.HandleFunc("/api/v1/analysis/", s.handleAnalysisOperations)

	// Unified extraction endpoints
	s.mux.HandleFunc("/api/v1/unified/status", s.handleUnifiedExtractionStatus)
	s.mux.HandleFunc("/api/v1/unified/config", s.handleUnifiedExtractionConfig)
	s.mux.HandleFunc("/api/v1/unified/analyze", s.handleUnifiedAnalyze)

	// Search endpoints
	s.mux.HandleFunc("/api/v1/search/documents", s.handleDocumentSearch)
	s.mux.HandleFunc("/api/v1/search/entities", s.handleSearchEntities)
	s.mux.HandleFunc("/api/v1/search/citations", s.handleSearchCitations)
	s.mux.HandleFunc("/api/v1/search/topics", s.handleSearchTopics)
	s.mux.HandleFunc("/api/v1/search/semantic", s.handleSemanticSearch)

	// System status and monitoring
	s.mux.HandleFunc("/api/v1/status", s.handleSystemStatus)
	s.mux.HandleFunc("/api/v1/health", s.handleHealthCheck)
	s.mux.HandleFunc("/api/v1/metrics", s.handleMetrics)

	// WebSocket endpoint for real-time updates
	s.mux.HandleFunc("/ws/metrics", s.handleWebSocketMetrics)

	// Static file serving
	fs := http.FileServer(http.Dir(s.config.StaticDir))
	s.mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Web UI routes
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/upload", s.handleUploadPage)
	s.mux.HandleFunc("/search", s.handleSearchPage)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/performance", s.handlePerformanceDashboard)
	s.mux.HandleFunc("/results", s.handleResultsPage)
	s.mux.HandleFunc("/results/", s.handleResultsPage)
}

// API Response helpers
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func (s *Server) sendJSON(w http.ResponseWriter, r *http.Request, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: statusCode < 400,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode JSON response", "error", err)
	}
}

func (s *Server) sendError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: false,
		Error:   message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode error response", "error", err)
	}
}

// Document upload handler
func (s *Server) handleDocumentUpload(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "Document upload handler called", "method", r.Method)

	if r.Method != http.MethodPost {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	slog.DebugContext(r.Context(), "Parsing multipart form")
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32 MB max
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to parse form data", "error", err)
		s.sendError(w, r, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	slog.DebugContext(r.Context(), "Getting form file")
	file, header, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(r.Context(), "No file provided", "error", err)
		s.sendError(w, r, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	slog.DebugContext(r.Context(), "Reading file content", "filename", header.Filename)
	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to read file", "error", err)
		s.sendError(w, r, http.StatusInternalServerError, "Failed to read file")
		return
	}

	slog.DebugContext(r.Context(), "Creating document", "filename", header.Filename, "size", len(content))
	// Create document
	doc := types.NewDocument(
		header.Filename,
		header.Filename,
		string(content),
		header.Header.Get("Content-Type"),
	)

	slog.DebugContext(r.Context(), "Document created", "doc_id", doc.ID, "filename", doc.Name)
	slog.DebugContext(r.Context(), "Checking analysis service", "analysis_service_nil", s.analysisService == nil)

	if s.analysisService == nil {
		slog.ErrorContext(r.Context(), "Analysis service is nil")
		s.sendError(w, r, http.StatusInternalServerError, "Analysis service not initialized")
		return
	}

	slog.DebugContext(r.Context(), "Starting document analysis (which includes storage)", "doc_id", doc.ID)
	// Analyze document (this should include storage in the IDAES architecture)
	analysisResult, err := s.analysisService.AnalyzeDocument(r.Context(), doc)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to analyze document", "error", err)
		s.sendError(w, r, http.StatusInternalServerError, "Failed to analyze document")
		return
	}

	slog.DebugContext(r.Context(), "Document analysis completed", "doc_id", doc.ID, "result", analysisResult != nil)
	slog.InfoContext(r.Context(), "document uploaded successfully", slog.String("document_id", doc.ID), slog.String("filename", header.Filename), slog.Int("size", len(content)))

	s.sendJSON(w, r, http.StatusCreated, map[string]any{
		"document_id": doc.ID,
		"filename":    doc.Name,
		"size":        doc.Size,
		"uploaded_at": doc.CreatedAt,
	})
}

// Document analysis handler
func (s *Server) handleDocumentAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var request struct {
		DocumentID string `json:"document_id"`
		Content    string `json:"content,omitempty"`
		Filename   string `json:"filename,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, r, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var doc *types.Document
	var err error

	if request.DocumentID != "" {
		// Analyze existing document
		doc, err = s.analysisSystem.StorageManager.GetDocument(r.Context(), request.DocumentID)
		if err != nil {
			s.sendError(w, r, http.StatusNotFound, "Document not found")
			return
		}
		if doc == nil {
			s.sendError(w, r, http.StatusNotFound, "Document not found")
			return
		}
	} else if request.Content != "" {
		// Analyze provided content
		filename := request.Filename
		if filename == "" {
			filename = "untitled.txt"
		}
		doc = types.NewDocument(filename, filename, request.Content, "text/plain")
	} else {
		s.sendError(w, r, http.StatusBadRequest, "Either document_id or content must be provided")
		return
	}

	// Analyze document using the appropriate service
	var response *analyzers.AnalysisResponse

	if s.analysisService != nil {
		response, err = s.analysisService.AnalyzeDocument(r.Context(), doc)
	} else if s.analysisSystem != nil {
		response, err = s.analysisSystem.AnalyzeDocument(r.Context(), doc)
	} else {
		s.sendError(w, r, http.StatusInternalServerError, "No analysis service available")
		return
	}

	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to analyze document", "error", err)
		s.sendError(w, r, http.StatusInternalServerError, "Analysis failed")
		return
	}

	if response == nil {
		slog.ErrorContext(r.Context(), "Received nil response from analysis service")
		s.sendError(w, r, http.StatusInternalServerError, "Analysis service returned no response")
		return
	}

	if response.Error != nil {
		slog.ErrorContext(r.Context(), "Analysis completed with error", "error", response.Error)
		s.sendError(w, r, http.StatusInternalServerError, response.Error.Error())
		return
	}

	slog.InfoContext(r.Context(), "document analysis completed", slog.String("document_id", doc.ID), slog.String("processing_time", response.ProcessingTime.String()), slog.Int("entities", len(response.Result.Entities)), slog.Int("citations", len(response.Result.Citations)))

	s.sendJSON(w, r, http.StatusOK, map[string]any{
		"analysis_id":     response.Result.ID,
		"document_id":     doc.ID,
		"processing_time": response.ProcessingTime.String(),
		"entities":        response.Result.Entities,
		"citations":       response.Result.Citations,
		"topics":          response.Result.Topics,
		"extractors_used": response.ExtractorUsed,
	})
}

// System status handler
func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var status any
	if s.analysisSystem != nil {
		status = s.analysisSystem.GetSystemStatus()
	} else {
		status = map[string]any{
			"mode":    "external_service",
			"message": "Using external analysis service",
		}
	}
	s.sendJSON(w, r, http.StatusOK, status)
}

// Health check handler
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check system health
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services":  make(map[string]any),
	}

	// Use the analysisService Health method if available, otherwise fall back to direct system checks
	if s.analysisService != nil {
		// Get health from the analysis service
		serviceHealth := s.analysisService.Health()
		if serviceHealth != nil {
			health["analysis_service"] = serviceHealth
		}
	} else if s.analysisSystem != nil {
		// Check LLM client health
		if err := s.analysisSystem.LLMClient.Health(ctx); err != nil {
			health["services"].(map[string]any)["llm"] = map[string]any{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			health["status"] = "degraded"
		} else {
			health["services"].(map[string]any)["llm"] = map[string]any{
				"status": "healthy",
			}
		}

		// Check storage health
		if err := s.analysisSystem.StorageManager.Health(ctx); err != nil {
			health["services"].(map[string]any)["storage"] = map[string]any{
				"status": "unhealthy",
				"error":  err.Error(),
			}
			health["status"] = "degraded"
		} else {
			health["services"].(map[string]any)["storage"] = map[string]any{
				"status": "healthy",
			}
		}
	}

	statusCode := http.StatusOK
	if health["status"] == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	s.sendJSON(w, r, statusCode, health)
}

// Metrics handler
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var metrics map[string]any

	if s.analysisSystem != nil {
		metrics = map[string]any{
			"orchestrator": s.analysisSystem.Orchestrator.GetMetrics(),
			"pipeline":     s.analysisSystem.Pipeline.GetMetrics(),
			"workers":      s.analysisSystem.Orchestrator.GetWorkerStatus(),
		}
	} else {
		// Fetch metrics from the dedicated metrics server
		resp, err := http.Get("http://localhost:8082/debug/vars")
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to fetch metrics from metrics server", "error", err)
			metrics = map[string]any{
				"status":  "error",
				"message": "Failed to fetch metrics from metrics server",
				"error":   err.Error(),
			}
		} else if resp == nil {
			slog.ErrorContext(r.Context(), "Received nil response from metrics server")
			metrics = map[string]any{
				"status":  "error",
				"message": "Received nil response from metrics server",
			}
		} else {
			defer resp.Body.Close()

			var metricsData map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&metricsData); err != nil {
				slog.ErrorContext(r.Context(), "Failed to decode metrics data", "error", err)
				metrics = map[string]any{
					"status":  "error",
					"message": "Failed to decode metrics data",
					"error":   err.Error(),
				}
			} else {
				// Filter out only app-specific metrics for cleaner response
				appMetrics := make(map[string]any)
				for key, value := range metricsData {
					if strings.HasPrefix(key, "app.") {
						// Remove the "app." prefix for cleaner API response
						cleanKey := strings.TrimPrefix(key, "app.")
						appMetrics[cleanKey] = value
					} else if strings.HasPrefix(key, "runtime.") {
						// Include runtime metrics with clean names
						cleanKey := strings.TrimPrefix(key, "runtime.")
						appMetrics[cleanKey] = value
					}
				}
				metrics = appMetrics
			}
		}
	}

	s.sendJSON(w, r, http.StatusOK, metrics)
}

// WebSocket metrics handler for real-time updates
func (s *Server) handleWebSocketMetrics(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "WebSocket upgrade failed", slog.Any("error", err))
		return
	}

	// Register connection with channel-based manager
	s.connectionManager.RegisterConnection(conn)
	connectionCount := s.connectionManager.GetConnectionCount()

	slog.InfoContext(r.Context(), "WebSocket connection established",
		"remote_addr", r.RemoteAddr,
		"total_connections", connectionCount)

	// Handle connection cleanup
	defer func() {
		s.connectionManager.UnregisterConnection(conn)
		remainingCount := s.connectionManager.GetConnectionCount()

		slog.InfoContext(r.Context(), "WebSocket connection closed",
			"remote_addr", r.RemoteAddr,
			"remaining_connections", remainingCount)
	}()

	// Start metrics broadcasting for this connection
	s.handleWebSocketConnection(conn, r.Context())
}

// handleWebSocketConnection manages individual WebSocket connection lifecycle
func (s *Server) handleWebSocketConnection(conn *websocket.Conn, ctx context.Context) {
	// Set up periodic metrics broadcasting
	ticker := time.NewTicker(2 * time.Second) // Broadcast every 2 seconds
	defer ticker.Stop()

	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping/pong to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-s.broadcastCtx.Done():
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Broadcast current metrics
			if err := s.broadcastMetricsToConnection(conn); err != nil {
				slog.ErrorContext(ctx, "Failed to broadcast metrics", slog.Any("error", err))
				return
			}
		case <-pingTicker.C:
			// Send ping to keep connection alive
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.ErrorContext(ctx, "Failed to send ping", slog.Any("error", err))
				return
			}
		}
	}
}

// broadcastMetricsToConnection sends current metrics to a specific WebSocket connection
func (s *Server) broadcastMetricsToConnection(conn *websocket.Conn) error {
	// Gather current metrics (same as REST endpoint)
	var metrics map[string]any
	if s.analysisSystem != nil {
		metrics = map[string]any{
			"orchestrator": s.analysisSystem.Orchestrator.GetMetrics(),
			"pipeline":     s.analysisSystem.Pipeline.GetMetrics(),
			"workers":      s.analysisSystem.Orchestrator.GetWorkerStatus(),
			"timestamp":    time.Now().UTC(),
		}
	} else {
		metrics = map[string]any{
			"status":    "external_service",
			"message":   "Metrics available through analysis service",
			"timestamp": time.Now().UTC(),
		}
	}

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Send metrics as JSON
	return conn.WriteJSON(metrics)
}

// startMetricsBroadcast initializes the global metrics broadcasting system
func (s *Server) startMetricsBroadcast(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Second) // Check for broadcasts every second
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.broadcastCtx.Done():
				return
			case <-ticker.C:
				s.broadcastToAllConnections()
			}
		}
	}()
}

// broadcastToAllConnections sends metrics to all active WebSocket connections
func (s *Server) broadcastToAllConnections() {
	connectionCount := s.connectionManager.GetConnectionCount()
	if connectionCount == 0 {
		return // No connections to broadcast to
	}

	// Gather metrics once for all connections
	var metrics map[string]any
	if s.analysisSystem != nil {
		metrics = map[string]any{
			"orchestrator": s.analysisSystem.Orchestrator.GetMetrics(),
			"pipeline":     s.analysisSystem.Pipeline.GetMetrics(),
			"workers":      s.analysisSystem.Orchestrator.GetWorkerStatus(),
			"timestamp":    time.Now().UTC(),
		}
	} else {
		metrics = map[string]any{
			"status":    "external_service",
			"message":   "Metrics available through analysis service",
			"timestamp": time.Now().UTC(),
		}
	}

	// Broadcast using channel-based manager
	s.connectionManager.Broadcast(metrics)
}

// Unified extraction status handler - GET /api/v1/unified/status
func (s *Server) handleUnifiedExtractionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var status map[string]any

	if s.analysisSystem != nil {
		// Legacy mode - direct system access
		pipelineConfig := s.analysisSystem.Pipeline.GetConfig()

		if pipelineConfig == nil {
			status = map[string]any{
				"unified_extraction_enabled": false,
				"fallback_enabled":           false,
				"extractor_registered":       false,
				"extraction_timeout":         0,
				"max_concurrent_analyses":    0,
				"pipeline_pattern":           "unknown",
				"compression_enabled":        false,
				"compression_threshold":      0,
				"error":                      "pipeline configuration not available",
			}
		} else {
			status = map[string]any{
				"unified_extraction_enabled": pipelineConfig.UseUnifiedExtraction,
				"fallback_enabled":           pipelineConfig.UnifiedFallbackEnabled,
				"extractor_registered":       s.analysisSystem.Pipeline.HasUnifiedExtractor(),
				"extraction_timeout":         pipelineConfig.AnalysisTimeout.Seconds(),
				"max_concurrent_analyses":    pipelineConfig.MaxConcurrentAnalyses,
				"min_confidence_score":       pipelineConfig.MinConfidenceScore,
				"retry_attempts":             pipelineConfig.RetryAttempts,
				"enable_entity_extraction":   pipelineConfig.EnableEntityExtraction,
				"enable_citation_extraction": pipelineConfig.EnableCitationExtraction,
				"enable_topic_extraction":    pipelineConfig.EnableTopicExtraction,
			}
		}

		// Add metrics if available
		pipelineMetrics := s.analysisSystem.Pipeline.GetMetrics()
		if pipelineMetrics != nil {
			status["pipeline_metrics"] = map[string]any{
				"total_analyses":          pipelineMetrics.TotalAnalyses,
				"successful_analyses":     pipelineMetrics.SuccessfulAnalyses,
				"failed_analyses":         pipelineMetrics.FailedAnalyses,
				"average_processing_time": pipelineMetrics.AverageProcessingTime.Milliseconds(),
				"extractor_usage":         pipelineMetrics.ExtractorUsageCount,
				"error_counts":            pipelineMetrics.ErrorCounts,
			}
		}
	} else {
		// Service mode - limited access
		status = map[string]any{
			"status":  "service_mode",
			"message": "Unified extraction status available through analysis service",
			"health":  s.analysisService.Health(),
		}
	}

	s.sendJSON(w, r, http.StatusOK, status)
}

// Unified extraction configuration handler - GET/PUT /api/v1/unified/config
func (s *Server) handleUnifiedExtractionConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetUnifiedConfig(w, r)
	case http.MethodPut:
		s.handleUpdateUnifiedConfig(w, r)
	default:
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// Get unified extraction configuration
func (s *Server) handleGetUnifiedConfig(w http.ResponseWriter, r *http.Request) {
	if s.analysisSystem == nil {
		s.sendError(w, r, http.StatusServiceUnavailable, "Direct pipeline access not available in service mode")
		return
	}

	config := s.analysisSystem.Pipeline.GetConfig()
	if config == nil {
		s.sendError(w, r, http.StatusInternalServerError, "Pipeline configuration not available")
		return
	}

	response := map[string]any{
		"use_unified_extraction":     config.UseUnifiedExtraction,
		"unified_fallback_enabled":   config.UnifiedFallbackEnabled,
		"enable_entity_extraction":   config.EnableEntityExtraction,
		"enable_citation_extraction": config.EnableCitationExtraction,
		"enable_topic_extraction":    config.EnableTopicExtraction,
		"min_confidence_score":       config.MinConfidenceScore,
		"max_concurrent_analyses":    config.MaxConcurrentAnalyses,
		"analysis_timeout_seconds":   config.AnalysisTimeout.Seconds(),
		"preferred_entity_method":    config.PreferredEntityMethod,
	}

	s.sendJSON(w, r, http.StatusOK, response)
}

// Update unified extraction configuration
func (s *Server) handleUpdateUnifiedConfig(w http.ResponseWriter, r *http.Request) {
	if s.analysisSystem == nil {
		s.sendError(w, r, http.StatusServiceUnavailable, "Direct pipeline access not available in service mode")
		return
	}

	var updateRequest struct {
		UseUnifiedExtraction     *bool    `json:"use_unified_extraction,omitempty"`
		UnifiedFallbackEnabled   *bool    `json:"unified_fallback_enabled,omitempty"`
		EnableEntityExtraction   *bool    `json:"enable_entity_extraction,omitempty"`
		EnableCitationExtraction *bool    `json:"enable_citation_extraction,omitempty"`
		EnableTopicExtraction    *bool    `json:"enable_topic_extraction,omitempty"`
		MinConfidenceScore       *float64 `json:"min_confidence_score,omitempty"`
		MaxConcurrentAnalyses    *int     `json:"max_concurrent_analyses,omitempty"`
		AnalysisTimeoutSeconds   *float64 `json:"analysis_timeout_seconds,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil {
		s.sendError(w, r, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Get current config
	currentConfig := s.analysisSystem.Pipeline.GetConfig()
	if currentConfig == nil {
		s.sendError(w, r, http.StatusInternalServerError, "Pipeline configuration not available")
		return
	}

	// Apply updates
	if updateRequest.UseUnifiedExtraction != nil {
		currentConfig.UseUnifiedExtraction = *updateRequest.UseUnifiedExtraction
	}
	if updateRequest.UnifiedFallbackEnabled != nil {
		currentConfig.UnifiedFallbackEnabled = *updateRequest.UnifiedFallbackEnabled
	}
	if updateRequest.EnableEntityExtraction != nil {
		currentConfig.EnableEntityExtraction = *updateRequest.EnableEntityExtraction
	}
	if updateRequest.EnableCitationExtraction != nil {
		currentConfig.EnableCitationExtraction = *updateRequest.EnableCitationExtraction
	}
	if updateRequest.EnableTopicExtraction != nil {
		currentConfig.EnableTopicExtraction = *updateRequest.EnableTopicExtraction
	}
	if updateRequest.MinConfidenceScore != nil {
		currentConfig.MinConfidenceScore = *updateRequest.MinConfidenceScore
	}
	if updateRequest.MaxConcurrentAnalyses != nil {
		currentConfig.MaxConcurrentAnalyses = *updateRequest.MaxConcurrentAnalyses
	}
	if updateRequest.AnalysisTimeoutSeconds != nil {
		currentConfig.AnalysisTimeout = time.Duration(*updateRequest.AnalysisTimeoutSeconds * float64(time.Second))
	}

	// Update the pipeline configuration
	err := s.analysisSystem.Pipeline.UpdateConfig(currentConfig)
	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "Failed to update configuration: "+err.Error())
		return
	}

	// Return updated configuration
	s.handleGetUnifiedConfig(w, r)
}

// Unified analyze handler - POST /api/v1/unified/analyze
func (s *Server) handleUnifiedAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check if we have either analysis system or service
	if s.analysisSystem == nil && s.analysisService == nil {
		s.sendError(w, r, http.StatusServiceUnavailable, "No analysis system available")
		return
	}

	var analyzeRequest struct {
		Text         string `json:"text"`
		Filename     string `json:"filename,omitempty"`
		MimeType     string `json:"mime_type,omitempty"`
		ForceUnified *bool  `json:"force_unified,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&analyzeRequest); err != nil {
		s.sendError(w, r, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if analyzeRequest.Text == "" {
		s.sendError(w, r, http.StatusBadRequest, "Text field is required")
		return
	}

	// Set defaults
	if analyzeRequest.Filename == "" {
		analyzeRequest.Filename = "api_request.txt"
	}
	if analyzeRequest.MimeType == "" {
		analyzeRequest.MimeType = "text/plain"
	}

	// Create document
	document := types.NewDocument(
		analyzeRequest.Filename,
		analyzeRequest.Filename,
		analyzeRequest.Text,
		analyzeRequest.MimeType,
	)

	// Temporarily force unified extraction if requested (only works with direct system access)
	var originalConfig *analyzers.AnalysisConfig
	if analyzeRequest.ForceUnified != nil && *analyzeRequest.ForceUnified && s.analysisSystem != nil {
		originalConfig = s.analysisSystem.Pipeline.GetConfig()
		if originalConfig != nil {
			tempConfig := *originalConfig
			tempConfig.UseUnifiedExtraction = true
			s.analysisSystem.Pipeline.UpdateConfig(&tempConfig)
			defer s.analysisSystem.Pipeline.UpdateConfig(originalConfig)
		}
	}

	// Analyze document using available service
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var response *analyzers.AnalysisResponse
	var err error

	if s.analysisSystem != nil {
		response, err = s.analysisSystem.AnalyzeDocument(ctx, document)
	} else {
		response, err = s.analysisService.AnalyzeDocument(ctx, document)
	}

	if err != nil {
		s.sendError(w, r, http.StatusInternalServerError, "Analysis failed: "+err.Error())
		return
	}

	// Check for analysis errors
	if response.Error != nil {
		s.sendError(w, r, http.StatusInternalServerError, "Analysis error: "+response.Error.Error())
		return
	}

	// Prepare response
	analysisResponse := map[string]any{
		"document_id":      response.Result.DocumentID,
		"entities":         response.Result.Entities,
		"citations":        response.Result.Citations,
		"topics":           response.Result.Topics,
		"processing_time":  response.ProcessingTime.Milliseconds(),
		"extraction_mode":  getExtractionMode(response),
		"confidence_score": getOverallConfidence(response.Result),
	}

	s.sendJSON(w, r, http.StatusOK, analysisResponse)
}

// Helper function to determine extraction mode used
func getExtractionMode(_ *analyzers.AnalysisResponse) string {
	// This would need to be implemented based on how the response tracks extraction mode
	// For now, return a placeholder
	return "unified" // Could be "unified", "individual", or "fallback"
}

// Helper function to calculate overall confidence score
func getOverallConfidence(result *types.AnalysisResult) float64 {
	if result == nil {
		return 0.0
	}

	var totalConfidence float64
	var count int

	// Add entity confidences
	for _, entity := range result.Entities {
		totalConfidence += entity.Confidence
		count++
	}

	// Add citation confidences
	for _, citation := range result.Citations {
		totalConfidence += citation.Confidence
		count++
	}

	// Add topic confidences
	for _, topic := range result.Topics {
		totalConfidence += topic.Confidence
		count++
	}

	if count == 0 {
		return 0.0
	}

	return totalConfidence / float64(count)
}

// Placeholder handlers for other endpoints
func (s *Server) handleDocumentOperations(w http.ResponseWriter, r *http.Request) {
	// Parse document ID from URL path: /api/v1/documents/{id} or /api/v1/documents/{id}/citations
	pathPrefix := "/api/v1/documents/"
	if !strings.HasPrefix(r.URL.Path, pathPrefix) {
		s.sendError(w, r, http.StatusNotFound, "Invalid document path")
		return
	}

	// Extract the path after /api/v1/documents/
	remainder := strings.TrimPrefix(r.URL.Path, pathPrefix)
	slog.Debug("Document operations", "url_path", r.URL.Path, "remainder", remainder)

	if remainder == "" {
		s.sendError(w, r, http.StatusBadRequest, "Document ID required")
		return
	}

	// Split the remainder to get document ID and optional sub-path
	pathParts := strings.Split(remainder, "/")
	if len(pathParts) == 0 {
		s.sendError(w, r, http.StatusBadRequest, "Invalid document path")
		return
	}
	documentID := pathParts[0]

	if documentID == "" {
		s.sendError(w, r, http.StatusBadRequest, "Document ID cannot be empty")
		return
	}

	slog.Debug("Document operations routing",
		"document_id", documentID,
		"path_parts", pathParts,
		"method", r.Method)

	// Route based on the sub-path and HTTP method
	if len(pathParts) == 1 {
		// /api/v1/documents/{id}
		switch r.Method {
		case http.MethodDelete:
			s.handleDocumentDelete(w, r, documentID)
		case http.MethodGet:
			s.handleDocumentGet(w, r, documentID)
		default:
			s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed for document operations")
		}
	} else if len(pathParts) == 2 && pathParts[1] == "citations" {
		// /api/v1/documents/{id}/citations
		slog.Debug("Routing to citations handler", "document_id", documentID)
		switch r.Method {
		case http.MethodGet:
			s.handleDocumentCitations(w, r, documentID)
		default:
			s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed for citations")
		}
	} else {
		s.sendError(w, r, http.StatusNotFound, "Invalid document operation path")
	}
}

func (s *Server) handleAnalysisOperations(w http.ResponseWriter, r *http.Request) {
	s.sendError(w, r, http.StatusNotImplemented, "Analysis operations not yet implemented")
}

// handleDocumentCitations returns all citations extracted from a specific document
func (s *Server) handleDocumentCitations(w http.ResponseWriter, r *http.Request, documentID string) {
	ctx := r.Context()

	slog.DebugContext(ctx, "Getting citations for document", "document_id", documentID)

	// Get citations filtered by document ID
	var documentCitations []*types.Citation

	if s.analysisSystem != nil {
		// Direct access mode - use a meaningful search query to get all citations
		storageManager := s.analysisSystem.StorageManager
		allCitations, err := storageManager.SearchCitations(ctx, "citation document reference", 1000) // Use meaningful query
		if err != nil {
			slog.ErrorContext(ctx, "Failed to retrieve citations", "error", err, "document_id", documentID)
			s.sendError(w, r, http.StatusInternalServerError, "Failed to retrieve citations: "+err.Error())
			return
		}

		// Filter citations by document ID
		for _, citation := range allCitations {
			if citation.DocumentID == documentID {
				documentCitations = append(documentCitations, citation)
			}
		}
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		allCitations, err := s.analysisService.SearchCitations(ctx, "citation document reference", 1000)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to retrieve citations", "error", err, "document_id", documentID)
			s.sendError(w, r, http.StatusInternalServerError, "Failed to retrieve citations: "+err.Error())
			return
		}

		// Filter citations by document ID
		for _, citation := range allCitations {
			if citation.DocumentID == documentID {
				documentCitations = append(documentCitations, citation)
			}
		}
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	slog.InfoContext(ctx, "Found citations for document",
		"document_id", documentID,
		"citation_count", len(documentCitations))

	// The e2e test expects a simple array of citations, not the standard API response format
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(documentCitations); err != nil {
		slog.ErrorContext(ctx, "Failed to encode citations response", "error", err)
		s.sendError(w, r, http.StatusInternalServerError, "Failed to encode response")
		return
	}
}

// handleDocumentGet retrieves a specific document by ID
func (s *Server) handleDocumentGet(w http.ResponseWriter, r *http.Request, documentID string) {
	// Placeholder for future implementation
	s.sendError(w, r, http.StatusNotImplemented, "Document retrieval not yet implemented")
}

// handleDocumentDelete deletes a specific document by ID
func (s *Server) handleDocumentDelete(w http.ResponseWriter, r *http.Request, documentID string) {
	ctx := r.Context()

	slog.InfoContext(ctx, "Starting document deletion",
		"document_id", documentID,
		"remote_addr", r.RemoteAddr,
	)

	// Validate document ID
	if documentID == "" {
		s.sendError(w, r, http.StatusBadRequest, "Document ID is required")
		return
	}

	// Delete the document using the analysis service or system
	var err error
	if s.analysisSystem != nil {
		// Direct access mode
		storageManager := s.analysisSystem.StorageManager
		err = storageManager.DeleteDocument(ctx, documentID)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		err = s.analysisService.DeleteDocument(ctx, documentID)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No analysis system available")
		return
	}

	if err != nil {
		slog.ErrorContext(ctx, "Failed to delete document",
			"document_id", documentID,
			"error", err,
		)
		s.sendError(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to delete document: %v", err))
		return
	}

	slog.InfoContext(ctx, "Document deleted successfully",
		"document_id", documentID,
	)

	// Send success response
	response := map[string]any{
		"success":     true,
		"message":     "Document deleted successfully",
		"document_id": documentID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleDocumentSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		s.sendError(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	collection := r.URL.Query().Get("collection")
	if collection == "" {
		collection = "documents" // Default collection
	}

	// Get storage manager from analysis system or use analysis service
	var documents []*types.Document
	var err error
	ctx := r.Context()

	if s.analysisSystem != nil {
		// Direct access mode
		storageManager := s.analysisSystem.StorageManager
		documents, err = storageManager.SearchDocumentsInCollection(ctx, collection, queryText, limit)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		documents, err = s.analysisService.SearchDocumentsInCollection(ctx, collection, queryText, limit)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	// Perform the search
	if err != nil {
		slog.ErrorContext(ctx, "Document search failed", "error", err, "query", queryText)
		s.sendError(w, r, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	// Prepare response
	searchResponse := map[string]any{
		"query":       queryText,
		"collection":  collection,
		"limit":       limit,
		"total_found": len(documents),
		"documents":   documents,
	}

	s.sendJSON(w, r, http.StatusOK, searchResponse)
}

func (s *Server) handleSearchEntities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		s.sendError(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Optional filter by entity type
	entityType := r.URL.Query().Get("type")

	// Get storage manager from analysis system or use analysis service
	var entities []*types.Entity
	var err error
	ctx := r.Context()

	if s.analysisSystem != nil {
		// Direct access mode
		storageManager := s.analysisSystem.StorageManager
		entities, err = storageManager.SearchEntities(ctx, queryText, limit)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		entities, err = s.analysisService.SearchEntities(ctx, queryText, limit)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	// Perform the search
	if err != nil {
		slog.ErrorContext(ctx, "Entity search failed", "error", err, "query", queryText)
		s.sendError(w, r, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	// Filter by entity type if specified
	if entityType != "" {
		filteredEntities := make([]*types.Entity, 0)
		for _, entity := range entities {
			if string(entity.Type) == entityType {
				filteredEntities = append(filteredEntities, entity)
			}
		}
		entities = filteredEntities
	}

	// Prepare response
	searchResponse := map[string]any{
		"query":       queryText,
		"type_filter": entityType,
		"limit":       limit,
		"total_found": len(entities),
		"entities":    entities,
	}

	s.sendJSON(w, r, http.StatusOK, searchResponse)
}

func (s *Server) handleSearchCitations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		s.sendError(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Optional filter by citation format
	format := r.URL.Query().Get("format")

	// Get storage manager from analysis system or use analysis service
	var citations []*types.Citation
	var err error
	ctx := r.Context()

	if s.analysisSystem != nil {
		// Direct access mode
		storageManager := s.analysisSystem.StorageManager
		citations, err = storageManager.SearchCitations(ctx, queryText, limit)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		citations, err = s.analysisService.SearchCitations(ctx, queryText, limit)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	// Perform the search
	if err != nil {
		slog.ErrorContext(ctx, "Citation search failed", "error", err, "query", queryText)
		s.sendError(w, r, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	// Filter by citation format if specified
	if format != "" {
		filteredCitations := make([]*types.Citation, 0)
		for _, citation := range citations {
			if string(citation.Format) == format {
				filteredCitations = append(filteredCitations, citation)
			}
		}
		citations = filteredCitations
	}

	// Prepare response
	searchResponse := map[string]any{
		"query":         queryText,
		"format_filter": format,
		"limit":         limit,
		"total_found":   len(citations),
		"citations":     citations,
	}

	s.sendJSON(w, r, http.StatusOK, searchResponse)
}

func (s *Server) handleSearchTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		s.sendError(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Optional minimum confidence filter
	minConfidenceStr := r.URL.Query().Get("min_confidence")
	var minConfidence float64
	if minConfidenceStr != "" {
		if parsed, err := strconv.ParseFloat(minConfidenceStr, 64); err == nil && parsed >= 0 && parsed <= 1 {
			minConfidence = parsed
		}
	}

	// Get storage manager from analysis system or use analysis service
	var topics []*types.Topic
	var err error
	ctx := r.Context()

	if s.analysisSystem != nil {
		// Direct access mode
		storageManager := s.analysisSystem.StorageManager
		topics, err = storageManager.SearchTopics(ctx, queryText, limit)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		topics, err = s.analysisService.SearchTopics(ctx, queryText, limit)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	// Perform the search
	if err != nil {
		slog.ErrorContext(ctx, "Topic search failed", "error", err, "query", queryText)
		s.sendError(w, r, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	// Filter by minimum confidence if specified
	if minConfidence > 0 {
		filteredTopics := make([]*types.Topic, 0)
		for _, topic := range topics {
			if topic.Confidence >= minConfidence {
				filteredTopics = append(filteredTopics, topic)
			}
		}
		topics = filteredTopics
	}

	// Prepare response
	searchResponse := map[string]any{
		"query":          queryText,
		"min_confidence": minConfidence,
		"limit":          limit,
		"total_found":    len(topics),
		"topics":         topics,
	}

	s.sendJSON(w, r, http.StatusOK, searchResponse)
}

func (s *Server) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		s.sendError(w, r, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 5 // Default limit per type for semantic search
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	// Optional filter by types to search
	searchTypes := r.URL.Query().Get("types")
	includeDocuments := true
	includeEntities := true
	includeCitations := true
	includeTopics := true

	if searchTypes != "" {
		// Parse comma-separated list of types
		types := make(map[string]bool)
		for _, t := range strings.Split(searchTypes, ",") {
			types[strings.TrimSpace(t)] = true
		}
		includeDocuments = types["documents"]
		includeEntities = types["entities"]
		includeCitations = types["citations"]
		includeTopics = types["topics"]
	}

	// Get documents via semantic search
	var documents []*types.Document
	var documentsErr error
	ctx := r.Context()

	if s.analysisSystem != nil {
		// Direct access mode - use SearchDocumentsInCollection for semantic search
		storageManager := s.analysisSystem.StorageManager
		documents, documentsErr = storageManager.SearchDocumentsInCollection(ctx, "documents", queryText, limit)
	} else if s.analysisService != nil {
		// Service mode - use the AnalysisService interface
		documents, documentsErr = s.analysisService.SemanticSearch(ctx, queryText, limit)
	} else {
		s.sendError(w, r, http.StatusServiceUnavailable, "No storage system available")
		return
	}

	results := make(map[string]any)

	// Search documents
	if includeDocuments {
		if documentsErr != nil {
			slog.WarnContext(ctx, "Document search failed in semantic search", "error", documentsErr)
			results["documents"] = []any{}
		} else {
			results["documents"] = documents
			slog.InfoContext(ctx, "Document search succeeded", "count", len(documents))
		}
	}

	// Search entities
	if includeEntities {
		var entities []*types.Entity
		var entitiesErr error

		if s.analysisSystem != nil {
			entities, entitiesErr = s.analysisSystem.StorageManager.SearchEntities(ctx, queryText, limit)
		} else if s.analysisService != nil {
			entities, entitiesErr = s.analysisService.SearchEntities(ctx, queryText, limit)
		}

		if entitiesErr != nil {
			slog.WarnContext(ctx, "Entity search failed in semantic search", "error", entitiesErr)
			results["entities"] = []any{}
		} else {
			results["entities"] = entities
		}
	}

	// Search citations
	if includeCitations {
		var citations []*types.Citation
		var citationsErr error

		if s.analysisSystem != nil {
			citations, citationsErr = s.analysisSystem.StorageManager.SearchCitations(ctx, queryText, limit)
		} else if s.analysisService != nil {
			citations, citationsErr = s.analysisService.SearchCitations(ctx, queryText, limit)
		}

		if citationsErr != nil {
			slog.WarnContext(ctx, "Citation search failed in semantic search", "error", citationsErr)
			results["citations"] = []any{}
		} else {
			results["citations"] = citations
		}
	}

	// Search topics
	if includeTopics {
		var topics []*types.Topic
		var topicsErr error

		if s.analysisSystem != nil {
			topics, topicsErr = s.analysisSystem.StorageManager.SearchTopics(ctx, queryText, limit)
		} else if s.analysisService != nil {
			topics, topicsErr = s.analysisService.SearchTopics(ctx, queryText, limit)
		}

		if topicsErr != nil {
			slog.WarnContext(ctx, "Topic search failed in semantic search", "error", topicsErr)
			results["topics"] = []any{}
		} else {
			results["topics"] = topics
		}
	}

	// Calculate total results and collect unique document IDs
	totalResults := 0
	documentIDs := make(map[string]bool)

	if docs, ok := results["documents"].(([]*types.Document)); ok {
		totalResults += len(docs)
		for _, doc := range docs {
			documentIDs[doc.ID] = true
		}
	}
	if entities, ok := results["entities"].(([]*types.Entity)); ok {
		totalResults += len(entities)
		for _, entity := range entities {
			documentIDs[entity.DocumentID] = true
		}
	}
	if citations, ok := results["citations"].(([]*types.Citation)); ok {
		totalResults += len(citations)
		for _, citation := range citations {
			documentIDs[citation.DocumentID] = true
		}
	}
	if topics, ok := results["topics"].(([]*types.Topic)); ok {
		totalResults += len(topics)
		for _, topic := range topics {
			documentIDs[topic.DocumentID] = true
		}
	}

	// If documents search failed OR returned 0 results but we found related content, retrieve documents by ID
	if includeDocuments && len(documentIDs) > 0 {
		if docs, ok := results["documents"].(([]*types.Document)); ok {
			if len(docs) == 0 {
				slog.InfoContext(ctx, "Document search returned 0 results, attempting to retrieve via citation/entity references",
					"document_ids_found", len(documentIDs), "had_error", documentsErr != nil)

				var foundDocuments []*types.Document
				for docID := range documentIDs {
					if s.analysisSystem != nil {
						if doc, err := s.analysisSystem.StorageManager.GetDocument(ctx, docID); err == nil && doc != nil {
							foundDocuments = append(foundDocuments, doc)
						} else {
							slog.WarnContext(ctx, "Failed to retrieve document by ID", "document_id", docID, "error", err)
						}
					}
				}
				if len(foundDocuments) > 0 {
					results["documents"] = foundDocuments
					slog.InfoContext(ctx, "Retrieved documents via citation/entity references",
						"document_count", len(foundDocuments), "total_referenced", len(documentIDs))
				} else {
					slog.WarnContext(ctx, "No documents could be retrieved despite having references",
						"document_ids_attempted", len(documentIDs))
				}
			}
		}
	}
	if citations, ok := results["citations"].(([]*types.Citation)); ok {
		totalResults += len(citations)
	}
	if topics, ok := results["topics"].(([]*types.Topic)); ok {
		totalResults += len(topics)
	}

	// Prepare response
	searchResponse := map[string]any{
		"query":          queryText,
		"search_types":   searchTypes,
		"limit_per_type": limit,
		"total_results":  totalResults,
		"results":        results,
	}

	s.sendJSON(w, r, http.StatusOK, searchResponse)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get some basic stats to display
	var stats struct {
		TotalDocuments int    `json:"total_documents"`
		TotalAnalyses  int    `json:"total_analyses"`
		ActiveWorkers  int    `json:"active_workers"`
		SystemUptime   string `json:"system_uptime"`
		Status         string `json:"status"`
	}

	// You could fetch real stats here from your metrics or database
	stats.TotalDocuments = 0
	stats.TotalAnalyses = 0
	stats.ActiveWorkers = 1
	stats.SystemUptime = "Running"
	stats.Status = "healthy"

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IDAES - Intelligent Document Analysis</title>
    <link rel="stylesheet" href="/static/styles.css">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
</head>
<body>
    <div class="header">
        <div class="container">
            <h1>IDAES</h1>
            <p class="subtitle">Intelligent Document Analysis and Extraction System</p>
        </div>
    </div>

    <div class="container">
        <nav class="nav">
            <div class="nav-container">
                <div class="nav-links">
                    <a href="/" class="active">Home</a>
                    <a href="/upload">Upload</a>
                    <a href="/search">Search</a>
                    <a href="/dashboard">Dashboard</a>
                    <a href="/performance">Performance</a>
                    <a href="/results">Results</a>
                </div>
                <div class="nav-links">
                    <a href="/api/v1/health" class="btn btn-sm btn-secondary">Health Check</a>
                </div>
            </div>
        </nav>

        <!-- System Status Cards -->
        <div class="grid grid-cols-4 mb-8">
            <div class="stats-card">
                <h3>Documents Processed</h3>
                <div class="value" id="total-documents">%d</div>
            </div>
            <div class="stats-card">
                <h3>Analyses Completed</h3>
                <div class="value" id="total-analyses">%d</div>
            </div>
            <div class="stats-card">
                <h3>Active Workers</h3>
                <div class="value" id="active-workers">%d</div>
            </div>
            <div class="stats-card">
                <h3>System Status</h3>
                <div class="value" id="system-status">%s</div>
            </div>
        </div>

        <!-- Main Content -->
        <div class="grid grid-cols-2 mb-8">
            <div class="card">
                <div class="card-header">
                    <h3>🚀 Quick Start</h3>
                </div>
                <div class="card-body">
                    <p class="mb-4">Get started with document analysis in just a few clicks:</p>
                    <div class="mb-4">
                        <a href="/upload" class="btn btn-primary btn-lg w-full mb-2">
                            📄 Upload Document
                        </a>
                        <a href="/search" class="btn btn-secondary w-full">
                            🔍 Search Existing
                        </a>
                    </div>
                    <p class="text-sm text-gray-600">
                        Supported formats: PDF, DOC, DOCX, TXT
                    </p>
                </div>
            </div>

            <div class="card">
                <div class="card-header">
                    <h3>📊 System Overview</h3>
                </div>
                <div class="card-body">
                    <div class="metric">
                        <span class="metric-label">Current System Load</span>
                        <span class="metric-value" id="system-load">Low</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Processing Queue</span>
                        <span class="metric-value" id="queue-size">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Average Processing Time</span>
                        <span class="metric-value" id="avg-time">< 1s</span>
                    </div>
                    <div class="mt-4">
                        <a href="/performance" class="btn btn-primary btn-sm">View Detailed Metrics</a>
                    </div>
                </div>
            </div>
        </div>

        <!-- Features Section -->
        <div class="card mb-8">
            <div class="card-header">
                <h3>✨ Core Features</h3>
            </div>
            <div class="card-body">
                <div class="grid grid-cols-3">
                    <div class="text-center p-4">
                        <div class="mb-4" style="font-size: 3rem;">🤖</div>
                        <h4 class="mb-2">AI-Powered Analysis</h4>
                        <p class="text-sm text-gray-600">
                            Advanced entity extraction using dual AI models for maximum accuracy
                        </p>
                    </div>
                    <div class="text-center p-4">
                        <div class="mb-4" style="font-size: 3rem;">🔍</div>
                        <h4 class="mb-2">Semantic Search</h4>
                        <p class="text-sm text-gray-600">
                            Find documents and information using natural language queries
                        </p>
                    </div>
                    <div class="text-center p-4">
                        <div class="mb-4" style="font-size: 3rem;">📈</div>
                        <h4 class="mb-2">Real-time Analytics</h4>
                        <p class="text-sm text-gray-600">
                            Monitor processing performance and system health in real-time
                        </p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Recent Activity -->
        <div class="grid grid-cols-2">
            <div class="card">
                <div class="card-header">
                    <h3>📋 Recent Activity</h3>
                </div>
                <div class="card-body">
                    <div id="recent-activity">
                        <p class="text-gray-500 text-center py-8">
                            No recent activity to display
                        </p>
                    </div>
                    <div class="card-footer text-center">
                        <a href="/dashboard" class="btn btn-secondary btn-sm">View All Activity</a>
                    </div>
                </div>
            </div>

            <div class="card">
                <div class="card-header">
                    <h3>🔗 API Endpoints</h3>
                </div>
                <div class="card-body">
                    <div class="space-y-2">
                        <div class="flex justify-between items-center py-2">
                            <span class="text-sm font-medium">Health Check</span>
                            <a href="/api/v1/health" class="status-indicator status-success">
                                <span class="status-dot"></span>
                                Active
                            </a>
                        </div>
                        <div class="flex justify-between items-center py-2">
                            <span class="text-sm font-medium">System Status</span>
                            <a href="/api/v1/status" class="status-indicator status-success">
                                <span class="status-dot"></span>
                                Active
                            </a>
                        </div>
                        <div class="flex justify-between items-center py-2">
                            <span class="text-sm font-medium">Metrics</span>
                            <a href="/api/v1/metrics" class="status-indicator status-success">
                                <span class="status-dot"></span>
                                Active
                            </a>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        // Auto-refresh system stats
        async function updateStats() {
            try {
                const response = await fetch('/api/v1/metrics');
                const data = await response.json();
                
                if (data.success && data.data) {
                    const metrics = data.data;
                    
                    // Update stats cards
                    if (metrics.documents_processed_total !== undefined) {
                        document.getElementById('total-documents').textContent = metrics.documents_processed_total;
                    }
                    if (metrics.analysis_requests_total !== undefined) {
                        document.getElementById('total-analyses').textContent = metrics.analysis_requests_total;
                    }
                    
                    // Update system overview
                    if (metrics.analysis_requests_active !== undefined) {
                        document.getElementById('queue-size').textContent = metrics.analysis_requests_active;
                    }
                    if (metrics.avg_processing_time_seconds !== undefined) {
                        const avgTime = metrics.avg_processing_time_seconds;
                        document.getElementById('avg-time').textContent = 
                            avgTime < 1 ? '< 1s' : avgTime.toFixed(1) + 's';
                    }
                    
                    // Determine system load
                    const load = metrics.analysis_requests_active > 5 ? 'High' : 
                                metrics.analysis_requests_active > 2 ? 'Medium' : 'Low';
                    document.getElementById('system-load').textContent = load;
                }
            } catch (error) {
                console.error('Failed to update stats:', error);
            }
        }

        // Update stats on page load and every 30 seconds
        updateStats();
        setInterval(updateStats, 30000);
    </script>
</body>
</html>`, stats.TotalDocuments, stats.TotalAnalyses, stats.ActiveWorkers, stats.Status)
}

func (s *Server) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Upload Document - IDAES</title>
    <link rel="stylesheet" href="/static/styles.css">
</head>
<body>
    <div class="header">
        <div class="container">
            <h1>Upload Document</h1>
            <p class="subtitle">Upload and analyze documents with AI-powered extraction</p>
        </div>
    </div>

    <div class="container">
        <nav class="nav">
            <div class="nav-container">
                <div class="nav-links">
                    <a href="/">Home</a>
                    <a href="/upload" class="active">Upload</a>
                    <a href="/search">Search</a>
                    <a href="/dashboard">Dashboard</a>
                    <a href="/performance">Performance</a>
                    <a href="/results">Results</a>
                </div>
            </div>
        </nav>

        <div class="grid grid-cols-1">
            <!-- Upload Area -->
            <div class="card mb-6">
                <div class="card-header">
                    <h3>📄 Select Document</h3>
                </div>
                <div class="card-body">
                    <form id="uploadForm" enctype="multipart/form-data">
                        <div class="form-group">
                            <div id="dropZone" class="form-file" onclick="document.getElementById('fileInput').click()">
                                <div id="dropZoneContent">
                                    <div style="font-size: 3rem; margin-bottom: 1rem; opacity: 0.6;">📁</div>
                                    <h4 class="mb-2">Drop files here or click to browse</h4>
                                    <p class="text-sm text-gray-600 mb-4">
                                        Supported formats: PDF, DOC, DOCX, TXT (Max: 10MB)
                                    </p>
                                    <div class="btn btn-secondary">
                                        Choose File
                                    </div>
                                </div>
                            </div>
                            <input type="file" id="fileInput" name="file" 
                                   accept=".pdf,.doc,.docx,.txt" 
                                   style="display: none;" 
                                   onchange="handleFileSelect(this.files[0])">
                        </div>

                        <!-- File Info -->
                        <div id="fileInfo" class="hidden mb-4">
                            <div class="card" style="background-color: var(--gray-50); border: 1px solid var(--gray-200);">
                                <div class="card-body p-4">
                                    <div class="flex items-center justify-between">
                                        <div>
                                            <h5 id="fileName" class="mb-1"></h5>
                                            <p id="fileDetails" class="text-sm text-gray-600 mb-0"></p>
                                        </div>
                                        <button type="button" onclick="clearFile()" class="btn btn-sm btn-error">
                                            Remove
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Upload Options -->
                        <div id="uploadOptions" class="hidden">
                            <div class="grid grid-cols-2 mb-4">
                                <div class="form-group">
                                    <label class="form-label">Analysis Type</label>
                                    <select id="analysisType" class="form-input">
                                        <option value="full">Full Analysis (Entities + Citations + Topics)</option>
                                        <option value="entities">Entity Extraction Only</option>
                                        <option value="citations">Citation Analysis Only</option>
                                        <option value="topics">Topic Modeling Only</option>
                                    </select>
                                </div>
                                <div class="form-group">
                                    <label class="form-label">Priority</label>
                                    <select id="priority" class="form-input">
                                        <option value="normal">Normal</option>
                                        <option value="high">High Priority</option>
                                        <option value="low">Low Priority</option>
                                    </select>
                                </div>
                            </div>

                            <div class="form-group">
                                <button type="submit" id="uploadBtn" class="btn btn-primary btn-lg w-full">
                                    🚀 Upload and Analyze
                                </button>
                            </div>
                        </div>
                    </form>
                </div>
            </div>

            <!-- Progress Section -->
            <div id="progressSection" class="hidden">
                <div class="card mb-6">
                    <div class="card-header">
                        <h3>📊 Upload Progress</h3>
                    </div>
                    <div class="card-body">
                        <div class="mb-4">
                            <div class="flex justify-between items-center mb-2">
                                <span class="text-sm font-medium">Upload Progress</span>
                                <span id="uploadProgress" class="text-sm text-gray-600">0%</span>
                            </div>
                            <div class="progress">
                                <div id="uploadProgressBar" class="progress-bar" style="width: 0%"></div>
                            </div>
                        </div>

                        <div id="analysisProgress" class="hidden">
                            <div class="flex justify-between items-center mb-2">
                                <span class="text-sm font-medium">Analysis Progress</span>
                                <span id="analysisStatus" class="text-sm text-gray-600">Initializing...</span>
                            </div>
                            <div class="progress">
                                <div id="analysisProgressBar" class="progress-bar" style="width: 0%"></div>
                            </div>
                        </div>

                        <div id="statusMessages" class="mt-4">
                            <!-- Status messages will appear here -->
                        </div>
                    </div>
                </div>
            </div>

            <!-- Results Section -->
            <div id="resultsSection" class="hidden">
                <div class="card">
                    <div class="card-header">
                        <h3>✅ Analysis Complete</h3>
                    </div>
                    <div class="card-body">
                        <div class="alert alert-success mb-4">
                            <strong>Success!</strong> Your document has been processed successfully.
                        </div>
                        
                        <div id="analysisResults">
                            <!-- Analysis results will be populated here -->
                        </div>

                        <div class="card-footer text-center">
                            <a id="viewResultsBtn" href="#" class="btn btn-primary mr-4">
                                📊 View Detailed Results
                            </a>
                            <button onclick="resetUpload()" class="btn btn-secondary">
                                📄 Upload Another Document
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Error Section -->
            <div id="errorSection" class="hidden">
                <div class="card">
                    <div class="card-body">
                        <div class="alert alert-error">
                            <strong>Error:</strong> <span id="errorMessage"></span>
                        </div>
                        <div class="text-center">
                            <button onclick="resetUpload()" class="btn btn-secondary">
                                Try Again
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let currentFile = null;
        let uploadXHR = null;

        // Drag and drop handling
        const dropZone = document.getElementById('dropZone');

        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            dropZone.addEventListener(eventName, preventDefaults, false);
        });

        function preventDefaults(e) {
            e.preventDefault();
            e.stopPropagation();
        }

        ['dragenter', 'dragover'].forEach(eventName => {
            dropZone.addEventListener(eventName, highlight, false);
        });

        ['dragleave', 'drop'].forEach(eventName => {
            dropZone.addEventListener(eventName, unhighlight, false);
        });

        function highlight(e) {
            dropZone.style.backgroundColor = 'var(--primary-color)';
            dropZone.style.color = 'white';
            dropZone.style.borderColor = 'var(--primary-dark)';
        }

        function unhighlight(e) {
            dropZone.style.backgroundColor = '';
            dropZone.style.color = '';
            dropZone.style.borderColor = '';
        }

        dropZone.addEventListener('drop', handleDrop, false);

        function handleDrop(e) {
            const dt = e.dataTransfer;
            const files = dt.files;
            
            if (files.length > 0) {
                handleFileSelect(files[0]);
            }
        }

        function handleFileSelect(file) {
            if (!file) return;

            // Validate file type
            const allowedTypes = ['.pdf', '.doc', '.docx', '.txt'];
            const fileExtension = '.' + file.name.split('.').pop().toLowerCase();
            
            if (!allowedTypes.includes(fileExtension)) {
                alert('Please select a supported file type: PDF, DOC, DOCX, or TXT');
                return;
            }

            // Validate file size (10MB limit)
            if (file.size > 10 * 1024 * 1024) {
                alert('File size must be less than 10MB');
                return;
            }

            currentFile = file;
            
            // Update UI
            document.getElementById('fileName').textContent = file.name;
            document.getElementById('fileDetails').textContent = 
                formatFileSize(file.size) + ' • ' + fileExtension.toUpperCase() + ' • Modified: ' + 
                new Date(file.lastModified).toLocaleDateString();
            
            document.getElementById('fileInfo').classList.remove('hidden');
            document.getElementById('uploadOptions').classList.remove('hidden');
            
            // Update drop zone
            document.getElementById('dropZoneContent').innerHTML = 
                '<div style="font-size: 2rem; margin-bottom: 1rem; color: var(--success-color);">✅</div>' +
                '<h4 class="mb-2">File Ready for Upload</h4>' +
                '<p class="text-sm text-gray-600">Click "Upload and Analyze" to proceed</p>';
        }

        function clearFile() {
            currentFile = null;
            document.getElementById('fileInput').value = '';
            document.getElementById('fileInfo').classList.add('hidden');
            document.getElementById('uploadOptions').classList.add('hidden');
            
            // Reset drop zone
            document.getElementById('dropZoneContent').innerHTML = 
                '<div style="font-size: 3rem; margin-bottom: 1rem; opacity: 0.6;">📁</div>' +
                '<h4 class="mb-2">Drop files here or click to browse</h4>' +
                '<p class="text-sm text-gray-600 mb-4">Supported formats: PDF, DOC, DOCX, TXT (Max: 10MB)</p>' +
                '<div class="btn btn-secondary">Choose File</div>';
        }

        // Form submission
        document.getElementById('uploadForm').addEventListener('submit', function(e) {
            e.preventDefault();
            
            if (!currentFile) {
                alert('Please select a file first');
                return;
            }

            uploadFile();
        });

        function uploadFile() {
            const formData = new FormData();
            formData.append('file', currentFile);
            formData.append('analysis_type', document.getElementById('analysisType').value);
            formData.append('priority', document.getElementById('priority').value);

            // Show progress section
            document.getElementById('progressSection').classList.remove('hidden');
            hideAllSections(['resultsSection', 'errorSection']);

            uploadXHR = new XMLHttpRequest();

            // Upload progress
            uploadXHR.upload.addEventListener('progress', function(e) {
                if (e.lengthComputable) {
                    const percentComplete = (e.loaded / e.total) * 100;
                    updateUploadProgress(percentComplete);
                }
            });

            uploadXHR.addEventListener('load', function() {
                if (uploadXHR.status === 200) {
                    try {
                        const response = JSON.parse(uploadXHR.responseText);
                        if (response.success) {
                            handleUploadSuccess(response.data);
                        } else {
                            handleUploadError(response.error || 'Upload failed');
                        }
                    } catch (e) {
                        handleUploadError('Invalid response from server');
                    }
                } else {
                    handleUploadError('Upload failed with status: ' + uploadXHR.status);
                }
            });

            uploadXHR.addEventListener('error', function() {
                handleUploadError('Network error during upload');
            });

            uploadXHR.open('POST', '/api/v1/documents/upload');
            uploadXHR.send(formData);

            addStatusMessage('Starting upload...', 'info');
        }

        function updateUploadProgress(percent) {
            document.getElementById('uploadProgress').textContent = Math.round(percent) + '%';
            document.getElementById('uploadProgressBar').style.width = percent + '%';
            
            if (percent >= 100) {
                addStatusMessage('Upload complete, starting analysis...', 'success');
                document.getElementById('analysisProgress').classList.remove('hidden');
                simulateAnalysisProgress();
            }
        }

        function simulateAnalysisProgress() {
            let progress = 0;
            const interval = setInterval(() => {
                progress += Math.random() * 15;
                if (progress > 100) progress = 100;
                
                document.getElementById('analysisProgressBar').style.width = progress + '%';
                
                if (progress < 30) {
                    document.getElementById('analysisStatus').textContent = 'Extracting text...';
                } else if (progress < 60) {
                    document.getElementById('analysisStatus').textContent = 'Analyzing entities...';
                } else if (progress < 90) {
                    document.getElementById('analysisStatus').textContent = 'Processing citations...';
                } else {
                    document.getElementById('analysisStatus').textContent = 'Finalizing results...';
                }
                
                if (progress >= 100) {
                    clearInterval(interval);
                    document.getElementById('analysisStatus').textContent = 'Complete!';
                }
            }, 500);
        }

        function handleUploadSuccess(data) {
            addStatusMessage('Analysis completed successfully!', 'success');
            
            // Populate results
            let resultsHTML = '<div class="grid grid-cols-3 mb-4">';
            
            if (data.entities && data.entities.length > 0) {
                resultsHTML += '<div class="metric"><span class="metric-label">Entities Extracted</span>';
                resultsHTML += '<span class="metric-value">' + data.entities.length + '</span></div>';
            }
            
            if (data.citations && data.citations.length > 0) {
                resultsHTML += '<div class="metric"><span class="metric-label">Citations Found</span>';
                resultsHTML += '<span class="metric-value">' + data.citations.length + '</span></div>';
            }
            
            if (data.topics && data.topics.length > 0) {
                resultsHTML += '<div class="metric"><span class="metric-label">Topics Identified</span>';
                resultsHTML += '<span class="metric-value">' + data.topics.length + '</span></div>';
            }
            
            resultsHTML += '</div>';
            
            document.getElementById('analysisResults').innerHTML = resultsHTML;
            
            if (data.document_id) {
                document.getElementById('viewResultsBtn').href = '/results/' + data.document_id;
            }
            
            hideAllSections(['progressSection']);
            document.getElementById('resultsSection').classList.remove('hidden');
        }

        function handleUploadError(error) {
            document.getElementById('errorMessage').textContent = error;
            hideAllSections(['progressSection']);
            document.getElementById('errorSection').classList.remove('hidden');
            addStatusMessage('Error: ' + error, 'error');
        }

        function addStatusMessage(message, type) {
            const statusDiv = document.createElement('div');
            statusDiv.className = 'alert alert-' + type + ' text-sm mb-2';
            statusDiv.textContent = new Date().toLocaleTimeString() + ': ' + message;
            document.getElementById('statusMessages').appendChild(statusDiv);
        }

        function hideAllSections(except = []) {
            const sections = ['progressSection', 'resultsSection', 'errorSection'];
            sections.forEach(section => {
                if (!except.includes(section)) {
                    document.getElementById(section).classList.add('hidden');
                }
            });
        }

        function resetUpload() {
            clearFile();
            hideAllSections();
            document.getElementById('statusMessages').innerHTML = '';
            
            if (uploadXHR) {
                uploadXHR.abort();
                uploadXHR = null;
            }
        }

        function formatFileSize(bytes) {
            if (bytes === 0) return '0 Bytes';
            const k = 1024;
            const sizes = ['Bytes', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }
    </script>
</body>
</html>`)
}

func (s *Server) handleResultsPage(w http.ResponseWriter, r *http.Request) {
	// Extract document ID from URL path if present
	documentID := ""
	if len(r.URL.Path) > len("/results/") {
		documentID = r.URL.Path[len("/results/"):]
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Analysis Results - IDAES</title>
    <link rel="stylesheet" href="/static/styles.css">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="header">
        <div class="container">
            <h1>Analysis Results</h1>
            <p class="subtitle">Document analysis insights and extracted information</p>
        </div>
    </div>

    <div class="container">
        <nav class="nav">
            <div class="nav-container">
                <div class="nav-links">
                    <a href="/">Home</a>
                    <a href="/upload">Upload</a>
                    <a href="/search">Search</a>
                    <a href="/dashboard">Dashboard</a>
                    <a href="/performance">Performance</a>
                    <a href="/results" class="active">Results</a>
                </div>
            </div>
        </nav>

        <!-- Document Selector -->
        <div class="card mb-6" id="documentSelector">
            <div class="card-header">
                <h3>📄 Select Document</h3>
            </div>
            <div class="card-body">
                <div class="grid grid-cols-1">
                    <div class="form-group">
                        <label class="form-label">Choose a document to view results:</label>
                        <select id="documentSelect" class="form-input" onchange="loadDocumentResults()">
                            <option value="">-- Select a document --</option>
                            <!-- Options will be populated by JavaScript -->
                        </select>
                    </div>
                </div>
                <div class="text-center mt-4">
                    <button onclick="loadSampleResults()" class="btn btn-secondary">
                        📊 Load Sample Results
                    </button>
                </div>
            </div>
        </div>

        <!-- Results Content -->
        <div id="resultsContent" class="hidden">
            <!-- Document Summary -->
            <div class="card mb-6">
                <div class="card-header">
                    <h3 id="documentTitle">Document Summary</h3>
                </div>
                <div class="card-body">
                    <div class="grid grid-cols-4 mb-4">
                        <div class="metric">
                            <span class="metric-label">File Name</span>
                            <span class="metric-value text-sm" id="fileName">--</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">File Size</span>
                            <span class="metric-value text-sm" id="fileSize">--</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">Processing Time</span>
                            <span class="metric-value text-sm" id="processingTime">--</span>
                        </div>
                        <div class="metric">
                            <span class="metric-label">Analysis Date</span>
                            <span class="metric-value text-sm" id="analysisDate">--</span>
                        </div>
                    </div>
                    
                    <div id="documentContent" class="bg-gray-50 p-4 rounded-lg">
                        <h5 class="mb-2">Document Preview:</h5>
                        <p id="contentPreview" class="text-sm text-gray-700">Loading...</p>
                    </div>
                </div>
            </div>

            <!-- Analysis Overview -->
            <div class="grid grid-cols-3 mb-6">
                <div class="stats-card">
                    <h3>Entities Extracted</h3>
                    <div class="value" id="entityCount">0</div>
                </div>
                <div class="stats-card">
                    <h3>Citations Found</h3>
                    <div class="value" id="citationCount">0</div>
                </div>
                <div class="stats-card">
                    <h3>Topics Identified</h3>
                    <div class="value" id="topicCount">0</div>
                </div>
            </div>

            <!-- Detailed Results -->
            <div class="grid grid-cols-2 mb-6">
                <!-- Entity Analysis -->
                <div class="card">
                    <div class="card-header">
                        <h3>🏷️ Extracted Entities</h3>
                    </div>
                    <div class="card-body">
                        <div class="mb-4">
                            <canvas id="entityChart" width="400" height="200"></canvas>
                        </div>
                        <div id="entityList" class="space-y-2">
                            <!-- Entity list will be populated here -->
                        </div>
                    </div>
                </div>

                <!-- Citation Analysis -->
                <div class="card">
                    <div class="card-header">
                        <h3>📚 Citations</h3>
                    </div>
                    <div class="card-body">
                        <div id="citationList" class="space-y-3">
                            <!-- Citation list will be populated here -->
                        </div>
                    </div>
                </div>
            </div>

            <!-- Topic Analysis -->
            <div class="card mb-6">
                <div class="card-header">
                    <h3>🎯 Topic Analysis</h3>
                </div>
                <div class="card-body">
                    <div class="grid grid-cols-2">
                        <div>
                            <canvas id="topicChart" width="400" height="200"></canvas>
                        </div>
                        <div id="topicDetails">
                            <!-- Topic details will be populated here -->
                        </div>
                    </div>
                </div>
            </div>

            <!-- Advanced Analytics -->
            <div class="grid grid-cols-2">
                <!-- Confidence Scores -->
                <div class="card">
                    <div class="card-header">
                        <h3>📈 Confidence Scores</h3>
                    </div>
                    <div class="card-body">
                        <div id="confidenceMetrics">
                            <div class="metric mb-3">
                                <span class="metric-label">Entity Extraction Confidence</span>
                                <div class="progress">
                                    <div id="entityConfidence" class="progress-bar" style="width: 0%%"></div>
                                </div>
                                <span class="text-sm text-gray-600" id="entityConfidenceText">0%%</span>
                            </div>
                            <div class="metric mb-3">
                                <span class="metric-label">Citation Detection Confidence</span>
                                <div class="progress">
                                    <div id="citationConfidence" class="progress-bar" style="width: 0%%"></div>
                                </div>
                                <span class="text-sm text-gray-600" id="citationConfidenceText">0%%</span>
                            </div>
                            <div class="metric">
                                <span class="metric-label">Topic Modeling Confidence</span>
                                <div class="progress">
                                    <div id="topicConfidence" class="progress-bar" style="width: 0%%"></div>
                                </div>
                                <span class="text-sm text-gray-600" id="topicConfidenceText">0%%</span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Processing Timeline -->
                <div class="card">
                    <div class="card-header">
                        <h3>⏱️ Processing Timeline</h3>
                    </div>
                    <div class="card-body">
                        <div id="timeline" class="space-y-3">
                            <!-- Timeline will be populated here -->
                        </div>
                    </div>
                </div>
            </div>

            <!-- Export Options -->
            <div class="card mt-6">
                <div class="card-header">
                    <h3>💾 Export Results</h3>
                </div>
                <div class="card-body">
                    <div class="grid grid-cols-4 gap-4">
                        <button onclick="exportResults('json')" class="btn btn-secondary">
                            📄 Export JSON
                        </button>
                        <button onclick="exportResults('csv')" class="btn btn-secondary">
                            📊 Export CSV
                        </button>
                        <button onclick="exportResults('pdf')" class="btn btn-secondary">
                            📋 Export PDF
                        </button>
                        <button onclick="shareResults()" class="btn btn-primary">
                            🔗 Share Results
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Loading State -->
        <div id="loadingState" class="hidden">
            <div class="card">
                <div class="card-body text-center py-8">
                    <div class="spin mb-4" style="width: 40px; height: 40px; border: 4px solid var(--gray-200); border-top: 4px solid var(--primary-color); border-radius: 50%%; margin: 0 auto;"></div>
                    <p class="text-gray-600">Loading analysis results...</p>
                </div>
            </div>
        </div>
    </div>

    <script>
        let currentResults = null;
        let entityChart = null;
        let topicChart = null;

        // Document ID from URL
        const documentID = '%s';

        // Initialize page
        document.addEventListener('DOMContentLoaded', function() {
            loadDocumentList();
            if (documentID) {
                loadDocumentResults(documentID);
            }
        });

        async function loadDocumentList() {
            try {
                // This would normally fetch from an API
                const select = document.getElementById('documentSelect');
                select.innerHTML = '<option value="">-- Select a document --</option>';
                
                // Add some sample documents for demo
                const sampleDocs = [
                    { id: 'doc1', name: 'Research Paper - AI Ethics.pdf' },
                    { id: 'doc2', name: 'Financial Report Q3 2024.docx' },
                    { id: 'doc3', name: 'Meeting Notes - Project Alpha.txt' }
                ];
                
                sampleDocs.forEach(doc => {
                    const option = document.createElement('option');
                    option.value = doc.id;
                    option.textContent = doc.name;
                    select.appendChild(option);
                });
                
            } catch (error) {
                console.error('Failed to load document list:', error);
            }
        }

        async function loadDocumentResults(docId) {
            if (!docId && !documentID) {
                const selectedDoc = document.getElementById('documentSelect').value;
                if (!selectedDoc) return;
                docId = selectedDoc;
            }

            showLoading();

            try {
                // This would normally fetch real results from the API
                // For now, we'll simulate with sample data
                setTimeout(() => {
                    loadSampleResults();
                }, 1000);
                
            } catch (error) {
                console.error('Failed to load results:', error);
                hideLoading();
            }
        }

        function loadSampleResults() {
            currentResults = {
                document: {
                    id: 'sample-doc',
                    filename: 'Sample Research Paper.pdf',
                    size: '2.5 MB',
                    processing_time: '3.2s',
                    analysis_date: new Date().toLocaleDateString(),
                    content_preview: 'This research paper discusses the implications of artificial intelligence in modern healthcare systems. The study examines various machine learning algorithms and their applications in diagnostic medicine, highlighting both opportunities and challenges...'
                },
                entities: [
                    { text: 'Artificial Intelligence', type: 'TECHNOLOGY', confidence: 0.95 },
                    { text: 'Healthcare', type: 'INDUSTRY', confidence: 0.92 },
                    { text: 'Machine Learning', type: 'TECHNOLOGY', confidence: 0.89 },
                    { text: 'Dr. Sarah Johnson', type: 'PERSON', confidence: 0.87 },
                    { text: 'Stanford University', type: 'ORGANIZATION', confidence: 0.94 },
                    { text: 'California', type: 'LOCATION', confidence: 0.91 }
                ],
                citations: [
                    { text: 'Smith, J. et al. (2023). "Machine Learning in Healthcare"', authors: 'Smith, J., Brown, A.', year: 2023 },
                    { text: 'Johnson, S. (2022). "AI Ethics Framework"', authors: 'Johnson, S.', year: 2022 },
                    { text: 'Data Science Journal, Vol. 15', authors: 'Various', year: 2023 }
                ],
                topics: [
                    { name: 'Artificial Intelligence', weight: 0.35, keywords: ['AI', 'machine learning', 'algorithms'] },
                    { name: 'Healthcare Technology', weight: 0.28, keywords: ['medical', 'diagnosis', 'patient care'] },
                    { name: 'Research Methodology', weight: 0.22, keywords: ['study', 'analysis', 'methodology'] },
                    { name: 'Ethics', weight: 0.15, keywords: ['ethics', 'responsibility', 'governance'] }
                ],
                confidence: {
                    entity_extraction: 0.91,
                    citation_detection: 0.88,
                    topic_modeling: 0.85
                },
                timeline: [
                    { stage: 'Document Upload', time: '0.1s', status: 'completed' },
                    { stage: 'Text Extraction', time: '0.5s', status: 'completed' },
                    { stage: 'Entity Recognition', time: '1.2s', status: 'completed' },
                    { stage: 'Citation Analysis', time: '0.8s', status: 'completed' },
                    { stage: 'Topic Modeling', time: '0.6s', status: 'completed' }
                ]
            };

            displayResults(currentResults);
            hideLoading();
        }

        function displayResults(results) {
            // Update document summary
            document.getElementById('fileName').textContent = results.document.filename;
            document.getElementById('fileSize').textContent = results.document.size;
            document.getElementById('processingTime').textContent = results.document.processing_time;
            document.getElementById('analysisDate').textContent = results.document.analysis_date;
            document.getElementById('contentPreview').textContent = results.document.content_preview;

            // Update overview stats
            document.getElementById('entityCount').textContent = results.entities.length;
            document.getElementById('citationCount').textContent = results.citations.length;
            document.getElementById('topicCount').textContent = results.topics.length;

            // Display entities
            displayEntities(results.entities);
            
            // Display citations
            displayCitations(results.citations);
            
            // Display topics
            displayTopics(results.topics);
            
            // Display confidence scores
            displayConfidenceScores(results.confidence);
            
            // Display timeline
            displayTimeline(results.timeline);

            // Show results
            document.getElementById('documentSelector').classList.add('hidden');
            document.getElementById('resultsContent').classList.remove('hidden');
        }

        function displayEntities(entities) {
            const container = document.getElementById('entityList');
            container.innerHTML = '';

            // Group entities by type
            const entityTypes = {};
            entities.forEach(entity => {
                if (!entityTypes[entity.type]) {
                    entityTypes[entity.type] = [];
                }
                entityTypes[entity.type].push(entity);
            });

            // Create entity chart
            createEntityChart(entityTypes);

            // Display entity list
            Object.keys(entityTypes).forEach(type => {
                const typeDiv = document.createElement('div');
                typeDiv.innerHTML = '<h5 class="font-medium mb-2">' + type + '</h5>';
                
                entityTypes[type].forEach(entity => {
                    const entityDiv = document.createElement('div');
                    entityDiv.className = 'flex justify-between items-center py-1';
                    entityDiv.innerHTML = 
                        '<span class="text-sm">' + entity.text + '</span>' +
                        '<span class="status-indicator status-success text-xs">Confidence: ' + 
                        (entity.confidence * 100).toFixed(0) + '%%</span>';
                    typeDiv.appendChild(entityDiv);
                });
                
                container.appendChild(typeDiv);
            });
        }

        function displayCitations(citations) {
            const container = document.getElementById('citationList');
            container.innerHTML = '';

            citations.forEach(citation => {
                const citationDiv = document.createElement('div');
                citationDiv.className = 'p-3 bg-gray-50 rounded-lg';
                citationDiv.innerHTML = 
                    '<p class="text-sm font-medium mb-1">' + citation.text + '</p>' +
                    '<p class="text-xs text-gray-600">Authors: ' + citation.authors + ' • Year: ' + citation.year + '</p>';
                container.appendChild(citationDiv);
            });
        }

        function displayTopics(topics) {
            createTopicChart(topics);
            
            const container = document.getElementById('topicDetails');
            container.innerHTML = '';

            topics.forEach(topic => {
                const topicDiv = document.createElement('div');
                topicDiv.className = 'mb-4';
                topicDiv.innerHTML = 
                    '<h5 class="font-medium mb-2">' + topic.name + '</h5>' +
                    '<div class="progress mb-2"><div class="progress-bar" style="width: ' + (topic.weight * 100) + '%%"></div></div>' +
                    '<p class="text-sm text-gray-600">Keywords: ' + topic.keywords.join(', ') + '</p>';
                container.appendChild(topicDiv);
            });
        }

        function displayConfidenceScores(confidence) {
            Object.keys(confidence).forEach(key => {
                const value = confidence[key] * 100;
                const elementId = key.replace('_', '') + 'Confidence';
                const textId = key.replace('_', '') + 'ConfidenceText';
                
                if (document.getElementById(elementId)) {
                    document.getElementById(elementId).style.width = value + '%%';
                    document.getElementById(textId).textContent = value.toFixed(0) + '%%';
                }
            });
        }

        function displayTimeline(timeline) {
            const container = document.getElementById('timeline');
            container.innerHTML = '';

            timeline.forEach(stage => {
                const stageDiv = document.createElement('div');
                stageDiv.className = 'flex justify-between items-center py-2';
                stageDiv.innerHTML = 
                    '<span class="text-sm font-medium">' + stage.stage + '</span>' +
                    '<div class="flex items-center gap-2">' +
                    '<span class="text-xs text-gray-600">' + stage.time + '</span>' +
                    '<span class="status-indicator status-success"><span class="status-dot"></span></span>' +
                    '</div>';
                container.appendChild(stageDiv);
            });
        }

        function createEntityChart(entityTypes) {
            const ctx = document.getElementById('entityChart').getContext('2d');
            
            if (entityChart) {
                entityChart.destroy();
            }

            const labels = Object.keys(entityTypes);
            const data = labels.map(type => entityTypes[type].length);

            entityChart = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        backgroundColor: [
                            '#007cba', '#667eea', '#4CAF50', '#FF9800', '#f44336', '#2196F3'
                        ]
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            position: 'bottom'
                        }
                    }
                }
            });
        }

        function createTopicChart(topics) {
            const ctx = document.getElementById('topicChart').getContext('2d');
            
            if (topicChart) {
                topicChart.destroy();
            }

            topicChart = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: topics.map(t => t.name),
                    datasets: [{
                        label: 'Topic Weight',
                        data: topics.map(t => t.weight),
                        backgroundColor: '#007cba'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true,
                            max: 1
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });
        }

        function showLoading() {
            document.getElementById('loadingState').classList.remove('hidden');
            document.getElementById('resultsContent').classList.add('hidden');
            document.getElementById('documentSelector').classList.add('hidden');
        }

        function hideLoading() {
            document.getElementById('loadingState').classList.add('hidden');
        }

        function exportResults(format) {
            if (!currentResults) {
                alert('No results to export');
                return;
            }

            // This would normally trigger a download
            alert('Export to ' + format.toUpperCase() + ' would be triggered here');
        }

        function shareResults() {
            if (!currentResults) {
                alert('No results to share');
                return;
            }

            // This would normally generate a shareable link
            alert('Shareable link would be generated here');
        }

        function goBack() {
            document.getElementById('resultsContent').classList.add('hidden');
            document.getElementById('documentSelector').classList.remove('hidden');
        }
    </script>
</body>
</html>`, documentID)
}

func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Search - IDAES</title>
    <link rel="stylesheet" href="/static/styles.css">
</head>
<body>
    <div class="header">
        <div class="container">
            <h1>Search Documents</h1>
            <p class="subtitle">Find documents, entities, and insights across your corpus</p>
        </div>
    </div>

    <div class="container">
        <nav class="nav">
            <div class="nav-container">
                <div class="nav-links">
                    <a href="/">Home</a>
                    <a href="/upload">Upload</a>
                    <a href="/search" class="active">Search</a>
                    <a href="/dashboard">Dashboard</a>
                    <a href="/performance">Performance</a>
                    <a href="/results">Results</a>
                </div>
            </div>
        </nav>

        <!-- Search Interface -->
        <div class="card mb-6">
            <div class="card-body">
                <div class="grid grid-cols-1 mb-6">
                    <div class="form-group">
                        <label class="form-label">Search Query</label>
                        <input type="text" id="searchQuery" class="form-input" 
                               placeholder="Enter your search query (e.g., 'artificial intelligence research', 'John Doe', 'medical devices')"
                               onkeypress="handleSearchKeyPress(event)">
                    </div>
                </div>

                <div class="grid grid-cols-4 mb-6">
                    <div class="form-group">
                        <label class="form-label">Search Type</label>
                        <select id="searchType" class="form-input">
                            <option value="semantic">Semantic Search</option>
                            <option value="documents">Document Search</option>
                            <option value="entities">Entity Search</option>
                            <option value="citations">Citation Search</option>
                            <option value="topics">Topic Search</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Results Limit</label>
                        <select id="resultLimit" class="form-input">
                            <option value="10">10 Results</option>
                            <option value="25" selected>25 Results</option>
                            <option value="50">50 Results</option>
                            <option value="100">100 Results</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">Sort By</label>
                        <select id="sortBy" class="form-input">
                            <option value="relevance">Relevance</option>
                            <option value="date">Date</option>
                            <option value="title">Title</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label class="form-label">&nbsp;</label>
                        <button onclick="performSearch()" class="btn btn-primary w-full">
                            🔍 Search
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Search Results -->
        <div id="searchResults" class="hidden">
            <div class="card">
                <div class="card-header">
                    <h3>Search Results</h3>
                    <div id="resultsInfo" class="text-sm text-gray-600"></div>
                </div>
                <div class="card-body">
                    <div id="resultsContainer">
                        <!-- Results will be populated here -->
                    </div>
                </div>
            </div>
        </div>

        <!-- Loading State -->
        <div id="loadingState" class="hidden">
            <div class="card">
                <div class="card-body text-center py-8">
                    <div class="spin mb-4" style="width: 40px; height: 40px; border: 4px solid var(--gray-200); border-top: 4px solid var(--primary-color); border-radius: 50%; margin: 0 auto;"></div>
                    <p class="text-gray-600">Searching documents...</p>
                </div>
            </div>
        </div>

        <!-- No Results State -->
        <div id="noResults" class="hidden">
            <div class="card">
                <div class="card-body text-center py-8">
                    <div class="mb-4" style="font-size: 4rem; opacity: 0.5;">🔍</div>
                    <h3 class="mb-2">No Results Found</h3>
                    <p class="text-gray-600 mb-4">Try adjusting your search query or search type</p>
                    <div class="grid grid-cols-3 gap-4">
                        <div class="text-center p-4">
                            <h4 class="mb-2">Try Semantic Search</h4>
                            <p class="text-sm text-gray-600">Use natural language queries</p>
                        </div>
                        <div class="text-center p-4">
                            <h4 class="mb-2">Search by Entity</h4>
                            <p class="text-sm text-gray-600">Look for specific people, places, or organizations</p>
                        </div>
                        <div class="text-center p-4">
                            <h4 class="mb-2">Browse by Topic</h4>
                            <p class="text-sm text-gray-600">Explore documents by subject area</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Search Examples -->
        <div class="card mt-6">
            <div class="card-header">
                <h3>💡 Search Examples</h3>
            </div>
            <div class="card-body">
                <div class="grid grid-cols-2">
                    <div>
                        <h4 class="mb-3">Semantic Search Examples:</h4>
                        <ul class="space-y-2">
                            <li><button onclick="setSearchQuery('machine learning algorithms')" class="text-left text-primary-color hover:underline">machine learning algorithms</button></li>
                            <li><button onclick="setSearchQuery('environmental impact assessment')" class="text-left text-primary-color hover:underline">environmental impact assessment</button></li>
                            <li><button onclick="setSearchQuery('financial market analysis')" class="text-left text-primary-color hover:underline">financial market analysis</button></li>
                        </ul>
                    </div>
                    <div>
                        <h4 class="mb-3">Entity Search Examples:</h4>
                        <ul class="space-y-2">
                            <li><button onclick="setSearchQuery('John Smith', 'entities')" class="text-left text-primary-color hover:underline">John Smith (Person)</button></li>
                            <li><button onclick="setSearchQuery('Microsoft Corporation', 'entities')" class="text-left text-primary-color hover:underline">Microsoft Corporation (Organization)</button></li>
                            <li><button onclick="setSearchQuery('New York', 'entities')" class="text-left text-primary-color hover:underline">New York (Location)</button></li>
                        </ul>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        function handleSearchKeyPress(event) {
            if (event.key === 'Enter') {
                performSearch();
            }
        }

        function setSearchQuery(query, type = null) {
            document.getElementById('searchQuery').value = query;
            if (type) {
                document.getElementById('searchType').value = type;
            }
            performSearch();
        }

        async function performSearch() {
            const query = document.getElementById('searchQuery').value.trim();
            const searchType = document.getElementById('searchType').value;
            const limit = document.getElementById('resultLimit').value;
            const sortBy = document.getElementById('sortBy').value;

            if (!query) {
                alert('Please enter a search query');
                return;
            }

            // Show loading state
            document.getElementById('searchResults').classList.add('hidden');
            document.getElementById('noResults').classList.add('hidden');
            document.getElementById('loadingState').classList.remove('hidden');

            try {
                let endpoint;
                let body = { query, limit: parseInt(limit), sort_by: sortBy };

                switch (searchType) {
                    case 'semantic':
                        endpoint = '/api/v1/search/semantic';
                        break;
                    case 'documents':
                        endpoint = '/api/v1/search/documents';
                        break;
                    case 'entities':
                        endpoint = '/api/v1/search/entities';
                        break;
                    case 'citations':
                        endpoint = '/api/v1/search/citations';
                        break;
                    case 'topics':
                        endpoint = '/api/v1/search/topics';
                        break;
                    default:
                        endpoint = '/api/v1/search/semantic';
                }

                const response = await fetch(endpoint, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(body)
                });

                const data = await response.json();

                // Hide loading state
                document.getElementById('loadingState').classList.add('hidden');

                if (data.success && data.data && data.data.length > 0) {
                    displayResults(data.data, searchType);
                } else {
                    // Show no results
                    document.getElementById('noResults').classList.remove('hidden');
                }

            } catch (error) {
                console.error('Search error:', error);
                document.getElementById('loadingState').classList.add('hidden');
                
                // Show error message
                document.getElementById('resultsContainer').innerHTML = 
                    '<div class="alert alert-error">Search failed: ' + error.message + '</div>';
                document.getElementById('resultsInfo').textContent = 'Error occurred during search';
                document.getElementById('searchResults').classList.remove('hidden');
            }
        }

        function displayResults(results, searchType) {
            const container = document.getElementById('resultsContainer');
            const info = document.getElementById('resultsInfo');
            
            info.textContent = 'Found ' + results.length + ' results';
            
            let html = '';
            
            results.forEach((result, index) => {
                html += '<div class="mb-4 p-4 border border-gray-200 rounded-lg hover:bg-gray-50">';
                
                if (searchType === 'semantic' || searchType === 'documents') {
                    html += '<h4 class="mb-2"><a href="/results/' + (result.id || index) + '" class="text-primary-color hover:underline">';
                    html += (result.title || result.filename || 'Document ' + (index + 1)) + '</a></h4>';
                    
                    if (result.content) {
                        html += '<p class="text-gray-600 mb-2">' + truncateText(result.content, 200) + '</p>';
                    }
                    
                    if (result.similarity_score || result.relevance_score) {
                        const score = result.similarity_score || result.relevance_score;
                        html += '<div class="flex items-center justify-between">';
                        html += '<span class="status-indicator status-info">Relevance: ' + (score * 100).toFixed(1) + '%</span>';
                        if (result.date) {
                            html += '<span class="text-sm text-gray-500">' + new Date(result.date).toLocaleDateString() + '</span>';
                        }
                        html += '</div>';
                    }
                    
                } else if (searchType === 'entities') {
                    html += '<h4 class="mb-2">' + (result.entity_text || result.text) + '</h4>';
                    html += '<p class="text-sm text-gray-600 mb-2">Type: ' + (result.entity_type || result.type || 'Unknown') + '</p>';
                    if (result.confidence) {
                        html += '<span class="status-indicator status-success">Confidence: ' + (result.confidence * 100).toFixed(1) + '%</span>';
                    }
                    
                } else if (searchType === 'citations') {
                    html += '<h4 class="mb-2">' + (result.citation_text || result.text) + '</h4>';
                    if (result.authors) {
                        html += '<p class="text-sm text-gray-600 mb-1">Authors: ' + result.authors + '</p>';
                    }
                    if (result.year) {
                        html += '<p class="text-sm text-gray-600 mb-1">Year: ' + result.year + '</p>';
                    }
                    
                } else if (searchType === 'topics') {
                    html += '<h4 class="mb-2">' + (result.topic_name || result.name) + '</h4>';
                    if (result.keywords) {
                        html += '<p class="text-sm text-gray-600 mb-2">Keywords: ' + result.keywords.join(', ') + '</p>';
                    }
                    if (result.document_count) {
                        html += '<span class="status-indicator status-info">' + result.document_count + ' documents</span>';
                    }
                }
                
                html += '</div>';
            });
            
            container.innerHTML = html;
            document.getElementById('searchResults').classList.remove('hidden');
        }

        function truncateText(text, maxLength) {
            if (text.length <= maxLength) return text;
            return text.substr(0, maxLength) + '...';
        }
    </script>
</body>
</html>`)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Dashboard - IDAES</title>
    <link rel="stylesheet" href="/static/styles.css">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="header">
        <div class="container">
            <h1>System Dashboard</h1>
            <p class="subtitle">Real-time analytics and system monitoring</p>
        </div>
    </div>

    <div class="container">
        <nav class="nav">
            <div class="nav-container">
                <div class="nav-links">
                    <a href="/">Home</a>
                    <a href="/upload">Upload</a>
                    <a href="/search">Search</a>
                    <a href="/dashboard" class="active">Dashboard</a>
                    <a href="/performance">Performance</a>
                    <a href="/results">Results</a>
                </div>
                <div class="nav-links">
                    <div class="status-indicator" id="wsStatusIndicator">
                        <span class="status-dot" id="wsStatusDot"></span>
                        <span id="wsStatusText">Starting...</span>
                    </div>
                </div>
            </div>
        </nav>

        <!-- Real-time Metrics Overview -->
        <div class="grid grid-cols-4 mb-6">
            <div class="stats-card">
                <h3>📄 Total Documents</h3>
                <div class="value" id="totalDocuments">0</div>
                <div class="change" id="documentsChange">+0 today</div>
            </div>
            <div class="stats-card">
                <h3>⚡ Analyses Completed</h3>
                <div class="value" id="totalAnalyses">0</div>
                <div class="change" id="analysesChange">+0 today</div>
            </div>
            <div class="stats-card">
                <h3>🔄 Active Workers</h3>
                <div class="value" id="activeWorkers">0</div>
                <div class="change" id="workersChange">0 processing</div>
            </div>
            <div class="stats-card">
                <h3>⏱️ Avg Processing Time</h3>
                <div class="value" id="avgProcessingTime">0ms</div>
                <div class="change" id="timeChange">Real-time</div>
            </div>
        </div>

        <!-- Charts and Analytics -->
        <div class="grid grid-cols-2 mb-6">
            <!-- Processing Performance Chart -->
            <div class="card">
                <div class="card-header">
                    <h3>📊 Processing Performance</h3>
                    <div class="card-actions">
                        <button onclick="toggleChartType()" class="btn btn-sm btn-secondary" id="chartToggle">
                            Line View
                        </button>
                    </div>
                </div>
                <div class="card-body">
                    <canvas id="performanceChart" width="400" height="200"></canvas>
                </div>
            </div>

            <!-- System Health Chart -->
            <div class="card">
                <div class="card-header">
                    <h3>💻 System Health</h3>
                </div>
                <div class="card-body">
                    <canvas id="healthChart" width="400" height="200"></canvas>
                </div>
            </div>
        </div>

        <!-- Detailed Metrics -->
        <div class="grid grid-cols-3 mb-6">
            <!-- Orchestrator Metrics -->
            <div class="card">
                <div class="card-header">
                    <h3>🎯 Orchestrator Metrics</h3>
                </div>
                <div class="card-body">
                    <div class="metric">
                        <span class="metric-label">Total Requests</span>
                        <span class="metric-value" id="totalRequests">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Successful Analyses</span>
                        <span class="metric-value text-success" id="successfulAnalyses">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Failed Analyses</span>
                        <span class="metric-value text-error" id="failedAnalyses">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Success Rate</span>
                        <span class="metric-value" id="successRate">100%</span>
                    </div>
                </div>
            </div>

            <!-- Pipeline Metrics -->
            <div class="card">
                <div class="card-header">
                    <h3>🔄 Pipeline Metrics</h3>
                </div>
                <div class="card-body">
                    <div class="metric">
                        <span class="metric-label">Pipeline Requests</span>
                        <span class="metric-value" id="pipelineRequests">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Pipeline Success</span>
                        <span class="metric-value text-success" id="pipelineSuccess">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Pipeline Failed</span>
                        <span class="metric-value text-error" id="pipelineFailed">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Avg Pipeline Time</span>
                        <span class="metric-value" id="pipelineAvgTime">0ms</span>
                    </div>
                </div>
            </div>

            <!-- System Resources -->
            <div class="card">
                <div class="card-header">
                    <h3>💾 System Resources</h3>
                </div>
                <div class="card-body">
                    <div class="metric">
                        <span class="metric-label">Memory Usage</span>
                        <div class="progress mb-2">
                            <div id="memoryProgress" class="progress-bar" style="width: 0%"></div>
                        </div>
                        <span class="metric-value text-sm" id="memoryUsage">0 MB</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Goroutines</span>
                        <span class="metric-value" id="goroutines">0</span>
                    </div>
                    <div class="metric">
                        <span class="metric-label">Uptime</span>
                        <span class="metric-value" id="uptime">0s</span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Worker Status Table -->
        <div class="card mb-6">
            <div class="card-header">
                <h3>👥 Worker Status</h3>
                <div class="text-sm text-gray-600">
                    Last Updated: <span id="lastUpdate">Never</span>
                </div>
            </div>
            <div class="card-body">
                <div class="table-container">
                    <table class="table">
                        <thead>
                            <tr>
                                <th>Worker ID</th>
                                <th>Status</th>
                                <th>Current Task</th>
                                <th>Processing Time</th>
                                <th>Queue Position</th>
                            </tr>
                        </thead>
                        <tbody id="workerTableBody">
                            <tr>
                                <td colspan="5" class="text-center py-4 text-gray-500">
                                    Loading worker status...
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <!-- Quick Actions -->
        <div class="card">
            <div class="card-header">
                <h3>⚡ Quick Actions</h3>
            </div>
            <div class="card-body">
                <div class="grid grid-cols-4">
                    <button onclick="refreshDashboard()" class="btn btn-primary">
                        🔄 Refresh Data
                    </button>
                    <button onclick="toggleAutoRefresh()" class="btn btn-secondary" id="autoRefreshBtn">
                        ⏸️ Pause Auto-refresh
                    </button>
                    <a href="/api/v1/health" target="_blank" class="btn btn-secondary">
                        🏥 Health Check
                    </a>
                    <a href="/performance" class="btn btn-secondary">
                        📈 Performance Details
                    </a>
                </div>
            </div>
        </div>
    </div>

    <script>
        let ws;
        let performanceChart;
        let healthChart;
        let autoRefreshEnabled = true;
        let chartType = 'bar'; // 'bar' or 'line'
        
        // Chart data storage
        let performanceData = {
            labels: [],
            datasets: [{
                label: 'Processing Time (ms)',
                data: [],
                backgroundColor: 'rgba(0, 124, 186, 0.6)',
                borderColor: 'rgba(0, 124, 186, 1)',
                borderWidth: 2
            }]
        };

        let healthData = {
            labels: ['CPU', 'Memory', 'Disk', 'Network'],
            datasets: [{
                data: [85, 45, 30, 20],
                backgroundColor: [
                    'rgba(255, 99, 132, 0.6)',
                    'rgba(54, 162, 235, 0.6)', 
                    'rgba(255, 205, 86, 0.6)',
                    'rgba(75, 192, 192, 0.6)'
                ]
            }]
        };

        function initializeCharts() {
            // Performance Chart
            const perfCtx = document.getElementById('performanceChart').getContext('2d');
            performanceChart = new Chart(perfCtx, {
                type: chartType,
                data: performanceData,
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });

            // Health Chart
            const healthCtx = document.getElementById('healthChart').getContext('2d');
            healthChart = new Chart(healthCtx, {
                type: 'doughnut',
                data: healthData,
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            position: 'bottom'
                        }
                    }
                }
            });
        }

        function connectWebSocket() {
            // Instead of WebSocket, use HTTP polling for real-time updates
            // This avoids nginx WebSocket proxy issues
            updateConnectionStatus(true);
            console.log('Using HTTP polling for real-time updates');
            
            // Start polling for metrics every 2 seconds
            setInterval(function() {
                fetch('/api/v1/metrics')
                    .then(response => response.json())
                    .then(data => {
                        if (data.success && data.data) {
                            updateMetrics(data.data);
                            updateLastUpdateTime();
                        }
                    })
                    .catch(error => {
                        console.error('Failed to fetch metrics:', error);
                        updateConnectionStatus(false);
                    });
            }, 2000);
        }

        function updateConnectionStatus(connected) {
            const statusDot = document.getElementById('wsStatusDot');
            const statusText = document.getElementById('wsStatusText');
            const indicator = document.getElementById('wsStatusIndicator');
            
            if (connected) {
                indicator.className = 'status-indicator status-success';
                statusText.textContent = 'Live Updates';
            } else {
                indicator.className = 'status-indicator status-error';
                statusText.textContent = 'Updates Paused';
            }
        }

        function updateMetrics(data) {
            // Update overview cards
            if (data.orchestrator) {
                const orch = data.orchestrator;
                document.getElementById('totalDocuments').textContent = orch.TotalRequests || 0;
                document.getElementById('totalAnalyses').textContent = orch.SuccessfulAnalyses || 0;
                document.getElementById('totalRequests').textContent = orch.TotalRequests || 0;
                document.getElementById('successfulAnalyses').textContent = orch.SuccessfulAnalyses || 0;
                document.getElementById('failedAnalyses').textContent = orch.FailedAnalyses || 0;
                
                // Calculate success rate
                const total = orch.TotalRequests || 0;
                const successful = orch.SuccessfulAnalyses || 0;
                const successRate = total > 0 ? ((successful / total) * 100).toFixed(1) : 100;
                document.getElementById('successRate').textContent = successRate + '%';
                
                // Update processing time
                if (orch.AverageProcessingTime) {
                    const timeMs = (orch.AverageProcessingTime / 1000000).toFixed(2);
                    document.getElementById('avgProcessingTime').textContent = timeMs + 'ms';
                    
                    // Update chart data
                    updatePerformanceChart(timeMs);
                }
            }

            // Update pipeline metrics
            if (data.pipeline) {
                const pipe = data.pipeline;
                document.getElementById('pipelineRequests').textContent = pipe.TotalRequests || 0;
                document.getElementById('pipelineSuccess').textContent = pipe.SuccessfulAnalyses || 0;
                document.getElementById('pipelineFailed').textContent = pipe.FailedAnalyses || 0;
                
                if (pipe.AverageProcessingTime) {
                    const timeMs = (pipe.AverageProcessingTime / 1000000).toFixed(2);
                    document.getElementById('pipelineAvgTime').textContent = timeMs + 'ms';
                }
            }

            // Update worker status
            if (data.workers) {
                document.getElementById('activeWorkers').textContent = data.workers.length || 0;
                document.getElementById('workersChange').textContent = 
                    (data.workers.filter(w => w.Status === 'processing').length || 0) + ' processing';
                updateWorkerTable(data.workers);
            }
        }

        function updatePerformanceChart(timeMs) {
            const now = new Date().toLocaleTimeString();
            
            // Add new data point
            performanceData.labels.push(now);
            performanceData.datasets[0].data.push(parseFloat(timeMs));
            
            // Keep only last 10 data points
            if (performanceData.labels.length > 10) {
                performanceData.labels.shift();
                performanceData.datasets[0].data.shift();
            }
            
            if (performanceChart) {
                performanceChart.update();
            }
        }

        function updateWorkerTable(workers) {
            const tbody = document.getElementById('workerTableBody');
            
            if (!workers || workers.length === 0) {
                tbody.innerHTML = '<tr><td colspan="5" class="text-center py-4 text-gray-500">No active workers</td></tr>';
                return;
            }

            let html = '';
            workers.forEach((worker, index) => {
                const statusClass = worker.Status === 'idle' ? 'status-success' : 
                                  worker.Status === 'processing' ? 'status-warning' : 'status-error';
                
                html += '<tr>';
                html += '<td><span class="font-mono">' + (worker.ID || 'worker-' + index) + '</span></td>';
                html += '<td><span class="status-indicator ' + statusClass + '"><span class="status-dot"></span>' + 
                       (worker.Status || 'unknown') + '</span></td>';
                html += '<td>' + (worker.CurrentTask || '-') + '</td>';
                html += '<td>' + (worker.ProcessingTime ? 
                       (worker.ProcessingTime / 1000000).toFixed(2) + 'ms' : '-') + '</td>';
                html += '<td>' + (worker.QueuePosition || '-') + '</td>';
                html += '</tr>';
            });
            
            tbody.innerHTML = html;
        }

        function updateLastUpdateTime() {
            document.getElementById('lastUpdate').textContent = new Date().toLocaleTimeString();
        }

        function toggleChartType() {
            chartType = chartType === 'bar' ? 'line' : 'bar';
            
            if (performanceChart) {
                performanceChart.destroy();
            }
            
            const ctx = document.getElementById('performanceChart').getContext('2d');
            performanceChart = new Chart(ctx, {
                type: chartType,
                data: performanceData,
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });
            
            document.getElementById('chartToggle').textContent = 
                chartType === 'bar' ? 'Line View' : 'Bar View';
        }

        function refreshDashboard() {
            // Fetch latest metrics via HTTP
            fetch('/api/v1/metrics')
                .then(response => response.json())
                .then(data => {
                    if (data.success && data.data) {
                        updateMetrics(data.data);
                        updateLastUpdateTime();
                    }
                })
                .catch(error => {
                    console.error('Failed to refresh dashboard:', error);
                });
        }

        function toggleAutoRefresh() {
            autoRefreshEnabled = !autoRefreshEnabled;
            const btn = document.getElementById('autoRefreshBtn');
            
            if (autoRefreshEnabled) {
                btn.innerHTML = '⏸️ Pause Auto-refresh';
                btn.className = 'btn btn-secondary';
            } else {
                btn.innerHTML = '▶️ Resume Auto-refresh';
                btn.className = 'btn btn-primary';
            }
        }

        // Initialize dashboard
        document.addEventListener('DOMContentLoaded', function() {
            initializeCharts();
            connectWebSocket();
            
            // Initial data fetch
            refreshDashboard();
        });
    </script>
</body>
</html>`)
}

func (s *Server) handlePerformanceDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IDAES Performance Dashboard</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px;
            border-radius: 10px;
            margin-bottom: 20px;
            text-align: center;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .metric-card {
            background: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .metric-card h3 {
            margin-top: 0;
            color: #333;
            border-bottom: 2px solid #f0f0f0;
            padding-bottom: 10px;
        }
        .metric-value {
            font-size: 2em;
            font-weight: bold;
            color: #667eea;
            margin: 10px 0;
        }
        .metric-label {
            color: #666;
            font-size: 0.9em;
        }
        .status-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 50%%;
            margin-right: 8px;
        }
        .status-healthy { background-color: #4CAF50; }
        .status-warning { background-color: #FF9800; }
        .status-error { background-color: #f44336; }
        .refresh-info {
            text-align: center;
            color: #666;
            margin-top: 20px;
        }
        .error-message {
            background-color: #ffebee;
            color: #c62828;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }
        .chart-container {
            background: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>IDAES Performance Dashboard</h1>
            <p>Real-time monitoring of application metrics and performance</p>
        </div>

        <div id="error-container"></div>

        <div class="metrics-grid">
            <!-- Service Health -->
            <div class="metric-card">
                <h3>Service Health</h3>
                <div id="service-health">
                    <div><span id="app-status" class="status-indicator"></span>Main Application</div>
                    <div><span id="metrics-status" class="status-indicator"></span>Metrics Endpoint</div>
                </div>
            </div>

            <!-- Application Metrics -->
            <div class="metric-card">
                <h3>Application Metrics</h3>
                <div class="metric-value" id="uptime">--</div>
                <div class="metric-label">Uptime</div>
                <div>Documents Uploaded: <span id="docs-uploaded">0</span></div>
                <div>Documents Processed: <span id="docs-processed">0</span></div>
                <div>Analysis Requests: <span id="analysis-total">0</span></div>
            </div>

            <!-- Runtime Metrics -->
            <div class="metric-card">
                <h3>Runtime Metrics</h3>
                <div class="metric-value" id="goroutines">--</div>
                <div class="metric-label">Active Goroutines</div>
                <div>Heap Memory: <span id="heap-memory">--</span></div>
                <div>System Memory: <span id="sys-memory">--</span></div>
            </div>

            <!-- Performance Metrics -->
            <div class="metric-card">
                <h3>Performance</h3>
                <div class="metric-value" id="avg-processing-time">--</div>
                <div class="metric-label">Avg Processing Time (s)</div>
                <div>Active Requests: <span id="active-requests">0</span></div>
                <div>Total Errors: <span id="total-errors">0</span></div>
            </div>
        </div>

        <div class="chart-container">
            <h3>Quick Actions</h3>
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
                <button onclick="window.open('/api/v1/health', '_blank')" 
                        style="padding: 10px 20px; background: #1890ff; color: white; border: none; border-radius: 5px; cursor: pointer;">
                    Health Check
                </button>
                <button onclick="refreshMetrics()" 
                        style="padding: 10px 20px; background: #fa541c; color: white; border: none; border-radius: 5px; cursor: pointer;">
                    Refresh Now
                </button>
            </div>
        </div>

        <div class="refresh-info">
            <p>Last updated: <span id="last-updated">--</span> | Auto-refresh every 10 seconds</p>
        </div>
    </div>

    <script>
        let refreshInterval;
        
        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function formatUptime(seconds) {
            if (!seconds) return '0s';
            const days = Math.floor(seconds / 86400);
            const hours = Math.floor((seconds %% 86400) / 3600);
            const minutes = Math.floor((seconds %% 3600) / 60);
            const secs = seconds %% 60;
            
            if (days > 0) return days + 'd ' + hours + 'h ' + minutes + 'm';
            if (hours > 0) return hours + 'h ' + minutes + 'm';
            if (minutes > 0) return minutes + 'm ' + secs + 's';
            return secs + 's';
        }

        function showError(message) {
            const container = document.getElementById('error-container');
            container.innerHTML = '<div class="error-message">Error: ' + message + '</div>';
        }

        function hideError() {
            document.getElementById('error-container').innerHTML = '';
        }

        async function checkServiceHealth(name, url, statusElementId) {
            try {
                const response = await fetch(url);
                document.getElementById(statusElementId).className = 'status-indicator status-healthy';
                return true;
            } catch (error) {
                document.getElementById(statusElementId).className = 'status-indicator status-error';
                return false;
            }
        }

        async function refreshMetrics() {
            try {
                hideError();
                
                // Check service health
                await checkServiceHealth('App', '/api/v1/health', 'app-status');
                await checkServiceHealth('Metrics', '/api/v1/metrics', 'metrics-status');

                // Fetch metrics from the current server's metrics endpoint
                const response = await fetch('/api/v1/metrics');
                if (!response.ok) {
                    throw new Error('HTTP ' + response.status + ': ' + response.statusText);
                }
                
                const data = await response.json();
                
                // The metrics endpoint should return app metrics
                const metrics = data.data || {};
                
                // Update application metrics
                document.getElementById('uptime').textContent = formatUptime(metrics.uptime_seconds || 0);
                document.getElementById('docs-uploaded').textContent = metrics.documents_uploaded_total || 0;
                document.getElementById('docs-processed').textContent = metrics.documents_processed_total || 0;
                document.getElementById('analysis-total').textContent = metrics.analysis_requests_total || 0;
                
                // Update runtime metrics
                document.getElementById('goroutines').textContent = metrics.goroutines || 0;
                document.getElementById('heap-memory').textContent = formatBytes(metrics.heap_bytes || 0);
                document.getElementById('sys-memory').textContent = formatBytes(metrics.sys_bytes || 0);
                
                // Update performance metrics
                document.getElementById('avg-processing-time').textContent = 
                    (metrics.avg_processing_time_seconds || 0).toFixed(3);
                document.getElementById('active-requests').textContent = metrics.analysis_requests_active || 0;
                document.getElementById('total-errors').textContent = metrics.errors_total || 0;
                
                // Update last refresh time
                document.getElementById('last-updated').textContent = new Date().toLocaleTimeString();
                
            } catch (error) {
                console.error('Failed to fetch metrics:', error);
                showError('Failed to fetch metrics: ' + error.message + '. Make sure the application is running properly.');
            }
        }

        // Initial load
        refreshMetrics();
        
        // Auto-refresh every 10 seconds
        refreshInterval = setInterval(refreshMetrics, 10000);
        
        // Cleanup on page unload
        window.addEventListener('beforeunload', function() {
            if (refreshInterval) {
                clearInterval(refreshInterval);
            }
        });
    </script>
</body>
</html>`)
}
