// Package interfaces defines core interfaces for external service clients.
package interfaces

import (
	"context"
	"time"
)

// ChromaDB Type definitions to avoid import cycles

// Collection represents a ChromaDB collection
type Collection struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Document represents a document in ChromaDB
type Document struct {
	ID        string         `json:"id"`
	Document  string         `json:"document,omitempty"`
	Embedding []float64      `json:"embedding,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Distance  float64        `json:"distance,omitempty"`
}

// AddDocumentsRequest represents a request to add documents
type AddDocumentsRequest struct {
	IDs        []string         `json:"ids"`
	Embeddings [][]float64      `json:"embeddings"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
}

// GetDocumentsResponse represents a get documents response
type GetDocumentsResponse struct {
	IDs        []string         `json:"ids"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
	Embeddings [][]float64      `json:"embeddings,omitempty"`
}

// QueryRequest represents a query request
type QueryRequest struct {
	QueryEmbeddings [][]float64    `json:"query_embeddings"`
	NResults        int            `json:"n_results,omitempty"`
	Where           map[string]any `json:"where,omitempty"`
	Include         []string       `json:"include,omitempty"`
}

// QueryResponse represents a query response
type QueryResponse struct {
	IDs       [][]string         `json:"ids"`
	Distances [][]float64        `json:"distances,omitempty"`
	Documents [][]string         `json:"documents,omitempty"`
	Metadatas [][]map[string]any `json:"metadatas,omitempty"`
}

// ListCollectionsResponse represents a list collections response
type ListCollectionsResponse struct {
	Collections []*Collection `json:"collections"`
}

// ChromaDBError represents a ChromaDB API error
type ChromaDBError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *ChromaDBError) Error() string {
	return e.Message
}

// ChromaDBClient defines the interface for ChromaDB operations
type ChromaDBClient interface {
	// Health and status operations
	Health(ctx context.Context) error
	Healthcheck(ctx context.Context) error
	Version(ctx context.Context) (string, error)

	// Collection operations
	CreateCollection(ctx context.Context, name string, metadata map[string]any) (*Collection, error)
	GetCollection(ctx context.Context, name string) (*Collection, error)
	ListCollections(ctx context.Context) (*ListCollectionsResponse, error)
	DeleteCollection(ctx context.Context, name string) error

	// Document operations
	AddDocuments(ctx context.Context, collectionID string, request AddDocumentsRequest) error
	GetDocuments(ctx context.Context, collectionID string, ids []string, include []string) (*GetDocumentsResponse, error)
	QueryDocuments(ctx context.Context, collectionID string, request QueryRequest) (*QueryResponse, error)
	DeleteDocuments(ctx context.Context, collectionID string, ids []string) error
	CountDocuments(ctx context.Context, collectionID string) (int, error)

	// Utility
	Close() error
}

// MockChromaDBClient provides a test double for ChromaDB operations
type MockChromaDBClient struct {
	// Function fields for customizing behavior
	HealthFunc           func(ctx context.Context) error
	HealthcheckFunc      func(ctx context.Context) error
	VersionFunc          func(ctx context.Context) (string, error)
	CreateCollectionFunc func(ctx context.Context, name string, metadata map[string]any) (*Collection, error)
	GetCollectionFunc    func(ctx context.Context, name string) (*Collection, error)
	ListCollectionsFunc  func(ctx context.Context) (*ListCollectionsResponse, error)
	DeleteCollectionFunc func(ctx context.Context, name string) error
	AddDocumentsFunc     func(ctx context.Context, collectionID string, request AddDocumentsRequest) error
	GetDocumentsFunc     func(ctx context.Context, collectionID string, ids []string, include []string) (*GetDocumentsResponse, error)
	QueryDocumentsFunc   func(ctx context.Context, collectionID string, request QueryRequest) (*QueryResponse, error)
	DeleteDocumentsFunc  func(ctx context.Context, collectionID string, ids []string) error
	CountDocumentsFunc   func(ctx context.Context, collectionID string) (int, error)
	CloseFunc            func() error

	// State tracking for test assertions
	Collections     map[string]*Collection
	Documents       map[string]map[string]*Document // collection -> document_id -> document
	CallCounts      map[string]int
	LastCalls       map[string][]any
	SimulateLatency time.Duration
	ForceErrors     map[string]error // operation_name -> error
}

// NewMockChromaDBClient creates a new mock ChromaDB client with default behaviors
func NewMockChromaDBClient() *MockChromaDBClient {
	return &MockChromaDBClient{
		Collections: make(map[string]*Collection),
		Documents:   make(map[string]map[string]*Document),
		CallCounts:  make(map[string]int),
		LastCalls:   make(map[string][]any),
		ForceErrors: make(map[string]error),
	}
}

// Health implements ChromaDBClient interface
func (m *MockChromaDBClient) Health(ctx context.Context) error {
	m.CallCounts["Health"]++
	m.LastCalls["Health"] = []any{ctx}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["Health"]; exists {
		return err
	}

	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

// Healthcheck implements ChromaDBClient interface
func (m *MockChromaDBClient) Healthcheck(ctx context.Context) error {
	m.CallCounts["Healthcheck"]++
	m.LastCalls["Healthcheck"] = []any{ctx}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["Healthcheck"]; exists {
		return err
	}

	if m.HealthcheckFunc != nil {
		return m.HealthcheckFunc(ctx)
	}
	return nil
}

// Version implements ChromaDBClient interface
func (m *MockChromaDBClient) Version(ctx context.Context) (string, error) {
	m.CallCounts["Version"]++
	m.LastCalls["Version"] = []any{ctx}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["Version"]; exists {
		return "", err
	}

	if m.VersionFunc != nil {
		return m.VersionFunc(ctx)
	}
	return "mock-version-1.0.0", nil
}

// CreateCollection implements ChromaDBClient interface
func (m *MockChromaDBClient) CreateCollection(ctx context.Context, name string, metadata map[string]any) (*Collection, error) {
	m.CallCounts["CreateCollection"]++
	m.LastCalls["CreateCollection"] = []any{ctx, name, metadata}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["CreateCollection"]; exists {
		return nil, err
	}

	if m.CreateCollectionFunc != nil {
		return m.CreateCollectionFunc(ctx, name, metadata)
	}

	// Default behavior: create and store collection
	collection := &Collection{
		ID:       "test-id-" + name,
		Name:     name,
		Metadata: metadata,
	}
	m.Collections[name] = collection
	m.Documents[collection.ID] = make(map[string]*Document)

	return collection, nil
}

// GetCollection implements ChromaDBClient interface
func (m *MockChromaDBClient) GetCollection(ctx context.Context, name string) (*Collection, error) {
	m.CallCounts["GetCollection"]++
	m.LastCalls["GetCollection"] = []any{ctx, name}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["GetCollection"]; exists {
		return nil, err
	}

	if m.GetCollectionFunc != nil {
		return m.GetCollectionFunc(ctx, name)
	}

	// Default behavior: return collection if exists
	if collection, exists := m.Collections[name]; exists {
		return collection, nil
	}

	return nil, &ChromaDBError{
		StatusCode: 404,
		Message:    "Collection not found: " + name,
	}
}

// ListCollections implements ChromaDBClient interface
func (m *MockChromaDBClient) ListCollections(ctx context.Context) (*ListCollectionsResponse, error) {
	m.CallCounts["ListCollections"]++
	m.LastCalls["ListCollections"] = []any{ctx}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["ListCollections"]; exists {
		return nil, err
	}

	if m.ListCollectionsFunc != nil {
		return m.ListCollectionsFunc(ctx)
	}

	// Default behavior: return all collections
	collections := make([]*Collection, 0, len(m.Collections))
	for _, collection := range m.Collections {
		collections = append(collections, collection)
	}

	return &ListCollectionsResponse{
		Collections: collections,
	}, nil
}

// DeleteCollection implements ChromaDBClient interface
func (m *MockChromaDBClient) DeleteCollection(ctx context.Context, name string) error {
	m.CallCounts["DeleteCollection"]++
	m.LastCalls["DeleteCollection"] = []any{ctx, name}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["DeleteCollection"]; exists {
		return err
	}

	if m.DeleteCollectionFunc != nil {
		return m.DeleteCollectionFunc(ctx, name)
	}

	// Default behavior: delete collection if exists
	if collection, exists := m.Collections[name]; exists {
		delete(m.Collections, name)
		delete(m.Documents, collection.ID)
		return nil
	}

	return &ChromaDBError{
		StatusCode: 404,
		Message:    "Collection not found: " + name,
	}
}

// AddDocuments implements ChromaDBClient interface
func (m *MockChromaDBClient) AddDocuments(ctx context.Context, collectionID string, request AddDocumentsRequest) error {
	m.CallCounts["AddDocuments"]++
	m.LastCalls["AddDocuments"] = []any{ctx, collectionID, request}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["AddDocuments"]; exists {
		return err
	}

	if m.AddDocumentsFunc != nil {
		return m.AddDocumentsFunc(ctx, collectionID, request)
	}

	// Default behavior: store documents
	if documents, exists := m.Documents[collectionID]; exists {
		for i, id := range request.IDs {
			var embedding []float64
			var content string
			var metadata map[string]any

			if i < len(request.Embeddings) {
				embedding = request.Embeddings[i]
			}
			if i < len(request.Documents) {
				content = request.Documents[i]
			}
			if i < len(request.Metadatas) {
				metadata = request.Metadatas[i]
			}

			documents[id] = &Document{
				ID:        id,
				Document:  content,
				Embedding: embedding,
				Metadata:  metadata,
			}
		}
		return nil
	}

	return &ChromaDBError{
		StatusCode: 404,
		Message:    "Collection not found: " + collectionID,
	}
}

// GetDocuments implements ChromaDBClient interface
func (m *MockChromaDBClient) GetDocuments(ctx context.Context, collectionID string, ids []string, include []string) (*GetDocumentsResponse, error) {
	m.CallCounts["GetDocuments"]++
	m.LastCalls["GetDocuments"] = []any{ctx, collectionID, ids, include}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["GetDocuments"]; exists {
		return nil, err
	}

	if m.GetDocumentsFunc != nil {
		return m.GetDocumentsFunc(ctx, collectionID, ids, include)
	}

	// Default behavior: return requested documents
	documents, exists := m.Documents[collectionID]
	if !exists {
		return nil, &ChromaDBError{
			StatusCode: 404,
			Message:    "Collection not found: " + collectionID,
		}
	}

	response := &GetDocumentsResponse{
		IDs:        []string{},
		Documents:  []string{},
		Metadatas:  []map[string]any{},
		Embeddings: [][]float64{},
	}

	for _, id := range ids {
		if doc, exists := documents[id]; exists {
			response.IDs = append(response.IDs, doc.ID)
			response.Documents = append(response.Documents, doc.Document)
			response.Metadatas = append(response.Metadatas, doc.Metadata)
			response.Embeddings = append(response.Embeddings, doc.Embedding)
		}
	}

	return response, nil
}

// QueryDocuments implements ChromaDBClient interface
func (m *MockChromaDBClient) QueryDocuments(ctx context.Context, collectionID string, request QueryRequest) (*QueryResponse, error) {
	m.CallCounts["QueryDocuments"]++
	m.LastCalls["QueryDocuments"] = []any{ctx, collectionID, request}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["QueryDocuments"]; exists {
		return nil, err
	}

	if m.QueryDocumentsFunc != nil {
		return m.QueryDocumentsFunc(ctx, collectionID, request)
	}

	// Default behavior: return mock query results
	documents, exists := m.Documents[collectionID]
	if !exists {
		return nil, &ChromaDBError{
			StatusCode: 404,
			Message:    "Collection not found: " + collectionID,
		}
	}

	// Simple mock implementation - return first N documents
	limit := request.NResults
	if limit <= 0 {
		limit = 10
	}

	response := &QueryResponse{
		IDs:       [][]string{},
		Documents: [][]string{},
		Metadatas: [][]map[string]any{},
		Distances: [][]float64{},
	}

	count := 0
	for _, doc := range documents {
		if count >= limit {
			break
		}

		// Mock: single query result set
		if len(response.IDs) == 0 {
			response.IDs = append(response.IDs, []string{})
			response.Documents = append(response.Documents, []string{})
			response.Metadatas = append(response.Metadatas, []map[string]any{})
			response.Distances = append(response.Distances, []float64{})
		}

		response.IDs[0] = append(response.IDs[0], doc.ID)
		response.Documents[0] = append(response.Documents[0], doc.Document)
		response.Metadatas[0] = append(response.Metadatas[0], doc.Metadata)
		response.Distances[0] = append(response.Distances[0], 0.1) // Mock distance

		count++
	}

	return response, nil
}

// DeleteDocuments implements ChromaDBClient interface
func (m *MockChromaDBClient) DeleteDocuments(ctx context.Context, collectionID string, ids []string) error {
	m.CallCounts["DeleteDocuments"]++
	m.LastCalls["DeleteDocuments"] = []any{ctx, collectionID, ids}

	if m.SimulateLatency > 0 {
		time.Sleep(m.SimulateLatency)
	}

	if err, exists := m.ForceErrors["DeleteDocuments"]; exists {
		return err
	}

	if m.DeleteDocumentsFunc != nil {
		return m.DeleteDocumentsFunc(ctx, collectionID, ids)
	}

	// Default behavior: delete documents
	documents, exists := m.Documents[collectionID]
	if !exists {
		return &ChromaDBError{
			StatusCode: 404,
			Message:    "Collection not found: " + collectionID,
		}
	}

	for _, id := range ids {
		delete(documents, id)
	}

	return nil
}

// CountDocuments implements ChromaDBClient interface
func (m *MockChromaDBClient) CountDocuments(ctx context.Context, collectionID string) (int, error) {
	m.CallCounts["CountDocuments"]++

	if err, exists := m.ForceErrors["CountDocuments"]; exists {
		return 0, err
	}

	if m.CountDocumentsFunc != nil {
		return m.CountDocumentsFunc(ctx, collectionID)
	}

	// Default behavior: count documents
	documents, exists := m.Documents[collectionID]
	if !exists {
		return 0, &ChromaDBError{
			StatusCode: 404,
			Message:    "Collection not found: " + collectionID,
		}
	}

	return len(documents), nil
}

// Close implements ChromaDBClient interface
func (m *MockChromaDBClient) Close() error {
	m.CallCounts["Close"]++

	if err, exists := m.ForceErrors["Close"]; exists {
		return err
	}

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	return nil
}

// Helper methods for test setup

// WithHealthError configures the mock to return an error on Health calls
func (m *MockChromaDBClient) WithHealthError(err error) *MockChromaDBClient {
	m.ForceErrors["Health"] = err
	return m
}

// WithLatency configures the mock to simulate network latency
func (m *MockChromaDBClient) WithLatency(duration time.Duration) *MockChromaDBClient {
	m.SimulateLatency = duration
	return m
}

// WithCollections pre-populates the mock with collections
func (m *MockChromaDBClient) WithCollections(collections map[string]*Collection) *MockChromaDBClient {
	for name, collection := range collections {
		m.Collections[name] = collection
		if m.Documents[collection.ID] == nil {
			m.Documents[collection.ID] = make(map[string]*Document)
		}
	}
	return m
}

// GetCallCount returns the number of times a method was called
func (m *MockChromaDBClient) GetCallCount(method string) int {
	return m.CallCounts[method]
}

// GetLastCall returns the arguments of the last call to a method
func (m *MockChromaDBClient) GetLastCall(method string) []any {
	return m.LastCalls[method]
}

// Reset clears all state and call tracking
func (m *MockChromaDBClient) Reset() {
	m.Collections = make(map[string]*Collection)
	m.Documents = make(map[string]map[string]*Document)
	m.CallCounts = make(map[string]int)
	m.LastCalls = make(map[string][]any)
	m.ForceErrors = make(map[string]error)
	m.SimulateLatency = 0
}
