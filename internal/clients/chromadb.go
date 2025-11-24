// Package clients provides implementations of external service clients.
//
// This package contains clients for ChromaDB vector storage, LLM services (Ollama),
// DOI resolution services (CrossRef), and other external APIs used by the IDAES system.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChromaDBClient provides a custom client for ChromaDB operations using v2 API
type ChromaDBClient struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	tenant     string
	database   string
}

// NewChromaDBClient creates a new ChromaDB client with v2 API
func NewChromaDBClient(baseURL string, timeout time.Duration, maxRetries int, tenant, database string) *ChromaDBClient {
	// Set defaults for tenant and database if not provided
	if tenant == "" {
		tenant = "default_tenant"
	}
	if database == "" {
		database = "default_database"
	}

	return &ChromaDBClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		tenant:     tenant,
		database:   database,
	}
}

// Health checks if ChromaDB is available using v2 heartbeat endpoint
func (c *ChromaDBClient) Health(ctx context.Context) error {
	var response HeartbeatResponse
	err := c.makeRequest(ctx, "GET", "/api/v2/heartbeat", nil, &response)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// Healthcheck checks if ChromaDB server and executor are ready
func (c *ChromaDBClient) Healthcheck(ctx context.Context) error {
	var response string
	err := c.makeRequest(ctx, "GET", "/api/v2/healthcheck", nil, &response)
	if err != nil {
		return fmt.Errorf("healthcheck failed: %w", err)
	}
	return nil
}

// Version returns the version of the ChromaDB server
func (c *ChromaDBClient) Version(ctx context.Context) (string, error) {
	var version string
	err := c.makeRequest(ctx, "GET", "/api/v2/version", nil, &version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	return version, nil
}

// CreateCollection creates a new collection using v2 API
func (c *ChromaDBClient) CreateCollection(ctx context.Context, name string, metadata map[string]any) (*Collection, error) {
	request := CreateCollectionRequest{
		Name:     name,
		Metadata: metadata,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var collection Collection
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", c.tenant, c.database)
	err = c.makeRequest(ctx, "POST", path, body, &collection)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	return &collection, nil
}

// GetCollection retrieves a collection by ID using v2 API
func (c *ChromaDBClient) GetCollection(ctx context.Context, collectionID string) (*Collection, error) {
	var collection Collection
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s", c.tenant, c.database, collectionID)
	err := c.makeRequest(ctx, "GET", path, nil, &collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	return &collection, nil
}

// ListCollections lists all collections using v2 API
func (c *ChromaDBClient) ListCollections(ctx context.Context) ([]Collection, error) {
	var collections []Collection
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", c.tenant, c.database)
	err := c.makeRequest(ctx, "GET", path, nil, &collections)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	return collections, nil
}

// DeleteCollection deletes a collection using v2 API
func (c *ChromaDBClient) DeleteCollection(ctx context.Context, collectionID string) error {
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s", c.tenant, c.database, collectionID)
	err := c.makeRequest(ctx, "DELETE", path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	return nil
}

// AddDocuments adds documents to a collection using v2 API
func (c *ChromaDBClient) AddDocuments(ctx context.Context, collectionID string, request AddDocumentsRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/add", c.tenant, c.database, collectionID)
	err = c.makeRequest(ctx, "POST", path, body, nil)
	if err != nil {
		return fmt.Errorf("failed to add documents: %w", err)
	}

	return nil
}

// UpdateDocuments updates documents in a collection using v2 API
func (c *ChromaDBClient) UpdateDocuments(ctx context.Context, collectionID string, request UpdateDocumentsRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/update", c.tenant, c.database, collectionID)
	err = c.makeRequest(ctx, "POST", path, body, nil)
	if err != nil {
		return fmt.Errorf("failed to update documents: %w", err)
	}

	return nil
}

// QueryDocuments queries documents in a collection using v2 API
func (c *ChromaDBClient) QueryDocuments(ctx context.Context, collectionID string, request QueryRequest) (*QueryResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var response QueryResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/query", c.tenant, c.database, collectionID)
	err = c.makeRequest(ctx, "POST", path, body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}

	return &response, nil
}

// DeleteDocuments deletes documents from a collection using v2 API
func (c *ChromaDBClient) DeleteDocuments(ctx context.Context, collectionID string, ids []string) error {
	request := DeleteDocumentsRequest{
		IDs: ids,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/delete", c.tenant, c.database, collectionID)
	err = c.makeRequest(ctx, "POST", path, body, nil)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}

// GetDocuments retrieves documents from a collection using v2 API
func (c *ChromaDBClient) GetDocuments(ctx context.Context, collectionID string, ids []string, include []string) (*GetResponse, error) {
	request := GetRequest{
		IDs:     ids,
		Include: include,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var response GetResponse
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/get", c.tenant, c.database, collectionID)
	err = c.makeRequest(ctx, "POST", path, body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}

	return &response, nil
}

// CountDocuments returns the number of documents in a collection using v2 API
func (c *ChromaDBClient) CountDocuments(ctx context.Context, collectionID string) (int, error) {
	var response int
	path := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/count", c.tenant, c.database, collectionID)
	err := c.makeRequest(ctx, "GET", path, nil, &response)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	return response, nil
}

// makeRequest makes an HTTP request with retries and error handling
func (c *ChromaDBClient) makeRequest(ctx context.Context, method, path string, body []byte, response any) error {
	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			waitTime := time.Duration(attempt) * time.Second
			slog.DebugContext(ctx, "Retrying ChromaDB request", "attempt", attempt, "wait", waitTime, "url", url)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			slog.DebugContext(ctx, "ChromaDB request failed", "attempt", attempt, "error", err)
			continue
		}
		if resp == nil {
			lastErr = fmt.Errorf("received nil response")
			slog.DebugContext(ctx, "ChromaDB request returned nil response", "attempt", attempt)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
			slog.DebugContext(ctx, "ChromaDB request returned error status", "status", resp.StatusCode, "response", string(bodyBytes))
			continue
		}

		if response != nil {
			if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
				lastErr = err
				slog.DebugContext(ctx, "Failed to decode ChromaDB response", "error", err)
				continue
			}
		}

		return nil
	}

	return fmt.Errorf("ChromaDB request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// Collection represents a ChromaDB collection in v2 API
type Collection struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	ConfigurationJSON CollectionConfig `json:"configuration_json"`
	Tenant            string           `json:"tenant"`
	Database          string           `json:"database"`
	LogPosition       int64            `json:"log_position"`
	Version           int32            `json:"version"`
	Metadata          map[string]any   `json:"metadata,omitempty"`
	Dimension         *int32           `json:"dimension,omitempty"`
}

// CollectionConfig represents collection configuration
type CollectionConfig struct {
	EmbeddingFunction *EmbeddingFunctionConfig `json:"embedding_function,omitempty"`
	HNSW              *HNSWConfig              `json:"hnsw,omitempty"`
	SPANN             *SPANNConfig             `json:"spann,omitempty"`
}

// EmbeddingFunctionConfig represents embedding function configuration
type EmbeddingFunctionConfig struct {
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

// HNSWConfig represents HNSW algorithm configuration
type HNSWConfig struct {
	EfConstruction *int     `json:"ef_construction,omitempty"`
	EfSearch       *int     `json:"ef_search,omitempty"`
	MaxNeighbors   *int     `json:"max_neighbors,omitempty"`
	ResizeFactor   *float64 `json:"resize_factor,omitempty"`
	Space          *string  `json:"space,omitempty"`
	SyncThreshold  *int     `json:"sync_threshold,omitempty"`
}

// SPANNConfig represents SPANN algorithm configuration
type SPANNConfig struct {
	EfConstruction        *int    `json:"ef_construction,omitempty"`
	EfSearch              *int    `json:"ef_search,omitempty"`
	MaxNeighbors          *int    `json:"max_neighbors,omitempty"`
	MergeThreshold        *int32  `json:"merge_threshold,omitempty"`
	ReassignNeighborCount *int32  `json:"reassign_neighbor_count,omitempty"`
	SearchNprobe          *int32  `json:"search_nprobe,omitempty"`
	Space                 *string `json:"space,omitempty"`
	SplitThreshold        *int32  `json:"split_threshold,omitempty"`
	WriteNprobe           *int32  `json:"write_nprobe,omitempty"`
}

// HeartbeatResponse represents the v2 heartbeat response
type HeartbeatResponse struct {
	NanosecondHeartbeat uint64 `json:"nanosecond heartbeat"`
}

// ErrorResponse represents API error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CreateCollectionRequest represents a request to create a collection in v2 API
type CreateCollectionRequest struct {
	Name          string            `json:"name"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
	Configuration *CollectionConfig `json:"configuration,omitempty"`
	GetOrCreate   bool              `json:"get_or_create,omitempty"`
}

// AddDocumentsRequest represents a request to add documents in v2 API
type AddDocumentsRequest struct {
	IDs        []string         `json:"ids"`
	Embeddings [][]float64      `json:"embeddings"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
	URIs       []string         `json:"uris,omitempty"`
}

// UpdateDocumentsRequest represents a request to update documents in v2 API
type UpdateDocumentsRequest struct {
	IDs        []string         `json:"ids"`
	Embeddings [][]float64      `json:"embeddings,omitempty"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
	URIs       []string         `json:"uris,omitempty"`
}

// QueryRequest represents a query request in v2 API
type QueryRequest struct {
	QueryEmbeddings [][]float64    `json:"query_embeddings"`
	NResults        int            `json:"n_results,omitempty"`
	Where           map[string]any `json:"where,omitempty"`
	WhereDocument   map[string]any `json:"where_document,omitempty"`
	Include         []string       `json:"include,omitempty"`
	IDs             []string       `json:"ids,omitempty"`
}

// QueryResponse represents a query response in v2 API
type QueryResponse struct {
	IDs        [][]string         `json:"ids"`
	Distances  [][]float64        `json:"distances,omitempty"`
	Embeddings [][][]float64      `json:"embeddings,omitempty"`
	Documents  [][]string         `json:"documents,omitempty"`
	Metadatas  [][]map[string]any `json:"metadatas,omitempty"`
	URIs       [][]string         `json:"uris,omitempty"`
	Include    []string           `json:"include"`
}

// DeleteDocumentsRequest represents a request to delete documents in v2 API
type DeleteDocumentsRequest struct {
	IDs           []string       `json:"ids,omitempty"`
	Where         map[string]any `json:"where,omitempty"`
	WhereDocument map[string]any `json:"where_document,omitempty"`
}

// GetRequest represents a request to get documents in v2 API
type GetRequest struct {
	IDs           []string       `json:"ids,omitempty"`
	Where         map[string]any `json:"where,omitempty"`
	WhereDocument map[string]any `json:"where_document,omitempty"`
	Include       []string       `json:"include,omitempty"`
	Limit         int            `json:"limit,omitempty"`
	Offset        int            `json:"offset,omitempty"`
}

// GetResponse represents a get response in v2 API
type GetResponse struct {
	IDs        []string         `json:"ids"`
	Embeddings [][]float64      `json:"embeddings,omitempty"`
	Documents  []string         `json:"documents,omitempty"`
	Metadatas  []map[string]any `json:"metadatas,omitempty"`
	URIs       []string         `json:"uris,omitempty"`
	Include    []string         `json:"include"`
}

// Document represents a document in ChromaDB
type Document struct {
	ID        string         `json:"id"`
	Embedding []float64      `json:"embedding,omitempty"`
	Document  string         `json:"document,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Distance  float64        `json:"distance,omitempty"`
}

// NewDocument creates a new document with generated ID
func NewDocument(text string, embedding []float64, metadata map[string]any) *Document {
	return &Document{
		ID:        uuid.New().String(),
		Document:  text,
		Embedding: embedding,
		Metadata:  metadata,
	}
}

// NewDocumentWithID creates a new document with specified ID
func NewDocumentWithID(id, text string, embedding []float64, metadata map[string]any) *Document {
	return &Document{
		ID:        id,
		Document:  text,
		Embedding: embedding,
		Metadata:  metadata,
	}
}
