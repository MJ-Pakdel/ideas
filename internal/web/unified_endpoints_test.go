package web

import (
	"bytes"
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

// TestUnifiedExtractionEndpoints tests the new unified extraction API endpoints
func TestUnifiedExtractionEndpoints(t *testing.T) {
	// Create a test server with mock analysis system
	ctx := t.Context()
	server := createTestServer(ctx, t)
	defer server.Close()

	t.Run("GetUnifiedStatus", func(t *testing.T) {
		testGetUnifiedStatus(t, server)
	})

	t.Run("GetUnifiedConfig", func(t *testing.T) {
		testGetUnifiedConfig(t, server)
	})

	t.Run("UpdateUnifiedConfig", func(t *testing.T) {
		testUpdateUnifiedConfig(t, server)
	})

	t.Run("UnifiedAnalyze", func(t *testing.T) {
		testUnifiedAnalyze(t, server)
	})
}

func createTestServer(ctx context.Context, t *testing.T) *httptest.Server {
	t.Helper()
	// Create mock clients
	mockLLMClient := &MockLLMClient{
		responses: map[string]string{
			"unified": `{
				"entities": [{"text": "Test Entity", "type": "PERSON", "confidence": 0.95}],
				"citations": [{"text": "Test et al. (2023)", "source": "Academic Paper", "confidence": 0.90}],
				"topics": [{"name": "testing", "confidence": 0.88}]
			}`,
		},
	}
	mockStorageManager := &MockStorageManager{}

	// Create simplified mock system
	mockSystem := &MockAnalysisSystem{
		pipeline: createMockPipeline(ctx, mockLLMClient, mockStorageManager),
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

	server, err := NewServerWithAnalysisService(ctx, serverConfig, mockSystem)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return httptest.NewServer(server.mux)
}

func createMockPipeline(ctx context.Context, llmClient interfaces.LLMClient, storageManager interfaces.StorageManager) *analyzers.AnalysisPipeline {
	config := &analyzers.AnalysisConfig{
		UseUnifiedExtraction:     true,
		UnifiedFallbackEnabled:   true,
		EnableEntityExtraction:   true,
		EnableCitationExtraction: true,
		EnableTopicExtraction:    true,
		MinConfidenceScore:       0.6,
		MaxConcurrentAnalyses:    4,
		AnalysisTimeout:          time.Second * 10,
		PreferredEntityMethod:    types.ExtractorMethodLLM,
	}

	pipeline := analyzers.NewAnalysisPipeline(ctx, storageManager, llmClient, config)

	// Register a unified extractor to make analysis work
	unifiedExtractor := &MockUnifiedExtractor{llmClient: llmClient}
	pipeline.RegisterUnifiedExtractor(unifiedExtractor)

	return pipeline
}

func testGetUnifiedStatus(t *testing.T, server *httptest.Server) {
	resp, err := http.Get(server.URL + "/api/v1/unified/status")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Response is nil")
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

	// In service mode, the response will be different
	if !apiResponse.Success {
		t.Error("Expected success to be true")
	}

	response := apiResponse.Data

	// Check if we're in service mode or direct system mode
	if status, exists := response["status"]; exists && status == "service_mode" {
		// Service mode - verify service-specific fields
		if _, exists := response["message"]; !exists {
			t.Error("Service mode response missing message field")
		}
		if _, exists := response["health"]; !exists {
			t.Error("Service mode response missing health field")
		}
		t.Logf("Service mode status response: %+v", response)
	} else {
		// Direct system mode - verify system-specific fields
		if _, exists := response["unified_extraction_enabled"]; !exists {
			t.Error("Direct mode response missing unified_extraction_enabled field")
		}
		if _, exists := response["fallback_enabled"]; !exists {
			t.Error("Direct mode response missing fallback_enabled field")
		}
		if _, exists := response["extractor_registered"]; !exists {
			t.Error("Direct mode response missing extractor_registered field")
		}
		t.Logf("Direct mode status response: %+v", response)
	}
}

func testGetUnifiedConfig(t *testing.T, server *httptest.Server) {
	resp, err := http.Get(server.URL + "/api/v1/unified/config")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Response is nil")
	}
	defer resp.Body.Close()

	// In service mode, this should return 503
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Logf("Config endpoint correctly returned 503 in service mode")
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
		return
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify configuration fields
	if _, exists := response["use_unified_extraction"]; !exists {
		t.Error("Response missing use_unified_extraction field")
	}
	if _, exists := response["min_confidence_score"]; !exists {
		t.Error("Response missing min_confidence_score field")
	}

	t.Logf("Config response: %+v", response)
}

func testUpdateUnifiedConfig(t *testing.T, server *httptest.Server) {
	updateRequest := map[string]any{
		"use_unified_extraction": false,
		"min_confidence_score":   0.8,
	}

	requestBody, err := json.Marshal(updateRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/unified/config", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Response is nil")
	}
	defer resp.Body.Close()

	// In service mode, this should return 503
	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Logf("Config update endpoint correctly returned 503 in service mode")
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
		return
	}

	var response map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify the update was applied
	if enabled, ok := response["use_unified_extraction"].(bool); !ok || enabled {
		t.Error("Expected use_unified_extraction to be false after update")
	}
	if confidence, ok := response["min_confidence_score"].(float64); !ok || confidence != 0.8 {
		t.Errorf("Expected min_confidence_score to be 0.8, got %v", confidence)
	}

	t.Logf("Update response: %+v", response)
}

func testUnifiedAnalyze(t *testing.T, server *httptest.Server) {
	analyzeRequest := map[string]any{
		"text":          "This is a test document with Dr. Smith and a reference to Johnson (2023).",
		"filename":      "test.txt",
		"force_unified": true,
	}

	requestBody, err := json.Marshal(analyzeRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(server.URL+"/api/v1/unified/analyze", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if resp == nil {
		t.Fatalf("Response is nil")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var apiResponse struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
		Error   string         `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !apiResponse.Success {
		t.Errorf("Expected success to be true, got error: %s", apiResponse.Error)
		return
	}

	response := apiResponse.Data

	// Verify response structure
	if _, exists := response["document_id"]; !exists {
		t.Error("Response missing document_id field")
	}
	if _, exists := response["entities"]; !exists {
		t.Error("Response missing entities field")
	}
	if _, exists := response["citations"]; !exists {
		t.Error("Response missing citations field")
	}
	if _, exists := response["topics"]; !exists {
		t.Error("Response missing topics field")
	}
	if _, exists := response["processing_time"]; !exists {
		t.Error("Response missing processing_time field")
	}

	t.Logf("Analyze response: %+v", response)
}

// Mock types for testing

type MockAnalysisSystem struct {
	pipeline *analyzers.AnalysisPipeline
}

func (m *MockAnalysisSystem) AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error) {
	request := &analyzers.AnalysisRequest{
		RequestID: "test-request",
		Document:  document,
	}
	return m.pipeline.AnalyzeDocument(ctx, request), nil
}

func (m *MockAnalysisSystem) GetSystemStatus() map[string]any {
	return map[string]any{
		"status": "running",
		"mode":   "test",
	}
}

func (m *MockAnalysisSystem) Health() any {
	return map[string]any{
		"status":  "healthy",
		"message": "mock analysis system",
	}
}

func (m *MockAnalysisSystem) Start(ctx context.Context) error { return nil }
func (m *MockAnalysisSystem) Stop(ctx context.Context) error  { return nil }
func (m *MockAnalysisSystem) Close() error {
	return m.pipeline.Shutdown(context.Background())
}
func (m *MockAnalysisSystem) Shutdown(ctx context.Context) error {
	return m.pipeline.Shutdown(ctx)
}

// Search methods for AnalysisService interface
func (m *MockAnalysisSystem) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}

func (m *MockAnalysisSystem) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return []*types.Entity{}, nil
}

func (m *MockAnalysisSystem) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	return []*types.Citation{}, nil
}

func (m *MockAnalysisSystem) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return []*types.Topic{}, nil
}

func (m *MockAnalysisSystem) SemanticSearch(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}

// Document management methods for AnalysisService interface
func (m *MockAnalysisSystem) DeleteDocument(ctx context.Context, documentID string) error {
	return nil // Mock implementation always succeeds
}

// Properties for compatibility
func (m *MockAnalysisSystem) GetPipeline() *analyzers.AnalysisPipeline {
	return m.pipeline
}

// Mock LLM and Storage clients (same as in integration test)
type MockLLMClient struct {
	responses map[string]string
	callCount int
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	m.callCount++
	return m.responses["unified"], nil
}

func (m *MockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (m *MockLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	}
	return result, nil
}

func (m *MockLLMClient) Health(ctx context.Context) error { return nil }

func (m *MockLLMClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{
		Name:         "mock-model",
		Provider:     "mock",
		Version:      "1.0",
		ContextSize:  4096,
		EmbeddingDim: 768,
	}
}

func (m *MockLLMClient) Close() error { return nil }

// Legacy method for backward compatibility
func (m *MockLLMClient) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	return m.Complete(ctx, prompt, nil)
}

func (m *MockLLMClient) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	return m.Embed(ctx, text)
}

type MockStorageManager struct{}

func (m *MockStorageManager) Initialize(ctx context.Context) error { return nil }
func (m *MockStorageManager) StoreDocument(ctx context.Context, document *types.Document) error {
	return nil
}
func (m *MockStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, document *types.Document) error {
	return nil
}
func (m *MockStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error) {
	return nil, nil
}
func (m *MockStorageManager) DeleteDocument(ctx context.Context, id string) error { return nil }
func (m *MockStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error {
	return nil
}
func (m *MockStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}
func (m *MockStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	return []*types.Document{}, nil
}
func (m *MockStorageManager) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	return []*types.Entity{}, nil
}
func (m *MockStorageManager) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	return []*types.Citation{}, nil
}
func (m *MockStorageManager) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	return []*types.Topic{}, nil
}
func (m *MockStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error { return nil }
func (m *MockStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error {
	return nil
}
func (m *MockStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error { return nil }
func (m *MockStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error) {
	return []interfaces.CollectionInfo{}, nil
}
func (m *MockStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error) {
	return &interfaces.StorageStats{}, nil
}
func (m *MockStorageManager) Health(ctx context.Context) error { return nil }
func (m *MockStorageManager) Close() error                     { return nil }

// MockUnifiedExtractor for testing
type MockUnifiedExtractor struct {
	llmClient interfaces.LLMClient
}

func (m *MockUnifiedExtractor) ExtractAll(ctx context.Context, document *types.Document, config *interfaces.UnifiedExtractionConfig) (*interfaces.UnifiedExtractionResult, error) {
	// Return mock extraction results
	return &interfaces.UnifiedExtractionResult{
		Entities: []*types.Entity{
			{
				Text:        "Test Entity",
				Type:        "PERSON",
				Confidence:  0.95,
				StartOffset: 0,
				EndOffset:   11,
			},
		},
		Citations: []*types.Citation{
			{
				Text:       "Test et al. (2023)",
				Confidence: 0.90,
			},
		},
		Topics: []*types.Topic{
			{
				Name:       "testing",
				Confidence: 0.88,
			},
		},
		QualityMetrics: &interfaces.QualityMetrics{
			ConfidenceScore:   0.91,
			CompletenessScore: 0.85,
			ReliabilityScore:  0.87,
		},
	}, nil
}

func (m *MockUnifiedExtractor) GetCapabilities() interfaces.ExtractorCapabilities {
	return interfaces.ExtractorCapabilities{
		SupportedEntityTypes:     []types.EntityType{"PERSON", "ORGANIZATION"},
		SupportedCitationFormats: []types.CitationFormat{"APA", "MLA"},
		SupportedDocumentTypes:   []types.DocumentType{"text", "academic_paper"},
		SupportsConfidenceScores: true,
		SupportsContext:          true,
		SupportsRelationships:    false,
		RequiresExternalService:  false,
		MaxDocumentSize:          1024 * 1024, // 1MB
		AverageProcessingTime:    time.Millisecond * 100,
	}
}

func (m *MockUnifiedExtractor) GetMetrics() interfaces.ExtractorMetrics {
	return interfaces.ExtractorMetrics{
		ProcessingTime:       time.Millisecond * 100,
		TotalDocuments:       1,
		TotalEntities:        1,
		TotalCitations:       1,
		AverageConfidence:    0.9,
		ErrorRate:            0.0,
		CacheHitRate:         0.0,
		ExternalServiceCalls: 0,
		MemoryUsage:          1024,
	}
}

func (m *MockUnifiedExtractor) Close() error {
	return nil
}
