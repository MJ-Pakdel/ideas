package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/idaes/internal/clients"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// Test constants
const (
	testDefaultCollection  = "test_documents"
	testEntityCollection   = "test_entities"
	testCitationCollection = "test_citations"
	testTopicCollection    = "test_topics"
	testEmbeddingDimension = 384
	testMaxRetries         = 1                     // Reduced for faster tests
	testRetryDelay         = time.Millisecond * 10 // Reduced for faster tests
	testTenant             = "test_tenant"
	testDatabase           = "test_database"
	testTimeout            = 2 * time.Second // Reduced for faster tests
)

// Mock LLM client for testing embedding generation
type mockLLMClient struct {
	embedFunc      func(ctx context.Context, text string) ([]float64, error)
	embedBatchFunc func(ctx context.Context, texts []string) ([][]float64, error)
	completeFunc   func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error)
	healthFunc     func(ctx context.Context) error
}

func (m *mockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, text)
	}
	// Return a test embedding vector
	embedding := make([]float64, testEmbeddingDimension)
	for i := range embedding {
		embedding[i] = float64(i) / float64(testEmbeddingDimension)
	}
	return embedding, nil
}

func (m *mockLLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if m.embedBatchFunc != nil {
		return m.embedBatchFunc(ctx, texts)
	}
	embeddings := make([][]float64, len(texts))
	for i := range texts {
		embedding, err := m.Embed(ctx, texts[i])
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt, options)
	}
	return "test completion response", nil
}

func (m *mockLLMClient) Health(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}

func (m *mockLLMClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{
		Name:         "test-model",
		Provider:     "test-provider",
		Version:      "1.0.0",
		ContextSize:  4096,
		EmbeddingDim: testEmbeddingDimension,
	}
}

func (m *mockLLMClient) Close() error {
	return nil
}

// Helper function to create a mock ChromaDB server
func createMockChromaDBServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Default handlers
	defaultHandlers := map[string]http.HandlerFunc{
		"/api/v2/heartbeat": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int64{"nanosecond_heartbeat": time.Now().UnixNano()})
		},
		"/api/v2/healthcheck": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `"OK"`)
		},
	}

	// Add custom handlers, overriding defaults
	maps.Copy(defaultHandlers, handlers)

	// Register all handlers
	for path, handler := range defaultHandlers {
		mux.HandleFunc(path, handler)
	}

	return httptest.NewServer(mux)
}

// Helper function to create test documents
func createTestDocument(id, content string) *types.Document {
	return &types.Document{
		ID:          id,
		Path:        "/test/" + id + ".txt",
		Name:        id + ".txt",
		Content:     content,
		ContentType: "text/plain",
		Size:        int64(len(content)),
		Hash:        "test-hash-" + id,
		Metadata:    map[string]any{"test": "value"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Helper function to create test entities
func createTestEntity(id, text string, entityType types.EntityType) *types.Entity {
	return &types.Entity{
		ID:          id,
		Type:        entityType,
		Text:        text,
		Confidence:  0.95,
		StartOffset: 0,
		EndOffset:   len(text),
		DocumentID:  "test-doc-1",
		ExtractedAt: time.Now(),
		Method:      types.ExtractorMethodLLM,
		Metadata:    map[string]any{"test": "entity"},
	}
}

// Helper function to create test citations
func createTestCitation(id, text string) *types.Citation {
	title := "Test Citation Title"
	year := 2023
	doi := "10.1000/test.citation"
	url := "https://example.com/citation"

	return &types.Citation{
		ID:          id,
		Text:        text,
		Authors:     []string{"Author One", "Author Two"},
		Title:       &title,
		Year:        &year,
		DOI:         &doi,
		URL:         &url,
		Format:      types.CitationFormatAPA,
		Confidence:  0.9,
		StartOffset: 0,
		EndOffset:   len(text),
		DocumentID:  "test-doc-1",
		ExtractedAt: time.Now(),
		Method:      types.ExtractorMethodLLM,
		Metadata:    map[string]any{"test": "citation"},
	}
}

// Helper function to create test topics
func createTestTopic(id, name string) *types.Topic {
	return &types.Topic{
		ID:          id,
		Name:        name,
		Keywords:    []string{"keyword1", "keyword2"},
		Confidence:  0.8,
		Weight:      0.6,
		Description: "Test topic description",
		DocumentID:  "test-doc-1",
		ExtractedAt: time.Now(),
		Method:      "test-method",
		Metadata:    map[string]any{"test": "topic"},
	}
}

func TestNewChromaStorageManager(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name                   string
		defaultCollection      string
		entityCollection       string
		citationCollection     string
		topicCollection        string
		embeddingDimension     int
		maxRetries             int
		retryDelay             time.Duration
		wantDefaultCollection  string
		wantEmbeddingDimension int
	}{
		{
			name:                   "valid configuration",
			defaultCollection:      testDefaultCollection,
			entityCollection:       testEntityCollection,
			citationCollection:     testCitationCollection,
			topicCollection:        testTopicCollection,
			embeddingDimension:     testEmbeddingDimension,
			maxRetries:             testMaxRetries,
			retryDelay:             testRetryDelay,
			wantDefaultCollection:  testDefaultCollection,
			wantEmbeddingDimension: testEmbeddingDimension,
		},
		{
			name:                   "different dimensions",
			defaultCollection:      "docs",
			entityCollection:       "entities",
			citationCollection:     "citations",
			topicCollection:        "topics",
			embeddingDimension:     768,
			maxRetries:             5,
			retryDelay:             time.Second,
			wantDefaultCollection:  "docs",
			wantEmbeddingDimension: 768,
		},
		{
			name:                   "zero retries",
			defaultCollection:      testDefaultCollection,
			entityCollection:       testEntityCollection,
			citationCollection:     testCitationCollection,
			topicCollection:        testTopicCollection,
			embeddingDimension:     testEmbeddingDimension,
			maxRetries:             0,
			retryDelay:             0,
			wantDefaultCollection:  testDefaultCollection,
			wantEmbeddingDimension: testEmbeddingDimension,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chromaClient := clients.NewChromaDBClient("http://test:8000", testTimeout, tt.maxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				tt.defaultCollection,
				tt.entityCollection,
				tt.citationCollection,
				tt.topicCollection,
				tt.embeddingDimension,
				tt.maxRetries,
				tt.retryDelay,
			)

			if manager == nil {
				t.Fatal("NewChromaStorageManager() returned nil")
			}

			if manager.defaultCollection != tt.wantDefaultCollection {
				t.Errorf("defaultCollection = %v, want %v", manager.defaultCollection, tt.wantDefaultCollection)
			}

			if manager.embeddingDimension != tt.wantEmbeddingDimension {
				t.Errorf("embeddingDimension = %v, want %v", manager.embeddingDimension, tt.wantEmbeddingDimension)
			}

			if manager.collectionIDs == nil {
				t.Error("collectionIDs map should be initialized")
			}

			_ = ctx // Use context to satisfy linting
		})
	}
}

func TestChromaStorageManager_Initialize(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name            string
		setupServer     func() *httptest.Server
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "successful initialization",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							// ListCollections - return empty list initially
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode([]clients.Collection{})
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name: "health check failure",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					"/api/v2/heartbeat": func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, "Health check failed")
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:         true,
			wantErrContains: "ChromaDB health check failed",
		},
		{
			name: "collections already exist",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						if r.Method == "GET" {
							// ListCollections - return existing collections
							collections := []clients.Collection{
								{ID: "existing-id-1", Name: testDefaultCollection},
								{ID: "existing-id-2", Name: testEntityCollection},
								{ID: "existing-id-3", Name: testCitationCollection},
								{ID: "existing-id-4", Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)

			err := manager.Initialize(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Initialize() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}
		})
	}
}

func TestChromaStorageManager_StoreDocument(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name            string
		document        *types.Document
		setupServer     func() *httptest.Server
		setupLLM        func() *mockLLMClient
		wantErr         bool
		wantErrContains string
	}{
		{
			name:     "successful document storage",
			document: createTestDocument("doc1", "Test document content for embedding"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						if r.Method == "GET" {
							collections := []clients.Collection{
								{ID: "test-doc-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-doc-collection-id/add", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						var req clients.AddDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						// Validate request
						if len(req.IDs) != 1 || req.IDs[0] != "doc1" {
							t.Errorf("Unexpected document ID: %v", req.IDs)
						}
						if len(req.Embeddings) != 1 || len(req.Embeddings[0]) != testEmbeddingDimension {
							t.Errorf("Unexpected embedding dimensions: %d", len(req.Embeddings[0]))
						}
						if len(req.Documents) != 1 || req.Documents[0] != "Test document content for embedding" {
							t.Errorf("Unexpected document content: %v", req.Documents)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			setupLLM: func() *mockLLMClient {
				return &mockLLMClient{}
			},
			wantErr: false,
		},
		{
			name:     "embedding generation failure",
			document: createTestDocument("doc2", "Test content"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						collections := []clients.Collection{
							{ID: "test-doc-collection-id", Name: testDefaultCollection},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(collections)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			setupLLM: func() *mockLLMClient {
				return &mockLLMClient{
					embedFunc: func(ctx context.Context, text string) ([]float64, error) {
						return nil, fmt.Errorf("embedding generation failed")
					},
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate embedding",
		},
		{
			name:     "collection not found",
			document: createTestDocument("doc3", "Test content"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						// Return empty collections list
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode([]clients.Collection{})
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			setupLLM: func() *mockLLMClient {
				return &mockLLMClient{}
			},
			wantErr:         true,
			wantErrContains: "collection test_documents not found",
		},
		{
			name:     "add documents API failure",
			document: createTestDocument("doc4", "Test content"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						collections := []clients.Collection{
							{ID: "test-doc-collection-id", Name: testDefaultCollection},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(collections)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-doc-collection-id/add", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, "Add documents failed")
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			setupLLM: func() *mockLLMClient {
				return &mockLLMClient{}
			},
			wantErr:         true,
			wantErrContains: "failed to store document in ChromaDB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := tt.setupLLM()

			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)

			err := manager.StoreDocument(ctx, tt.document)

			if (err != nil) != tt.wantErr {
				t.Errorf("StoreDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreDocument() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}
		})
	}
}

func TestChromaStorageManager_StoreDocumentInCollection(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name            string
		collection      string
		document        *types.Document
		setupServer     func() *httptest.Server
		wantErr         bool
		wantErrContains string
	}{
		{
			name:       "successful storage in custom collection",
			collection: "custom_collection",
			document:   createTestDocument("doc1", "Test content in custom collection"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						collections := []clients.Collection{
							{ID: "custom-collection-id", Name: "custom_collection"},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(collections)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/custom-collection-id/add", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:       "empty collection name",
			collection: "",
			document:   createTestDocument("doc2", "Test content"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode([]clients.Collection{})
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:         true,
			wantErrContains: "collection  not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)

			err := manager.StoreDocumentInCollection(ctx, tt.collection, tt.document)

			if (err != nil) != tt.wantErr {
				t.Errorf("StoreDocumentInCollection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreDocumentInCollection() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}
		})
	}
}

func TestChromaStorageManager_GetDocument(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name            string
		documentID      string
		setupServer     func() *httptest.Server
		wantErr         bool
		wantErrContains string
		wantDocument    *types.Document
	}{
		{
			name:       "successful document retrieval",
			documentID: "doc1",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						if r.Method == "GET" {
							collections := []clients.Collection{
								{ID: "test-doc-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-doc-collection-id/get", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						var req clients.GetRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "doc1" {
							t.Errorf("Unexpected document IDs in get request: %v", req.IDs)
						}

						response := clients.GetResponse{
							IDs:       []string{"doc1"},
							Documents: []string{"Test document content"},
							Metadatas: []map[string]any{
								{
									"path":         "/test/doc1.txt",
									"name":         "doc1.txt",
									"content_type": "text/plain",
									"size":         float64(100),
									"hash":         "test-hash-doc1",
									"created_at":   time.Now().Format(time.RFC3339),
									"updated_at":   time.Now().Format(time.RFC3339),
								},
							},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
			wantDocument: &types.Document{
				ID:      "doc1",
				Content: "Test document content",
				Path:    "/test/doc1.txt",
				Name:    "doc1.txt",
			},
		},
		{
			name:       "document not found",
			documentID: "nonexistent",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						collections := []clients.Collection{
							{ID: "test-doc-collection-id", Name: testDefaultCollection},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(collections)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-doc-collection-id/get", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						response := clients.GetResponse{
							IDs:       []string{},
							Documents: []string{},
							Metadatas: []map[string]any{},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:         true,
			wantErrContains: "document not found: nonexistent",
		},
		{
			name:       "API failure",
			documentID: "doc2",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						collections := []clients.Collection{
							{ID: "test-doc-collection-id", Name: testDefaultCollection},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(collections)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-doc-collection-id/get", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, "Get documents failed")
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:         true,
			wantErrContains: "failed to get document from ChromaDB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)

			doc, err := manager.GetDocument(ctx, tt.documentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("GetDocument() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if !tt.wantErr && tt.wantDocument != nil {
				if doc == nil {
					t.Fatal("GetDocument() returned nil document")
				}
				if doc.ID != tt.wantDocument.ID {
					t.Errorf("GetDocument() ID = %v, want %v", doc.ID, tt.wantDocument.ID)
				}
				if doc.Content != tt.wantDocument.Content {
					t.Errorf("GetDocument() Content = %v, want %v", doc.Content, tt.wantDocument.Content)
				}
				if doc.Path != tt.wantDocument.Path {
					t.Errorf("GetDocument() Path = %v, want %v", doc.Path, tt.wantDocument.Path)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func TestChromaStorageManager_SearchDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		query       string
		limit       int
		setupServer func() *httptest.Server
		wantErr     bool
		wantCount   int
		wantFirstID string
	}{
		{
			name:  "successful search with results",
			query: "test query",
			limit: 5,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-search-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-search-collection-id/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						var req clients.QueryRequest
						json.NewDecoder(r.Body).Decode(&req)

						if req.NResults != 5 {
							t.Errorf("Unexpected limit in search request: %d", req.NResults)
						}

						response := clients.QueryResponse{
							IDs:       [][]string{{"doc1", "doc2", "doc3"}},
							Documents: [][]string{{"Document 1 content", "Document 2 content", "Document 3 content"}},
							Metadatas: [][]map[string]any{
								{
									{
										"path":         "/test/doc1.txt",
										"name":         "doc1.txt",
										"content_type": "text/plain",
									},
									{
										"path":         "/test/doc2.txt",
										"name":         "doc2.txt",
										"content_type": "text/plain",
									},
									{
										"path":         "/test/doc3.txt",
										"name":         "doc3.txt",
										"content_type": "text/plain",
									},
								},
							},
							Distances: [][]float64{{0.1, 0.2, 0.3}},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:     false,
			wantCount:   3,
			wantFirstID: "doc1",
		},
		{
			name:  "search with no results",
			query: "nonexistent query",
			limit: 5,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-search-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-search-collection-id/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						response := clients.QueryResponse{
							IDs:       [][]string{{}},
							Documents: [][]string{{}},
							Metadatas: [][]map[string]any{{}},
							Distances: [][]float64{{}},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:   false,
			wantCount: 0,
		},
		{
			name:  "API error during search",
			query: "test query",
			limit: 5,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-search-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-search-collection-id/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
		{
			name:  "search with limit 1",
			query: "limited query",
			limit: 1,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-search-collection-id", Name: testDefaultCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-search-collection-id/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						var req clients.QueryRequest
						json.NewDecoder(r.Body).Decode(&req)

						if req.NResults != 1 {
							t.Errorf("Unexpected limit in search request: %d", req.NResults)
						}

						response := clients.QueryResponse{
							IDs:       [][]string{{"doc1"}},
							Documents: [][]string{{"Document 1 content"}},
							Metadatas: [][]map[string]any{
								{
									{
										"path":         "/test/doc1.txt",
										"name":         "doc1.txt",
										"content_type": "text/plain",
									},
								},
							},
							Distances: [][]float64{{0.1}},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:     false,
			wantCount:   1,
			wantFirstID: "doc1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test SearchDocuments
			docs, err := manager.SearchDocuments(ctx, tt.query, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchDocuments() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("SearchDocuments() unexpected error = %v", err)
				return
			}

			if len(docs) != tt.wantCount {
				t.Errorf("SearchDocuments() returned %d documents, want %d", len(docs), tt.wantCount)
				return
			}

			if tt.wantCount > 0 && tt.wantFirstID != "" {
				if docs[0].ID != tt.wantFirstID {
					t.Errorf("SearchDocuments() first document ID = %v, want %v", docs[0].ID, tt.wantFirstID)
				}
			}
		})
	}
}

func TestChromaStorageManager_SearchDocumentsInCollection(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionName string
		query          string
		limit          int
		setupServer    func() *httptest.Server
		wantErr        bool
		wantCount      int
		wantFirstID    string
	}{
		{
			name:           "successful search in specific collection",
			collectionName: "specific_collection",
			query:          "test query",
			limit:          3,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "specific-collection-123", Name: "specific_collection"},
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/specific-collection-123/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						response := clients.QueryResponse{
							IDs:       [][]string{{"spec_doc1", "spec_doc2"}},
							Documents: [][]string{{"Specific Document 1", "Specific Document 2"}},
							Metadatas: [][]map[string]any{
								{
									{
										"path":         "/specific/doc1.txt",
										"name":         "doc1.txt",
										"content_type": "text/plain",
									},
									{
										"path":         "/specific/doc2.txt",
										"name":         "doc2.txt",
										"content_type": "text/plain",
									},
								},
							},
							Distances: [][]float64{{0.1, 0.2}},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:     false,
			wantCount:   2,
			wantFirstID: "spec_doc1",
		},
		{
			name:           "collection not found",
			collectionName: "nonexistent_collection",
			query:          "test query",
			limit:          3,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
		{
			name:           "empty query string",
			collectionName: "test_collection",
			query:          "",
			limit:          5,
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-collection-123", Name: "test_collection"},
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-collection-123/query", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						response := clients.QueryResponse{
							IDs:       [][]string{{}},
							Documents: [][]string{{}},
							Metadatas: [][]map[string]any{{}},
							Distances: [][]float64{{}},
						}
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(response)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test SearchDocumentsInCollection
			docs, err := manager.SearchDocumentsInCollection(ctx, tt.collectionName, tt.query, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchDocumentsInCollection() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("SearchDocumentsInCollection() unexpected error = %v", err)
				return
			}

			if len(docs) != tt.wantCount {
				t.Errorf("SearchDocumentsInCollection() returned %d documents, want %d", len(docs), tt.wantCount)
				return
			}

			if tt.wantCount > 0 && tt.wantFirstID != "" {
				if docs[0].ID != tt.wantFirstID {
					t.Errorf("SearchDocumentsInCollection() first document ID = %v, want %v", docs[0].ID, tt.wantFirstID)
				}
			}
		})
	}
}

func TestChromaStorageManager_StoreEntity(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		entity      *types.Entity
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name:   "successful entity storage",
			entity: createTestEntity("entity1", "John Doe", types.EntityTypePerson),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testEntityCollection): func(w http.ResponseWriter, r *http.Request) {
						var req clients.AddDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "entity1" {
							t.Errorf("Unexpected entity ID in request: %v", req.IDs)
						}
						if len(req.Documents) != 1 || req.Documents[0] != "John Doe" {
							t.Errorf("Unexpected entity text in request: %v", req.Documents)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:   "API error during entity storage",
			entity: createTestEntity("entity2", "Jane Smith", types.EntityTypeOrganization),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testEntityCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test StoreEntity
			err = manager.StoreEntity(ctx, tt.entity)

			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreEntity() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("StoreEntity() unexpected error = %v", err)
			}
		})
	}
}

func TestChromaStorageManager_StoreCitation(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		citation    *types.Citation
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name:     "successful citation storage",
			citation: createTestCitation("citation1", "This is a citation from document"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testCitationCollection): func(w http.ResponseWriter, r *http.Request) {
						var req clients.AddDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "citation1" {
							t.Errorf("Unexpected citation ID in request: %v", req.IDs)
						}
						if len(req.Documents) != 1 || req.Documents[0] != "This is a citation from document" {
							t.Errorf("Unexpected citation text in request: %v", req.Documents)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:     "API error during citation storage",
			citation: createTestCitation("citation2", "Another citation"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testCitationCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test StoreCitation
			err = manager.StoreCitation(ctx, tt.citation)

			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreCitation() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("StoreCitation() unexpected error = %v", err)
			}
		})
	}
}

func TestChromaStorageManager_StoreTopic(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		topic       *types.Topic
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name:  "successful topic storage",
			topic: createTestTopic("topic1", "Machine Learning"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testTopicCollection): func(w http.ResponseWriter, r *http.Request) {
						var req clients.AddDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "topic1" {
							t.Errorf("Unexpected topic ID in request: %v", req.IDs)
						}
						if len(req.Documents) != 1 || req.Documents[0] != "Machine Learning: Test topic description" {
							t.Errorf("Unexpected topic name in request: %v", req.Documents)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:  "API error during topic storage",
			topic: createTestTopic("topic2", "Data Science"),
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/add", testTenant, testDatabase, testTopicCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test StoreTopic
			err = manager.StoreTopic(ctx, tt.topic)

			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreTopic() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("StoreTopic() unexpected error = %v", err)
			}
		})
	}
}

func TestChromaStorageManager_DeleteDocument(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		documentID  string
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name:       "successful document deletion",
			documentID: "doc1",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/delete", testTenant, testDatabase, testDefaultCollection): func(w http.ResponseWriter, r *http.Request) {
						var req clients.DeleteDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "doc1" {
							t.Errorf("Unexpected document ID in delete request: %v", req.IDs)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:       "API error during document deletion",
			documentID: "doc2",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/delete", testTenant, testDatabase, testDefaultCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test DeleteDocument
			err = manager.DeleteDocument(ctx, tt.documentID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteDocument() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("DeleteDocument() unexpected error = %v", err)
			}
		})
	}
}

func TestChromaStorageManager_DeleteDocumentFromCollection(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionName string
		documentID     string
		setupServer    func() *httptest.Server
		wantErr        bool
	}{
		{
			name:           "successful document deletion from specific collection",
			collectionName: "custom_collection",
			documentID:     "doc1",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "custom-collection-123", Name: "custom_collection"},
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/custom-collection-123/delete", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						var req clients.DeleteDocumentsRequest
						json.NewDecoder(r.Body).Decode(&req)

						if len(req.IDs) != 1 || req.IDs[0] != "doc1" {
							t.Errorf("Unexpected document ID in delete request: %v", req.IDs)
						}

						w.WriteHeader(http.StatusOK)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name:           "collection not found",
			collectionName: "nonexistent_collection",
			documentID:     "doc1",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test DeleteDocumentFromCollection
			err = manager.DeleteDocumentFromCollection(ctx, tt.collectionName, tt.documentID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteDocumentFromCollection() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("DeleteDocumentFromCollection() unexpected error = %v", err)
			}
		})
	}
}

func TestChromaStorageManager_ListCollections(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		want        []string
		wantErr     bool
	}{
		{
			name: "successful collection listing",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
								{ID: "custom-123", Name: "custom_collection"},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					// Count endpoints for all collections
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testDefaultCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("10"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testEntityCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("5"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testCitationCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("3"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testTopicCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("2"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/custom-123/count", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("7"))
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			want:    []string{testDefaultCollection, testEntityCollection, testCitationCollection, testTopicCollection, "custom_collection"},
			wantErr: false,
		},
		{
			name: "API error during collection listing",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							w.WriteHeader(http.StatusInternalServerError)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil && !tt.wantErr {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test ListCollections
			result, err := manager.ListCollections(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ListCollections() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ListCollections() unexpected error = %v", err)
				return
			}

			// Check result
			if len(result) != len(tt.want) {
				t.Errorf("ListCollections() expected %d collections, got %d", len(tt.want), len(result))
				return
			}

			// Convert to map for easier comparison
			gotMap := make(map[string]bool)
			for _, collection := range result {
				gotMap[collection.Name] = true
			}

			for _, expected := range tt.want {
				if !gotMap[expected] {
					t.Errorf("ListCollections() missing expected collection: %s", expected)
				}
			}
		})
	}
}

func TestChromaStorageManager_GetStats(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantCount   int
		wantErr     bool
	}{
		{
			name: "successful stats retrieval",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testDefaultCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("25"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testEntityCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("10"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testCitationCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("15"))
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testTopicCollection): func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.Write([]byte("5"))
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantCount: 25,
			wantErr:   false,
		},
		{
			name: "API error during stats retrieval - count operations fail but stats succeeds",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testDefaultCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testEntityCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testCitationCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/test-id-%s/count", testTenant, testDatabase, testTopicCollection): func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantCount: 0,     // Count operations fail, so all counts will be 0
			wantErr:   false, // GetStats should still succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			// Test GetStats
			result, err := manager.GetStats(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetStats() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetStats() unexpected error = %v", err)
				return
			}

			// Check result
			if result == nil {
				t.Errorf("GetStats() returned nil result")
				return
			}

			// Check collections count
			if len(result.Collections) != 4 { // 4 default collections
				t.Errorf("GetStats() expected 4 collections, got %d", len(result.Collections))
			}

			// Find the default collection and check its document count
			var foundDefaultCollection bool
			for _, collection := range result.Collections {
				if collection.Name == testDefaultCollection {
					foundDefaultCollection = true
					if collection.Documents != tt.wantCount {
						t.Errorf("GetStats() expected document count %d in default collection, got %d", tt.wantCount, collection.Documents)
					}
					break
				}
			}

			if !foundDefaultCollection {
				t.Errorf("GetStats() default collection not found in results")
			}

			// Verify metrics are present
			if result.Metrics == nil {
				t.Errorf("GetStats() Metrics should not be nil")
			}
		})
	}
}

func TestChromaStorageManager_Close(t *testing.T) {
	ctx := t.Context()
	server := createMockChromaDBServer(t, map[string]http.HandlerFunc{
		fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				collections := []clients.Collection{
					{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
					{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
					{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
					{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(collections)
			case "POST":
				// CreateCollection
				var req clients.CreateCollectionRequest
				json.NewDecoder(r.Body).Decode(&req)

				collection := clients.Collection{
					ID:       "test-id-" + req.Name,
					Name:     req.Name,
					Metadata: req.Metadata,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(collection)
			}
		},
	})
	defer server.Close()

	// Create ChromaDB client and mock LLM client
	chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
	mockLLM := &mockLLMClient{}

	// Create storage manager
	manager := NewChromaStorageManager(
		chromaClient,
		mockLLM,
		testDefaultCollection,
		testEntityCollection,
		testCitationCollection,
		testTopicCollection,
		testEmbeddingDimension,
		testMaxRetries,
		testRetryDelay,
	)

	// Initialize
	err := manager.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Test that Close can be called multiple times without issues
	manager.Close()
	manager.Close() // Should not panic or cause issues

	// Test that operations after Close still work (manager should be stateless)
	// Note: We don't test operations after Close because it may depend on implementation
}

func TestChromaStorageManager_Health(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		wantErr     bool
	}{
		{
			name: "healthy storage manager",
			setupServer: func() *httptest.Server {
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: false,
		},
		{
			name: "unhealthy storage manager - API error",
			setupServer: func() *httptest.Server {
				callCount := 0
				handlers := map[string]http.HandlerFunc{
					fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase): func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							// During initialization, succeed
							collections := []clients.Collection{
								{ID: "test-id-" + testDefaultCollection, Name: testDefaultCollection},
								{ID: "test-id-" + testEntityCollection, Name: testEntityCollection},
								{ID: "test-id-" + testCitationCollection, Name: testCitationCollection},
								{ID: "test-id-" + testTopicCollection, Name: testTopicCollection},
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collections)
						case "POST":
							// CreateCollection
							var req clients.CreateCollectionRequest
							json.NewDecoder(r.Body).Decode(&req)

							collection := clients.Collection{
								ID:       "test-id-" + req.Name,
								Name:     req.Name,
								Metadata: req.Metadata,
							}
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(collection)
						}
					},
					"/api/v2/heartbeat": func(w http.ResponseWriter, r *http.Request) {
						callCount++
						if callCount > 1 {
							// Health check during Health() method - fail
							w.WriteHeader(http.StatusInternalServerError)
						} else {
							// Health check during Initialize - succeed
							w.WriteHeader(http.StatusOK)
						}
					},
				}
				return createMockChromaDBServer(t, handlers)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			// Create ChromaDB client and mock LLM client
			chromaClient := clients.NewChromaDBClient(server.URL, testTimeout, testMaxRetries, testTenant, testDatabase)
			mockLLM := &mockLLMClient{}

			// Create storage manager
			manager := NewChromaStorageManager(
				chromaClient,
				mockLLM,
				testDefaultCollection,
				testEntityCollection,
				testCitationCollection,
				testTopicCollection,
				testEmbeddingDimension,
				testMaxRetries,
				testRetryDelay,
			)
			defer manager.Close()

			// Initialize
			err := manager.Initialize(ctx)
			if err != nil && !tt.wantErr {
				t.Fatalf("Initialize() error = %v", err)
			}

			// If Initialize failed and we expect an error, that's fine for some health tests
			if err != nil && tt.wantErr {
				// Health check failed during initialization - this covers the error path
				return
			}

			// Test Health
			err = manager.Health(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Health() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Health() unexpected error = %v", err)
			}
		})
	}
}
