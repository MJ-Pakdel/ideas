package subsystems

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/idaes/internal/config"
	"github.com/example/idaes/internal/web"
)

// WebSubsystem wraps the web server and provides subscription to IDAES subsystem
type WebSubsystem struct {
	server       *web.Server
	idaesClient  *IDaesClient
	idaesAdapter *IDaesAdapter
	config       *config.ServerConfig

	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// IDaesClient wraps access to the IDAES subsystem
type IDaesClient struct {
	idaesSubsystem  *IDaesSubsystem
	ctx             context.Context
	subscriberID    string
	responseChan    chan *AnalysisResponse
	pendingRequests map[string]chan *AnalysisResponse
	pendingMutex    sync.RWMutex
}

// WebSubsystemConfig contains configuration for the web subsystem
type WebSubsystemConfig struct {
	ServerConfig *config.ServerConfig `json:"server_config"`
	SubscriberID string               `json:"subscriber_id"`
}

// NewWebSubsystem creates a new web subsystem that subscribes to IDAES
func NewWebSubsystem(ctx context.Context, config *WebSubsystemConfig, idaesSubsystem *IDaesSubsystem) (*WebSubsystem, error) {
	if config == nil {
		return nil, fmt.Errorf("web subsystem config is required")
	}

	if idaesSubsystem == nil {
		return nil, fmt.Errorf("IDAES subsystem is required")
	}

	// Create subsystem context
	subsystemCtx, cancel := context.WithCancel(ctx)

	// Create IDAES client
	idaesClient := &IDaesClient{
		idaesSubsystem:  idaesSubsystem,
		subscriberID:    config.SubscriberID,
		responseChan:    make(chan *AnalysisResponse, 100),
		pendingRequests: make(map[string]chan *AnalysisResponse),
		ctx:             subsystemCtx,
	}

	// Create adapter for web server integration
	idaesAdapter := NewIDaesAdapter(idaesClient, 180*time.Second)

	// Create web server that uses IDAES adapter
	server, err := web.NewServerWithAnalysisService(subsystemCtx, config.ServerConfig, idaesAdapter)
	if err != nil {
		cancel() // Clean up context before returning error
		return nil, fmt.Errorf("failed to create web server: %w", err)
	}

	subsystem := &WebSubsystem{
		server:       server,
		idaesClient:  idaesClient,
		idaesAdapter: idaesAdapter,
		config:       config.ServerConfig,
		ctx:          subsystemCtx,
		cancel:       cancel,
	}

	return subsystem, nil
}

// Start begins the web subsystem
func (w *WebSubsystem) Start() error {
	slog.InfoContext(w.ctx, "Starting web subsystem")

	// Subscribe to IDAES responses
	w.idaesClient.idaesSubsystem.Subscribe(w.idaesClient.subscriberID, w.idaesClient.handleResponse)

	// Start the web server
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.server.Start(w.ctx); err != nil {
			slog.ErrorContext(w.ctx, "Web server failed", "error", err)
		}
	}()

	slog.InfoContext(w.ctx, "Web subsystem started")
	return nil
}

// Stop gracefully shuts down the web subsystem
func (w *WebSubsystem) Stop() error {
	slog.InfoContext(w.ctx, "Stopping web subsystem")

	// Unsubscribe from IDAES
	w.idaesClient.idaesSubsystem.Unsubscribe(w.idaesClient.subscriberID)

	// Stop the web server
	if err := w.server.Stop(w.ctx); err != nil {
		slog.WarnContext(w.ctx, "Error stopping web server", "error", err)
	}

	// Cancel context
	w.cancel()

	// Wait for all goroutines to finish
	w.wg.Wait()

	slog.InfoContext(w.ctx, "Web subsystem stopped")
	return nil
}

// GetHealth returns the health status of the web subsystem
func (w *WebSubsystem) GetHealth() map[string]any {
	health := make(map[string]any)

	// Web server health
	health["web_server"] = "healthy" // TODO: Add actual health check

	// IDAES connection health
	idaesHealth := w.idaesClient.idaesSubsystem.GetHealth()
	health["idaes_connection"] = map[string]any{
		"status":      idaesHealth.Status,
		"subscriber":  w.idaesClient.subscriberID,
		"last_update": idaesHealth.Timestamp,
	}

	return health
}

// SubmitAnalysisRequest submits an analysis request to IDAES
func (i *IDaesClient) SubmitAnalysisRequest(request *AnalysisRequest) error {
	return i.idaesSubsystem.SubmitRequest(request)
}

// handleResponse handles responses from IDAES subsystem
func (i *IDaesClient) handleResponse(response *AnalysisResponse) {
	slog.DebugContext(i.ctx, "Handling response from IDAES", "request_id", response.RequestID, "status", response.Status)

	// First, try to route to a specific pending request
	i.pendingMutex.RLock()
	requestChan, exists := i.pendingRequests[response.RequestID]
	i.pendingMutex.RUnlock()

	if exists {
		slog.DebugContext(i.ctx, "Found pending request for response", "request_id", response.RequestID)
		select {
		case requestChan <- response:
			slog.DebugContext(i.ctx, "Response routed to specific request", "request_id", response.RequestID, "status", response.Status)
			return
		case <-i.ctx.Done():
			slog.WarnContext(i.ctx, "Context cancelled while routing response to specific request", "request_id", response.RequestID)
			return
		default:
			slog.WarnContext(i.ctx, "Request-specific channel full, falling back to general channel", "request_id", response.RequestID)
		}
	} else {
		slog.WarnContext(i.ctx, "No pending request found for response", "request_id", response.RequestID)
	}

	// Fallback to general response channel
	select {
	case i.responseChan <- response:
		slog.DebugContext(i.ctx, "Response sent to general channel", "request_id", response.RequestID, "status", response.Status)
	case <-i.ctx.Done():
		slog.WarnContext(i.ctx, "Context cancelled while handling IDAES response", "request_id", response.RequestID)
	default:
		slog.WarnContext(i.ctx, "Response channel full, dropping response", "request_id", response.RequestID)
	}
}

// GetResponse retrieves a response from the IDAES subsystem for a specific request ID (blocking)
func (i *IDaesClient) GetResponse(timeout time.Duration) (*AnalysisResponse, error) {
	select {
	case response := <-i.responseChan:
		return response, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	case <-i.ctx.Done():
		return nil, fmt.Errorf("context cancelled")
	}
}

// GetResponseForRequest retrieves a response from the IDAES subsystem for a specific request ID
func (i *IDaesClient) GetResponseForRequest(requestID string, timeout time.Duration) (*AnalysisResponse, error) {
	slog.DebugContext(i.ctx, "Waiting for response", "request_id", requestID, "timeout", timeout)

	// Create a channel for this specific request
	requestChan := make(chan *AnalysisResponse, 1)

	// Register the request
	i.pendingMutex.Lock()
	i.pendingRequests[requestID] = requestChan
	i.pendingMutex.Unlock()

	slog.DebugContext(i.ctx, "Request registered for response routing", "request_id", requestID)

	// Ensure cleanup
	defer func() {
		i.pendingMutex.Lock()
		delete(i.pendingRequests, requestID)
		close(requestChan)
		i.pendingMutex.Unlock()
		slog.DebugContext(i.ctx, "Request cleaned up", "request_id", requestID)
	}()

	// Wait for response
	select {
	case response := <-requestChan:
		slog.DebugContext(i.ctx, "Response received for request", "request_id", requestID, "status", response.Status)
		return response, nil
	case <-time.After(timeout):
		slog.ErrorContext(i.ctx, "Timeout waiting for response", "request_id", requestID, "timeout", timeout)
		return nil, fmt.Errorf("timeout waiting for response with request ID %s", requestID)
	case <-i.ctx.Done():
		slog.WarnContext(i.ctx, "Context cancelled while waiting for response", "request_id", requestID)
		return nil, fmt.Errorf("context cancelled")
	}
}

// GetResponseAsync retrieves a response from the IDAES subsystem (non-blocking)
func (i *IDaesClient) GetResponseAsync() (*AnalysisResponse, bool) {
	select {
	case response := <-i.responseChan:
		return response, true
	default:
		return nil, false
	}
}

// GetHealth returns the health status of the IDAES connection
func (i *IDaesClient) GetHealth() *HealthStatus {
	return i.idaesSubsystem.GetHealth()
}

// DefaultWebSubsystemConfig returns a default configuration for the web subsystem
func DefaultWebSubsystemConfig() *WebSubsystemConfig {
	return &WebSubsystemConfig{
		ServerConfig: &config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			StaticDir:    "./web/static",
			TemplateDir:  "./web/templates",
		},
		SubscriberID: "web_subsystem",
	}
}
