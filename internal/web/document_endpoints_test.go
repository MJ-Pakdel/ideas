package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/config"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// TestDocumentEndpoints tests the document-specific API endpoints
func TestDocumentEndpoints(t *testing.T) {
	// Create a test server with mock analysis system
	server := createDocumentTestServer(t)
	defer server.Close()

	t.Run("DocumentCitations", func(t *testing.T) {
		testDocumentCitations(t, server)
	})

	t.Run("DocumentCitationsNotFound", func(t *testing.T) {
		testDocumentCitationsNotFound(t, server)
	})

	t.Run("DocumentCitationsInvalidID", func(t *testing.T) {
		testDocumentCitationsInvalidID(t, server)
	})

	t.Run("DocumentCitationsMethodNotAllowed", func(t *testing.T) {
		testDocumentCitationsMethodNotAllowed(t, server)
	})

	t.Run("DocumentOperationsInvalidPath", func(t *testing.T) {
		testDocumentOperationsInvalidPath(t, server)
	})

	t.Run("DocumentGetNotImplemented", func(t *testing.T) {
		testDocumentGetNotImplemented(t, server)
	})

	t.Run("DocumentDelete", func(t *testing.T) {
		testDocumentDelete(t, server)
	})
}

// createDocumentTestServer creates a test server with mock document operations
func createDocumentTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Create mock storage manager
	mockStorageManager := &MockDocumentStorageManager{}

	// Create mock LLM client
	mockLLMClient := &MockDocumentLLMClient{}

	// Create a real AnalysisSystem with our mocks
	mockAnalysisSystem := &analyzers.AnalysisSystem{
		LLMClient:      mockLLMClient,
		StorageManager: mockStorageManager,
		// Other fields can be nil for this test
	}

	// Create server config
	serverConfig := &config.ServerConfig{
		Host:         "localhost",
		Port:         8081,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		EnableCORS:   true,
		StaticDir:    "./static",
		TemplateDir:  "./templates",
	}

	ctx := t.Context()

	// Create server with no analysis service first
	server, err := NewServerWithAnalysisService(ctx, serverConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Manually set the analysisSystem field to use direct mode
	server.analysisSystem = mockAnalysisSystem

	// Create test server
	return httptest.NewServer(server.mux)
}

// testDocumentCitations tests successful citation retrieval for a document
func testDocumentCitations(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/documents/doc1/citations")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should return a JSON array of citations
	var citations []*types.Citation
	if err := json.NewDecoder(resp.Body).Decode(&citations); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have citations for doc1
	if len(citations) != 2 {
		t.Errorf("Expected 2 citations for doc1, got %d", len(citations))
	}

	// Check citation details
	for _, citation := range citations {
		if citation.DocumentID != "doc1" {
			t.Errorf("Expected citation document_id to be 'doc1', got '%s'", citation.DocumentID)
		}
		if citation.ID == "" {
			t.Error("Citation ID should not be empty")
		}
		if citation.Text == "" {
			t.Error("Citation text should not be empty")
		}
	}
}

// testDocumentCitationsNotFound tests citation retrieval for non-existent document
func testDocumentCitationsNotFound(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/documents/nonexistent/citations")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should return empty array for non-existent document
	var citations []*types.Citation
	if err := json.NewDecoder(resp.Body).Decode(&citations); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(citations) != 0 {
		t.Errorf("Expected 0 citations for non-existent document, got %d", len(citations))
	}
}

// testDocumentCitationsInvalidID tests citation retrieval with invalid document ID
func testDocumentCitationsInvalidID(t *testing.T, server *httptest.Server) {
	t.Helper()
	// Test with just the base path - no document ID
	resp, err := http.Get(server.URL + "/api/v1/documents/")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// testDocumentCitationsMethodNotAllowed tests citation endpoint with wrong HTTP method
func testDocumentCitationsMethodNotAllowed(t *testing.T, server *httptest.Server) {
	t.Helper()
	req, err := http.NewRequest("POST", server.URL+"/api/v1/documents/doc1/citations", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

// testDocumentOperationsInvalidPath tests document operations with invalid path
func testDocumentOperationsInvalidPath(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/documents/doc1/invalid")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// testDocumentGetNotImplemented tests document GET endpoint (not yet implemented)
func testDocumentGetNotImplemented(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/documents/doc1")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", resp.StatusCode)
	}
}

// testDocumentDelete tests document DELETE endpoint
func testDocumentDelete(t *testing.T, server *httptest.Server) {
	t.Helper()
	req, err := http.NewRequest("DELETE", server.URL+"/api/v1/documents/doc1", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("Expected success to be true")
	}
	if message, ok := response["message"].(string); !ok || message == "" {
		t.Error("Expected non-empty message")
	}
	if docID, ok := response["document_id"].(string); !ok || docID != "doc1" {
		t.Errorf("Expected document_id to be 'doc1', got %v", docID)
	}
}

// Mock implementations for testing

type MockDocumentStorageManager struct{}

func (m *MockDocumentStorageManager) Initialize(ctx context.Context) error { return nil }
func (m *MockDocumentStorageManager) Close() error                         { return nil }
func (m *MockDocumentStorageManager) StoreDocument(ctx context.Context, document *types.Document) error {
	return nil
}
func (m *MockDocumentStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, document *types.Document) error {
	return nil
}
func (m *MockDocumentStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockDocumentStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockDocumentStorageManager) DeleteDocument(ctx context.Context, id string) error { return nil }
func (m *MockDocumentStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error {
	return nil
}
func (m *MockDocumentStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}
func (m *MockDocumentStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}
func (m *MockDocumentStorageManager) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return []*types.Entity{}, nil
}

// SearchCitations returns mock citations based on document ID filtering
func (m *MockDocumentStorageManager) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	// Return all mock citations - the handler will filter by document ID
	return []*types.Citation{
		{
			ID:          "citation1",
			Text:        "Smith, J. (2023). Test Paper 1.",
			Format:      types.CitationFormatAPA,
			DocumentID:  "doc1",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
		{
			ID:          "citation2",
			Text:        "Johnson, A. (2022). Test Paper 2.",
			Format:      types.CitationFormatMLA,
			DocumentID:  "doc1",
			Confidence:  0.88,
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodRegex,
		},
		{
			ID:          "citation3",
			Text:        "Brown, B. (2021). Different Document Citation.",
			Format:      types.CitationFormatAPA,
			DocumentID:  "doc2",
			Confidence:  0.92,
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodLLM,
		},
	}, nil
}

func (m *MockDocumentStorageManager) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return []*types.Topic{}, nil
}
func (m *MockDocumentStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error {
	return nil
}
func (m *MockDocumentStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error {
	return nil
}
func (m *MockDocumentStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error {
	return nil
}
func (m *MockDocumentStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error) {
	return []interfaces.CollectionInfo{}, nil
}
func (m *MockDocumentStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error) {
	return &interfaces.StorageStats{}, nil
}
func (m *MockDocumentStorageManager) Ping(ctx context.Context) error   { return nil }
func (m *MockDocumentStorageManager) Health(ctx context.Context) error { return nil }

// Mock LLM client for testing
type MockDocumentLLMClient struct{}

func (m *MockDocumentLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	return "mock completion", nil
}
func (m *MockDocumentLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}
func (m *MockDocumentLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}
func (m *MockDocumentLLMClient) Health(ctx context.Context) error { return nil }
func (m *MockDocumentLLMClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{Name: "mock-model", Version: "1.0"}
}
func (m *MockDocumentLLMClient) Close() error { return nil }
