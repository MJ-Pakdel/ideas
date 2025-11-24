package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// TestChromaStorageManagerWithMocks demonstrates the new mocking approach
// This is a proof of concept showing how we can test storage operations without real ChromaDB
func TestChromaStorageManagerWithMocks(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*interfaces.MockChromaDBClient)
		setupLLM      func(*mockLLMClient)
		operation     func(context.Context, *ChromaStorageManager) error
		expectedError string
		expectedCalls map[string]int
	}{
		{
			name: "health_check_success",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Mock will return nil (success) by default
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in health check
			},
			operation: func(ctx context.Context, storage *ChromaStorageManager) error {
				// Test the health check operation used in Initialize
				return storage.Health(ctx)
			},
			expectedCalls: map[string]int{
				"Health": 1,
			},
		},
		{
			name: "health_check_failure",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				mock.WithHealthError(errors.New("connection failed"))
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in health check
			},
			operation: func(ctx context.Context, storage *ChromaStorageManager) error {
				return storage.Health(ctx)
			},
			expectedError: "connection failed",
			expectedCalls: map[string]int{
				"Health": 1,
			},
		},
		{
			name: "health_check_with_latency",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				mock.WithLatency(50 * time.Millisecond)
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in health check
			},
			operation: func(ctx context.Context, storage *ChromaStorageManager) error {
				start := time.Now()
				err := storage.Health(ctx)
				elapsed := time.Since(start)

				// Verify latency was simulated
				if elapsed < 45*time.Millisecond {
					t.Errorf("Expected latency simulation, but operation completed too quickly: %v", elapsed)
				}
				return err
			},
			expectedCalls: map[string]int{
				"Health": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager using new injectable constructor
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_docs", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test through the storage manager which now uses mocks
			ctx := context.Background()

			// Execute operation
			var err error
			if tt.operation != nil {
				err = tt.operation(ctx, storage)
			}

			// Assert error expectations
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, but got nil", tt.expectedError)
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, but got %q", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got %v", err)
				}
			}

			// Verify call counts
			for method, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(method)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, method, actualCount)
				}
			}
		})
	}
}

// TestMockChromaDBClient_Collections tests the collection management functionality
func TestMockChromaDBClient_Collections(t *testing.T) {
	mock := interfaces.NewMockChromaDBClient()
	ctx := t.Context()

	// Test creating a collection
	collection, err := mock.CreateCollection(ctx, "test_collection", map[string]any{"test": "metadata"})
	if err != nil {
		t.Fatalf("Expected no error creating collection, got %v", err)
	}

	if collection.Name != "test_collection" {
		t.Errorf("Expected collection name 'test_collection', got %q", collection.Name)
	}

	// Test getting the collection
	retrieved, err := mock.GetCollection(ctx, "test_collection")
	if err != nil {
		t.Fatalf("Expected no error getting collection, got %v", err)
	}

	if retrieved.Name != collection.Name {
		t.Errorf("Expected retrieved collection name %q, got %q", collection.Name, retrieved.Name)
	}

	// Test listing collections
	list, err := mock.ListCollections(ctx)
	if err != nil {
		t.Fatalf("Expected no error listing collections, got %v", err)
	}

	if len(list.Collections) != 1 {
		t.Errorf("Expected 1 collection, got %d", len(list.Collections))
	}

	// Verify call counts
	expectedCalls := map[string]int{
		"CreateCollection": 1,
		"GetCollection":    1,
		"ListCollections":  1,
	}

	for method, expectedCount := range expectedCalls {
		actualCount := mock.GetCallCount(method)
		if actualCount != expectedCount {
			t.Errorf("Expected %d calls to %s, but got %d", expectedCount, method, actualCount)
		}
	}
}

// TestMockChromaDBClient_Documents tests document operations
func TestMockChromaDBClient_Documents(t *testing.T) {
	mock := interfaces.NewMockChromaDBClient()
	ctx := context.Background()

	// Create a collection first
	collection, err := mock.CreateCollection(ctx, "test_docs", nil)
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	// Add documents
	request := interfaces.AddDocumentsRequest{
		IDs:        []string{"doc1", "doc2"},
		Documents:  []string{"content1", "content2"},
		Embeddings: [][]float64{{0.1, 0.2}, {0.3, 0.4}},
		Metadatas:  []map[string]any{{"type": "test"}, {"type": "test"}},
	}

	err = mock.AddDocuments(ctx, collection.ID, request)
	if err != nil {
		t.Fatalf("Expected no error adding documents, got %v", err)
	}

	// Get documents
	response, err := mock.GetDocuments(ctx, collection.ID, []string{"doc1"}, []string{"documents", "metadatas"})
	if err != nil {
		t.Fatalf("Expected no error getting documents, got %v", err)
	}

	if len(response.IDs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(response.IDs))
	}

	if response.IDs[0] != "doc1" {
		t.Errorf("Expected document ID 'doc1', got %q", response.IDs[0])
	}

	// Query documents
	queryRequest := interfaces.QueryRequest{
		QueryEmbeddings: [][]float64{{0.1, 0.2}},
		NResults:        10,
	}

	queryResponse, err := mock.QueryDocuments(ctx, collection.ID, queryRequest)
	if err != nil {
		t.Fatalf("Expected no error querying documents, got %v", err)
	}

	if len(queryResponse.IDs) == 0 || len(queryResponse.IDs[0]) != 2 {
		t.Errorf("Expected 2 documents in query results, got %v", queryResponse.IDs)
	}
}

// TestMockChromaDBClient_ErrorSimulation tests error simulation capabilities
func TestMockChromaDBClient_ErrorSimulation(t *testing.T) {
	mock := interfaces.NewMockChromaDBClient()
	ctx := context.Background()

	// Configure mock to return error for CreateCollection
	testError := errors.New("simulated creation error")
	mock.ForceErrors["CreateCollection"] = testError

	// Try to create collection
	_, err := mock.CreateCollection(ctx, "test", nil)
	if err == nil {
		t.Fatal("Expected error, but got nil")
	}

	if err != testError {
		t.Errorf("Expected specific test error, got %v", err)
	}

	// Clear error and try again
	delete(mock.ForceErrors, "CreateCollection")

	_, err = mock.CreateCollection(ctx, "test", nil)
	if err != nil {
		t.Errorf("Expected no error after clearing forced error, got %v", err)
	}
}

// TestChromaStorageManager_Initialize_MockBased tests the Initialize method using mocks
// This replaces the timeout-prone TestChromaStorageManager_Initialize
func TestChromaStorageManager_Initialize_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		expectedCalls   map[string]int
	}{
		{
			name: "successful_initialization",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Health check succeeds
				// ListCollections returns empty initially
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
				// CreateCollection succeeds for all collections
				mock.CreateCollectionFunc = func(ctx context.Context, name string, metadata map[string]any) (*interfaces.Collection, error) {
					return &interfaces.Collection{
						ID:       "test-id-" + name,
						Name:     name,
						Metadata: metadata,
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in Initialize
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"Health":           1,
				"ListCollections":  4, // Called once for each collection to check existence
				"CreateCollection": 4, // Four collections: docs, entities, citations, topics
			},
		},
		{
			name: "health_check_failure",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				mock.WithHealthError(errors.New("connection failed"))
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used when health check fails
			},
			wantErr:         true,
			wantErrContains: "ChromaDB health check failed",
			expectedCalls: map[string]int{
				"Health": 1,
				// No other calls should be made after health check fails
			},
		},
		{
			name: "collections_already_exist",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Health check succeeds
				// ListCollections returns existing collections
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "existing-id-1", Name: "test_documents"},
							{ID: "existing-id-2", Name: "test_entities"},
							{ID: "existing-id-3", Name: "test_citations"},
							{ID: "existing-id-4", Name: "test_topics"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in Initialize
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"Health":          1,
				"ListCollections": 4, // Called once for each collection
				// CreateCollection should not be called since all collections exist
			},
		},
		{
			name: "partial_collection_creation_failure",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Health check succeeds
				// ListCollections returns no collections
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
				// CreateCollection fails for the second collection
				callCount := 0
				mock.CreateCollectionFunc = func(ctx context.Context, name string, metadata map[string]any) (*interfaces.Collection, error) {
					callCount++
					if callCount == 2 {
						return nil, errors.New("failed to create collection: " + name)
					}
					return &interfaces.Collection{
						ID:       "test-id-" + name,
						Name:     name,
						Metadata: metadata,
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in Initialize
			},
			wantErr:         true,
			wantErrContains: "failed to create collection",
			expectedCalls: map[string]int{
				"Health":           1,
				"ListCollections":  2, // Called once for each collection before creation attempt
				"CreateCollection": 2, // First succeeds, second fails
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test Initialize method
			ctx := context.Background()
			err := storage.Initialize(ctx)

			// Assert error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("Initialize() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// TestChromaStorageManager_StoreDocument_MockBased tests document storage using mocks
// This replaces the timeout-prone TestChromaStorageManager_StoreDocument
func TestChromaStorageManager_StoreDocument_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		document        *mockDocument
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		expectedCalls   map[string]int
	}{
		{
			name: "successful_document_storage",
			document: &mockDocument{
				ID:      "doc1",
				Content: "Test document content for embedding",
				Metadata: map[string]any{
					"title": "Test Document",
				},
			},
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// AddDocuments succeeds
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					// Validate the request
					if len(request.IDs) != 1 || request.IDs[0] != "doc1" {
						return fmt.Errorf("unexpected document ID: %v", request.IDs)
					}
					if len(request.Embeddings) != 1 || len(request.Embeddings[0]) != 384 {
						return fmt.Errorf("unexpected embedding dimensions: %d", len(request.Embeddings[0]))
					}
					if len(request.Documents) != 1 || request.Documents[0] != "Test document content for embedding" {
						return fmt.Errorf("unexpected document content: %v", request.Documents)
					}
					return nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01 // Generate test embedding
					}
					return embedding, nil
				}
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
		{
			name: "embedding_generation_failure",
			document: &mockDocument{
				ID:      "doc2",
				Content: "Test content",
			},
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Collection exists but we should never reach AddDocuments
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function fails
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					return nil, fmt.Errorf("embedding generation failed")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate embedding",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to embedding failure
			},
		},
		{
			name: "collection_not_found",
			document: &mockDocument{
				ID:      "doc3",
				Content: "Test content",
			},
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Return empty collections list
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM setup but won't be called
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "collection test_documents not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to missing collection
			},
		},
		{
			name: "add_documents_API_failure",
			document: &mockDocument{
				ID:      "doc4",
				Content: "Test content",
			},
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// Collection exists
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// AddDocuments fails
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					return fmt.Errorf("Add documents failed")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function succeeds
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "failed to store document in ChromaDB",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test StoreDocument method
			ctx := context.Background()

			// Convert mock document to types.Document
			doc := &types.Document{
				ID:       tt.document.ID,
				Content:  tt.document.Content,
				Metadata: tt.document.Metadata,
			}

			err := storage.StoreDocument(ctx, doc)

			// Assert error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("StoreDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrContains != "" {
				if err == nil || !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreDocument() error = %v, want error containing %q", err, tt.wantErrContains)
				}
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// mockDocument represents a test document structure
type mockDocument struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// TestChromaStorageManager_GetDocument_MockBased tests document retrieval using mocks
// This replaces the timeout-prone TestChromaStorageManager_GetDocument
func TestChromaStorageManager_GetDocument_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		documentID      string
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		wantDocument    *types.Document
		expectedCalls   map[string]int
	}{
		{
			name:       "successful_document_retrieval",
			documentID: "doc1",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// GetDocuments returns the requested document
				mock.GetDocumentsFunc = func(ctx context.Context, collectionID string, ids []string, include []string) (*interfaces.GetDocumentsResponse, error) {
					if len(ids) != 1 || ids[0] != "doc1" {
						return nil, fmt.Errorf("unexpected document IDs: %v", ids)
					}
					return &interfaces.GetDocumentsResponse{
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
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in GetDocument
			},
			wantErr: false,
			wantDocument: &types.Document{
				ID:      "doc1",
				Content: "Test document content",
				Path:    "/test/doc1.txt",
				Name:    "doc1.txt",
			},
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"GetDocuments":    1,
			},
		},
		{
			name:       "document_not_found",
			documentID: "nonexistent",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// GetDocuments returns empty response (document not found)
				mock.GetDocumentsFunc = func(ctx context.Context, collectionID string, ids []string, include []string) (*interfaces.GetDocumentsResponse, error) {
					return &interfaces.GetDocumentsResponse{
						IDs:       []string{},
						Documents: []string{},
						Metadatas: []map[string]any{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in GetDocument
			},
			wantErr:         true,
			wantErrContains: "document not found: nonexistent",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"GetDocuments":    1,
			},
		},
		{
			name:       "API_failure",
			documentID: "doc2",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-doc-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// GetDocuments fails
				mock.GetDocumentsFunc = func(ctx context.Context, collectionID string, ids []string, include []string) (*interfaces.GetDocumentsResponse, error) {
					return nil, fmt.Errorf("Get documents failed")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in GetDocument
			},
			wantErr:         true,
			wantErrContains: "failed to get document from ChromaDB",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"GetDocuments":    1,
			},
		},
		{
			name:       "collection_not_found",
			documentID: "doc3",
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns empty list (no collections)
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// LLM not used in GetDocument
			},
			wantErr:         true,
			wantErrContains: "collection test_documents not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// GetDocuments should not be called due to missing collection
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test GetDocument method
			ctx := context.Background()
			doc, err := storage.GetDocument(ctx, tt.documentID)

			// Assert error expectations
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

			// Assert document expectations for success cases
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
				if doc.Name != tt.wantDocument.Name {
					t.Errorf("GetDocument() Name = %v, want %v", doc.Name, tt.wantDocument.Name)
				}
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// TestChromaStorageManager_SearchDocuments_MockBased tests document search using mocks
// This replaces the timeout-prone TestChromaStorageManager_SearchDocuments
func TestChromaStorageManager_SearchDocuments_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		limit           int
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		wantCount       int
		wantFirstID     string
		expectedCalls   map[string]int
	}{
		{
			name:  "successful_search_with_results",
			query: "test query",
			limit: 5,
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-search-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// QueryDocuments returns search results
				mock.QueryDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.QueryRequest) (*interfaces.QueryResponse, error) {
					if request.NResults != 5 {
						return nil, fmt.Errorf("unexpected limit in search request: %d", request.NResults)
					}
					return &interfaces.QueryResponse{
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
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding for search query
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01 // Generate test embedding
					}
					return embedding, nil
				}
			},
			wantErr:     false,
			wantCount:   3,
			wantFirstID: "doc1",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"QueryDocuments":  1,
			},
		},
		{
			name:  "embedding_generation_failure",
			query: "test query",
			limit: 5,
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-search-collection-id", Name: "test_documents"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function fails
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					return nil, fmt.Errorf("embedding generation failed")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate query embedding",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// QueryDocuments should not be called due to embedding failure
			},
		},
		{
			name:  "collection_not_found",
			query: "test query",
			limit: 5,
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns empty list
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "collection test_documents not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// QueryDocuments should not be called due to collection not found
			},
		},
		{
			name:  "chroma_query_api_error",
			query: "test query",
			limit: 5,
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-search-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// QueryDocuments returns API error
				mock.QueryDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.QueryRequest) (*interfaces.QueryResponse, error) {
					return nil, fmt.Errorf("ChromaDB API error: internal server error")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "ChromaDB API error",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"QueryDocuments":  1,
			},
		},
		{
			name:  "empty_search_results",
			query: "nonexistent query",
			limit: 5,
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-search-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// QueryDocuments returns empty results
				mock.QueryDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.QueryRequest) (*interfaces.QueryResponse, error) {
					return &interfaces.QueryResponse{
						IDs:       [][]string{{}},
						Documents: [][]string{{}},
						Metadatas: [][]map[string]any{{}},
						Distances: [][]float64{{}},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:   false,
			wantCount: 0,
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"QueryDocuments":  1,
			},
		},
		{
			name:  "large_limit_clamping",
			query: "test query",
			limit: 1000, // Above typical API limits
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns the default collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-search-collection-id", Name: "test_documents"},
						},
					}, nil
				}
				// QueryDocuments handles large limit gracefully
				mock.QueryDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.QueryRequest) (*interfaces.QueryResponse, error) {
					// Verify the limit is passed through correctly
					if request.NResults != 1000 {
						return nil, fmt.Errorf("unexpected limit in search request: %d", request.NResults)
					}
					// Return fewer results than requested (realistic scenario)
					return &interfaces.QueryResponse{
						IDs:       [][]string{{"doc1", "doc2"}},
						Documents: [][]string{{"Document 1 content", "Document 2 content"}},
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
							},
						},
						Distances: [][]float64{{0.1, 0.2}},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:     false,
			wantCount:   2,
			wantFirstID: "doc1",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"QueryDocuments":  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test SearchDocuments method
			ctx := context.Background()
			docs, err := storage.SearchDocuments(ctx, tt.query, tt.limit)

			// Assert error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("SearchDocuments() expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("SearchDocuments() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Errorf("SearchDocuments() unexpected error = %v", err)
				return
			}

			// Assert result expectations
			if len(docs) != tt.wantCount {
				t.Errorf("SearchDocuments() returned %d documents, want %d", len(docs), tt.wantCount)
				return
			}

			if tt.wantCount > 0 && tt.wantFirstID != "" {
				if docs[0].ID != tt.wantFirstID {
					t.Errorf("SearchDocuments() first document ID = %v, want %v", docs[0].ID, tt.wantFirstID)
				}
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// TestChromaStorageManager_StoreEntity_MockBased tests entity storage using mocks
// This replaces the timeout-prone TestChromaStorageManager_StoreEntity
func TestChromaStorageManager_StoreEntity_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		entity          *types.Entity
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		expectedCalls   map[string]int
	}{
		{
			name:   "successful_entity_storage",
			entity: createTestEntity("entity1", "John Doe", types.EntityTypePerson),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns entities collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-entities-collection-id", Name: "test_entities"},
						},
					}, nil
				}
				// AddDocuments succeeds for entity storage
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					if len(request.IDs) != 1 || request.IDs[0] != "entity1" {
						return fmt.Errorf("unexpected entity ID: %v", request.IDs)
					}
					if len(request.Documents) != 1 || request.Documents[0] != "John Doe" {
						return fmt.Errorf("unexpected entity text: %v", request.Documents)
					}
					// Verify metadata contains entity information
					if len(request.Metadatas) != 1 {
						return fmt.Errorf("expected metadata for entity")
					}
					metadata := request.Metadatas[0]
					if metadata["type"] != string(types.EntityTypePerson) {
						return fmt.Errorf("expected type in metadata")
					}
					return nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding for entity text
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
		{
			name:   "embedding_generation_failure",
			entity: createTestEntity("entity2", "Jane Smith", types.EntityTypeOrganization),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns entities collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-entities-collection-id", Name: "test_entities"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function fails
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					return nil, fmt.Errorf("embedding service unavailable")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate entity embedding",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to embedding failure
			},
		},
		{
			name:   "entities_collection_not_found",
			entity: createTestEntity("entity3", "Test Location", types.EntityTypeLocation),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns empty list (no entities collection)
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "collection test_entities not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to collection not found
			},
		},
		{
			name:   "add_documents_api_failure",
			entity: createTestEntity("entity4", "API Test Entity", types.EntityTypePerson),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns entities collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-entities-collection-id", Name: "test_entities"},
						},
					}, nil
				}
				// AddDocuments fails with API error
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					return fmt.Errorf("ChromaDB API error: internal server error")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "ChromaDB API error",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test StoreEntity method
			ctx := context.Background()
			err := storage.StoreEntity(ctx, tt.entity)

			// Assert error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreEntity() expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreEntity() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Errorf("StoreEntity() unexpected error = %v", err)
				return
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// TestChromaStorageManager_StoreCitation_MockBased tests citation storage using mocks
// This replaces the timeout-prone TestChromaStorageManager_StoreCitation
func TestChromaStorageManager_StoreCitation_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		citation        *types.Citation
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		expectedCalls   map[string]int
	}{
		{
			name:     "successful_citation_storage",
			citation: createTestCitation("citation1", "This is a citation from document"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns citations collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-citations-collection-id", Name: "test_citations"},
						},
					}, nil
				}
				// AddDocuments succeeds for citation storage
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					if len(request.IDs) != 1 || request.IDs[0] != "citation1" {
						return fmt.Errorf("unexpected citation ID: %v", request.IDs)
					}
					if len(request.Documents) != 1 || request.Documents[0] != "This is a citation from document" {
						return fmt.Errorf("unexpected citation text: %v", request.Documents)
					}
					// Verify metadata contains citation information
					if len(request.Metadatas) != 1 {
						return fmt.Errorf("expected metadata for citation")
					}
					metadata := request.Metadatas[0]
					if metadata["format"] != string(types.CitationFormatAPA) {
						return fmt.Errorf("expected format in metadata")
					}
					if metadata["document_id"] != "test-doc-1" {
						return fmt.Errorf("expected document_id in metadata")
					}
					return nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding for citation text
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
		{
			name:     "embedding_generation_failure",
			citation: createTestCitation("citation2", "Another citation"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns citations collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-citations-collection-id", Name: "test_citations"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function fails
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					return nil, fmt.Errorf("embedding service unavailable")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate citation embedding",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to embedding failure
			},
		},
		{
			name:     "citations_collection_not_found",
			citation: createTestCitation("citation3", "Test citation missing collection"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns empty list (no citations collection)
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "collection test_citations not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to collection not found
			},
		},
		{
			name:     "add_documents_api_failure",
			citation: createTestCitation("citation4", "API Test Citation"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns citations collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-citations-collection-id", Name: "test_citations"},
						},
					}, nil
				}
				// AddDocuments fails with API error
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					return fmt.Errorf("ChromaDB API error: internal server error")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "ChromaDB API error",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test StoreCitation method
			ctx := context.Background()
			err := storage.StoreCitation(ctx, tt.citation)

			// Assert error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreCitation() expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreCitation() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Errorf("StoreCitation() unexpected error = %v", err)
				return
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}

// TestChromaStorageManager_StoreTopic_MockBased tests topic storage using mocks
// This replaces the timeout-prone TestChromaStorageManager_StoreTopic
func TestChromaStorageManager_StoreTopic_MockBased(t *testing.T) {
	tests := []struct {
		name            string
		topic           *types.Topic
		setupMock       func(*interfaces.MockChromaDBClient)
		setupLLM        func(*mockLLMClient)
		wantErr         bool
		wantErrContains string
		expectedCalls   map[string]int
	}{
		{
			name:  "successful_topic_storage",
			topic: createTestTopic("topic1", "Machine Learning"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns topics collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-topics-collection-id", Name: "test_topics"},
						},
					}, nil
				}
				// AddDocuments succeeds for topic storage
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					if len(request.IDs) != 1 || request.IDs[0] != "topic1" {
						return fmt.Errorf("unexpected topic ID: %v", request.IDs)
					}
					// Topic content should be "Name: Description" format
					expectedContent := "Machine Learning: Test topic description"
					if len(request.Documents) != 1 || request.Documents[0] != expectedContent {
						return fmt.Errorf("unexpected topic content: %v, expected: %s", request.Documents, expectedContent)
					}
					// Verify metadata contains topic information
					if len(request.Metadatas) != 1 {
						return fmt.Errorf("expected metadata for topic")
					}
					metadata := request.Metadatas[0]
					if metadata["name"] != "Machine Learning" {
						return fmt.Errorf("expected name in metadata")
					}
					if metadata["description"] != "Test topic description" {
						return fmt.Errorf("expected description in metadata")
					}
					if metadata["document_id"] != "test-doc-1" {
						return fmt.Errorf("expected document_id in metadata")
					}
					return nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding for topic content
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					// Should receive "Machine Learning: Test topic description"
					expectedText := "Machine Learning: Test topic description"
					if text != expectedText {
						return nil, fmt.Errorf("unexpected text for embedding: %s, expected: %s", text, expectedText)
					}
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr: false,
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
		{
			name:  "embedding_generation_failure",
			topic: createTestTopic("topic2", "Data Science"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns topics collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-topics-collection-id", Name: "test_topics"},
						},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function fails
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					return nil, fmt.Errorf("embedding service unavailable")
				}
			},
			wantErr:         true,
			wantErrContains: "failed to generate topic embedding",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to embedding failure
			},
		},
		{
			name:  "topics_collection_not_found",
			topic: createTestTopic("topic3", "Test Topic"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns empty list (no topics collection)
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{},
					}, nil
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "collection test_topics not found",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				// AddDocuments should not be called due to collection not found
			},
		},
		{
			name:  "add_documents_api_failure",
			topic: createTestTopic("topic4", "API Test Topic"),
			setupMock: func(mock *interfaces.MockChromaDBClient) {
				// ListCollections returns topics collection
				mock.ListCollectionsFunc = func(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
					return &interfaces.ListCollectionsResponse{
						Collections: []*interfaces.Collection{
							{ID: "test-topics-collection-id", Name: "test_topics"},
						},
					}, nil
				}
				// AddDocuments fails with API error
				mock.AddDocumentsFunc = func(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
					return fmt.Errorf("ChromaDB API error: internal server error")
				}
			},
			setupLLM: func(llm *mockLLMClient) {
				// Embed function returns valid embedding
				llm.embedFunc = func(ctx context.Context, text string) ([]float64, error) {
					embedding := make([]float64, 384)
					for i := range embedding {
						embedding[i] = float64(i) * 0.01
					}
					return embedding, nil
				}
			},
			wantErr:         true,
			wantErrContains: "ChromaDB API error",
			expectedCalls: map[string]int{
				"ListCollections": 1,
				"AddDocuments":    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockChromaClient := interfaces.NewMockChromaDBClient()
			mockLLMClient := &mockLLMClient{}

			if tt.setupMock != nil {
				tt.setupMock(mockChromaClient)
			}

			if tt.setupLLM != nil {
				tt.setupLLM(mockLLMClient)
			}

			// Create storage manager with dependency injection
			storage := NewChromaStorageManagerWithClient(
				mockChromaClient,
				mockLLMClient,
				"test_documents", "test_entities", "test_citations", "test_topics",
				384, 3, 100*time.Millisecond,
			)

			// Test StoreTopic method
			ctx := context.Background()
			err := storage.StoreTopic(ctx, tt.topic)

			// Assert error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("StoreTopic() expected error but got none")
					return
				}
				if tt.wantErrContains != "" && !contains(err.Error(), tt.wantErrContains) {
					t.Errorf("StoreTopic() error = %v, want error containing %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Errorf("StoreTopic() unexpected error = %v", err)
				return
			}

			// Verify expected call counts
			for operation, expectedCount := range tt.expectedCalls {
				actualCount := mockChromaClient.GetCallCount(operation)
				if actualCount != expectedCount {
					t.Errorf("Expected %d calls to %s, but got %d", expectedCount, operation, actualCount)
				}
			}
		})
	}
}
