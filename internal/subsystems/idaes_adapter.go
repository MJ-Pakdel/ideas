package subsystems

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/types"
)

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
}

// IDaesAdapter adapts the IDAES subsystem to work with existing web server code
// This allows the web server to continue using the AnalyzeDocument pattern
// while transparently using the new channel-based IDAES subsystem
type IDaesAdapter struct {
	idaesClient *IDaesClient
	timeout     time.Duration
}

// NewIDaesAdapter creates a new adapter for the IDAES subsystem
func NewIDaesAdapter(idaesClient *IDaesClient, timeout time.Duration) *IDaesAdapter {
	return &IDaesAdapter{
		idaesClient: idaesClient,
		timeout:     timeout,
	}
}

// AnalyzeDocument adapts the synchronous AnalyzeDocument call to the async IDAES subsystem
// This maintains compatibility with existing web server code
func (a *IDaesAdapter) AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error) {
	// Generate a unique request ID
	requestID := uuid.New().String()

	// Create analysis request
	request := &AnalysisRequest{
		RequestID: requestID,
		Document:  document,
		Options: &AnalysisOptions{
			EnableEntities:  true,
			EnableCitations: true,
			EnableTopics:    true,
			Timeout:         a.timeout,
		},
	}

	// Submit request to IDAES subsystem
	if err := a.idaesClient.SubmitAnalysisRequest(request); err != nil {
		return nil, fmt.Errorf("failed to submit analysis request: %w", err)
	}

	// Wait for response with timeout
	response, err := a.idaesClient.GetResponseForRequest(requestID, a.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get analysis response: %w", err)
	}

	// Response is already verified to have the correct RequestID
	// Convert to the expected format
	if response.Error != nil {
		return &analyzers.AnalysisResponse{
			Result:         nil,
			Error:          response.Error,
			ProcessingTime: response.ProcessingTime,
			ExtractorUsed:  convertExtractorUsed(response.ExtractorUsed),
		}, nil
	}

	return &analyzers.AnalysisResponse{
		Result:         response.Result,
		Error:          nil,
		ProcessingTime: response.ProcessingTime,
		ExtractorUsed:  convertExtractorUsed(response.ExtractorUsed),
	}, nil
}

// convertExtractorUsed converts from interface{} map to ExtractorMethod map
func convertExtractorUsed(input map[string]any) map[string]types.ExtractorMethod {
	result := make(map[string]types.ExtractorMethod)
	for k, v := range input {
		if method, ok := v.(types.ExtractorMethod); ok {
			result[k] = method
		}
	}
	return result
}

// Health returns the health status of the IDAES subsystem
func (a *IDaesAdapter) Health() any {
	return a.idaesClient.GetHealth()
}

// Close cleans up the adapter (no-op for now)
func (a *IDaesAdapter) Close() error {
	// The adapter doesn't own the IDAES client, so no cleanup needed
	return nil
}

// Search methods - these delegate to the storage manager of the analysis system
func (a *IDaesAdapter) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.SearchDocumentsInCollection(ctx, collection, query, limit)
}

func (a *IDaesAdapter) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.SearchEntities(ctx, query, limit)
}

func (a *IDaesAdapter) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.SearchCitations(ctx, query, limit)
}

func (a *IDaesAdapter) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.SearchTopics(ctx, query, limit)
}

func (a *IDaesAdapter) SemanticSearch(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.SearchDocumentsInCollection(ctx, "documents", query, limit)
}

func (a *IDaesAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	// Get the storage manager from the IDAES subsystem's analysis system
	storageManager := a.idaesClient.idaesSubsystem.AnalysisSystem.StorageManager
	return storageManager.DeleteDocument(ctx, documentID)
}

// AnalysisSystemAdapter adapts the existing AnalysisSystem to the AnalysisService interface
type AnalysisSystemAdapter struct {
	analysisSystem *analyzers.AnalysisSystem
}

// NewAnalysisSystemAdapter creates a new adapter for the existing AnalysisSystem
func NewAnalysisSystemAdapter(analysisSystem *analyzers.AnalysisSystem) *AnalysisSystemAdapter {
	return &AnalysisSystemAdapter{
		analysisSystem: analysisSystem,
	}
}

// AnalyzeDocument implements AnalysisService interface
func (a *AnalysisSystemAdapter) AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error) {
	return a.analysisSystem.AnalyzeDocument(ctx, document)
}

// Health implements AnalysisService interface
func (a *AnalysisSystemAdapter) Health() any {
	// TODO: Add actual health check for AnalysisSystem
	return map[string]string{"status": "healthy"}
}

// Close implements AnalysisService interface
func (a *AnalysisSystemAdapter) Close() error {
	// TODO: Add proper cleanup for AnalysisSystem if needed
	return nil
}

// Search methods for AnalysisSystemAdapter
func (a *AnalysisSystemAdapter) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return a.analysisSystem.StorageManager.SearchDocumentsInCollection(ctx, collection, query, limit)
}

func (a *AnalysisSystemAdapter) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return a.analysisSystem.StorageManager.SearchEntities(ctx, query, limit)
}

func (a *AnalysisSystemAdapter) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	return a.analysisSystem.StorageManager.SearchCitations(ctx, query, limit)
}

func (a *AnalysisSystemAdapter) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return a.analysisSystem.StorageManager.SearchTopics(ctx, query, limit)
}

func (a *AnalysisSystemAdapter) SemanticSearch(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return a.analysisSystem.StorageManager.SearchDocumentsInCollection(ctx, "documents", query, limit)
}

func (a *AnalysisSystemAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	return a.analysisSystem.StorageManager.DeleteDocument(ctx, documentID)
}
