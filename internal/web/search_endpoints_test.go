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

// TestSearchEndpoints tests the new search API endpoints
func TestSearchEndpoints(t *testing.T) {
	// Create a test server with mock analysis system
	server := createSearchTestServer(t)
	defer server.Close()

	t.Run("DocumentSearch", func(t *testing.T) {
		testDocumentSearch(t, server)
	})

	t.Run("EntitySearch", func(t *testing.T) {
		testEntitySearch(t, server)
	})

	t.Run("CitationSearch", func(t *testing.T) {
		testCitationSearch(t, server)
	})

	t.Run("TopicSearch", func(t *testing.T) {
		testTopicSearch(t, server)
	})

	t.Run("SemanticSearch", func(t *testing.T) {
		testSemanticSearch(t, server)
	})
}

func createSearchTestServer(t *testing.T) *httptest.Server {
	// Create mock storage manager
	mockStorageManager := &MockSearchStorageManager{}

	// Create mock LLM client
	mockLLMClient := &MockSearchLLMClient{}

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

	return httptest.NewServer(server.mux)
}

func testDocumentSearch(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/search/documents?q=test&limit=5")
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

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}

	response := apiResponse.Data
	if query, ok := response["query"].(string); !ok || query != "test" {
		t.Errorf("Expected query 'test', got %v", response["query"])
	}

	if totalFound, ok := response["total_found"].(float64); !ok || totalFound != 1 {
		t.Errorf("Expected total_found 1, got %v", response["total_found"])
	}
}

func testEntitySearch(t *testing.T, server *httptest.Server) {
	t.Helper()
	resp, err := http.Get(server.URL + "/api/v1/search/entities?q=person&type=PERSON&limit=5")
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

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}

	response := apiResponse.Data
	if typeFilter, ok := response["type_filter"].(string); !ok || typeFilter != "PERSON" {
		t.Errorf("Expected type_filter 'PERSON', got %v", response["type_filter"])
	}
}

func testCitationSearch(t *testing.T, server *httptest.Server) {
	resp, err := http.Get(server.URL + "/api/v1/search/citations?q=smith&format=APA&limit=5")
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

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}
}

func testTopicSearch(t *testing.T, server *httptest.Server) {
	resp, err := http.Get(server.URL + "/api/v1/search/topics?q=learning&min_confidence=0.8&limit=5")
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

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}
}

func testSemanticSearch(t *testing.T, server *httptest.Server) {
	resp, err := http.Get(server.URL + "/api/v1/search/semantic?q=machine+learning&types=documents,entities&limit=3")
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

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}

	response := apiResponse.Data
	if results, ok := response["results"].(map[string]any); !ok {
		t.Error("Expected results object")
	} else {
		// Should have documents and entities but not citations or topics
		if _, hasDocuments := results["documents"]; !hasDocuments {
			t.Error("Expected documents in semantic search results")
		}
		if _, hasEntities := results["entities"]; !hasEntities {
			t.Error("Expected entities in semantic search results")
		}
	}
}

// Mock implementations for testing

type MockSearchStorageManager struct{}

func (m *MockSearchStorageManager) Initialize(ctx context.Context) error { return nil }
func (m *MockSearchStorageManager) StoreDocument(ctx context.Context, document *types.Document) error {
	return nil
}
func (m *MockSearchStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, document *types.Document) error {
	return nil
}
func (m *MockSearchStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockSearchStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockSearchStorageManager) DeleteDocument(ctx context.Context, id string) error { return nil }
func (m *MockSearchStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error {
	return nil
}

func (m *MockSearchStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{
		{
			ID:      "doc1",
			Name:    "Test Document",
			Content: "This is a test document",
		},
	}, nil
}

func (m *MockSearchStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return m.SearchDocuments(ctx, query, limit)
}

func (m *MockSearchStorageManager) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return []*types.Entity{
		{
			ID:   "entity1",
			Text: "John Smith",
			Type: types.EntityTypePerson,
		},
	}, nil
}

func (m *MockSearchStorageManager) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	return []*types.Citation{
		{
			ID:     "citation1",
			Text:   "Smith, J. (2023). Test Paper.",
			Format: types.CitationFormatAPA,
		},
	}, nil
}

func (m *MockSearchStorageManager) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return []*types.Topic{
		{
			ID:         "topic1",
			Name:       "Machine Learning",
			Confidence: 0.9,
		},
	}, nil
}

func (m *MockSearchStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error {
	return nil
}
func (m *MockSearchStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error {
	return nil
}
func (m *MockSearchStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error {
	return nil
}
func (m *MockSearchStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error) {
	return nil, nil
}
func (m *MockSearchStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error) {
	return nil, nil
}
func (m *MockSearchStorageManager) Health(ctx context.Context) error { return nil }
func (m *MockSearchStorageManager) Close() error                     { return nil }

type MockSearchLLMClient struct{}

func (m *MockSearchLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	return "mock response", nil
}
func (m *MockSearchLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}
func (m *MockSearchLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}
func (m *MockSearchLLMClient) Health(ctx context.Context) error { return nil }
func (m *MockSearchLLMClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{Name: "mock", Provider: "test"}
}
func (m *MockSearchLLMClient) Close() error { return nil }

type MockSearchAnalysisSystem struct {
	storageManager interfaces.StorageManager
	llmClient      interfaces.LLMClient
}

func (m *MockSearchAnalysisSystem) AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error) {
	return &analyzers.AnalysisResponse{
		Result: &types.AnalysisResult{
			DocumentID: document.ID,
			Entities:   []*types.Entity{},
			Citations:  []*types.Citation{},
			Topics:     []*types.Topic{},
		},
	}, nil
}

func (m *MockSearchAnalysisSystem) GetSystemStatus() map[string]any {
	return map[string]any{"status": "running"}
}

func (m *MockSearchAnalysisSystem) Health() any {
	return map[string]any{"status": "healthy"}
}

func (m *MockSearchAnalysisSystem) Start(ctx context.Context) error    { return nil }
func (m *MockSearchAnalysisSystem) Stop(ctx context.Context) error     { return nil }
func (m *MockSearchAnalysisSystem) Close() error                       { return nil }
func (m *MockSearchAnalysisSystem) Shutdown(ctx context.Context) error { return nil }

// Properties for compatibility
func (m *MockSearchAnalysisSystem) GetStorageManager() interfaces.StorageManager {
	return m.storageManager
}

func (m *MockSearchAnalysisSystem) GetLLMClient() interfaces.LLMClient {
	return m.llmClient
}
