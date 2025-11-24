package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test data for consistent test scenarios
var (
	testTenant       = "test_tenant"
	testDatabase     = "test_database"
	testTimeout      = 2 * time.Second // Reduced for faster tests
	testRetries      = 1               // Reduced for faster tests
	testMetadata     = map[string]any{"test": "value"}
	testCollectionID = "123e4567-e89b-12d3-a456-426614174000"
)

func TestNewChromaDBClient(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		timeout      time.Duration
		maxRetries   int
		tenant       string
		database     string
		expectTenant string
		expectDB     string
	}{
		{
			name:         "basic_client_creation",
			baseURL:      "http://localhost:8000",
			timeout:      30 * time.Second,
			maxRetries:   3,
			tenant:       "my_tenant",
			database:     "my_db",
			expectTenant: "my_tenant",
			expectDB:     "my_db",
		},
		{
			name:         "empty_tenant_uses_default",
			baseURL:      "http://localhost:8000",
			timeout:      30 * time.Second,
			maxRetries:   3,
			tenant:       "",
			database:     "my_db",
			expectTenant: "default_tenant",
			expectDB:     "my_db",
		},
		{
			name:         "empty_database_uses_default",
			baseURL:      "http://localhost:8000",
			timeout:      30 * time.Second,
			maxRetries:   3,
			tenant:       "my_tenant",
			database:     "",
			expectTenant: "my_tenant",
			expectDB:     "default_database",
		},
		{
			name:         "baseURL_with_trailing_slash",
			baseURL:      "http://localhost:8000/",
			timeout:      30 * time.Second,
			maxRetries:   3,
			tenant:       "my_tenant",
			database:     "my_db",
			expectTenant: "my_tenant",
			expectDB:     "my_db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewChromaDBClient(tt.baseURL, tt.timeout, tt.maxRetries, tt.tenant, tt.database)

			if client == nil {
				t.Fatal("Expected client to be created, got nil")
			}

			if client.tenant != tt.expectTenant {
				t.Errorf("Expected tenant %q, got %q", tt.expectTenant, client.tenant)
			}

			if client.database != tt.expectDB {
				t.Errorf("Expected database %q, got %q", tt.expectDB, client.database)
			}

			if client.maxRetries != tt.maxRetries {
				t.Errorf("Expected maxRetries %d, got %d", tt.maxRetries, client.maxRetries)
			}

			expectedBaseURL := strings.TrimSuffix(tt.baseURL, "/")
			if client.baseURL != expectedBaseURL {
				t.Errorf("Expected baseURL %q, got %q", expectedBaseURL, client.baseURL)
			}

			if client.httpClient.Timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, client.httpClient.Timeout)
			}
		})
	}
}

func TestChromaDBClient_Health(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:           "successful_health_check",
			serverResponse: `{"nanosecond heartbeat": 1635724800000000000}`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:           "server_error_response",
			serverResponse: `{"error": "internal_error", "message": "Database connection failed"}`,
			statusCode:     500,
			expectError:    true,
		},
		{
			name:           "service_unavailable",
			serverResponse: `{"error": "service_unavailable", "message": "Service temporarily unavailable"}`,
			statusCode:     503,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify correct endpoint
				expectedPath := "/api/v2/heartbeat"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				// Verify HTTP method
				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			// Create client with mock server URL
			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.Health(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_Healthcheck(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:           "successful_healthcheck",
			serverResponse: `"ready"`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:           "service_unavailable",
			serverResponse: `{"error": "service_unavailable", "message": "Executor not ready"}`,
			statusCode:     503,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify correct endpoint
				expectedPath := "/api/v2/healthcheck"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.Healthcheck(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_Version(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name            string
		serverResponse  string
		statusCode      int
		expectedVersion string
		expectError     bool
	}{
		{
			name:            "successful_version_check",
			serverResponse:  `"0.4.15"`,
			statusCode:      200,
			expectedVersion: "0.4.15",
			expectError:     false,
		},
		{
			name:            "server_error",
			serverResponse:  `{"error": "internal_error", "message": "Version info unavailable"}`,
			statusCode:      500,
			expectedVersion: "",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/api/v2/version"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			version, err := client.Version(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && version != tt.expectedVersion {
				t.Errorf("Expected version %q, got %q", tt.expectedVersion, version)
			}
		})
	}
}

func TestChromaDBClient_CreateCollection(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionName string
		metadata       map[string]any
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:           "successful_collection_creation",
			collectionName: "test_collection",
			metadata:       testMetadata,
			serverResponse: fmt.Sprintf(`{
				"id": "%s",
				"name": "test_collection",
				"tenant": "%s",
				"database": "%s",
				"metadata": {"test": "value"},
				"log_position": 0,
				"version": 1,
				"configuration_json": {}
			}`, testCollectionID, testTenant, testDatabase),
			statusCode:  200,
			expectError: false,
		},
		{
			name:           "collection_already_exists",
			collectionName: "existing_collection",
			metadata:       testMetadata,
			serverResponse: `{"error": "already_exists", "message": "Collection already exists"}`,
			statusCode:     409,
			expectError:    true,
		},
		{
			name:           "unauthorized_request",
			collectionName: "test_collection",
			metadata:       testMetadata,
			serverResponse: `{"error": "unauthorized", "message": "Authentication required"}`,
			statusCode:     401,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify correct endpoint path
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				// Verify HTTP method
				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Verify Content-Type
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Verify request body
				var reqBody CreateCollectionRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if reqBody.Name != tt.collectionName {
					t.Errorf("Expected collection name %q, got %q", tt.collectionName, reqBody.Name)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			collection, err := client.CreateCollection(ctx, tt.collectionName, tt.metadata)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if collection == nil {
					t.Fatal("Expected collection to be returned, got nil")
				}

				if collection.Name != tt.collectionName {
					t.Errorf("Expected collection name %q, got %q", tt.collectionName, collection.Name)
				}

				if collection.Tenant != testTenant {
					t.Errorf("Expected tenant %q, got %q", testTenant, collection.Tenant)
				}

				if collection.Database != testDatabase {
					t.Errorf("Expected database %q, got %q", testDatabase, collection.Database)
				}
			}
		})
	}
}

func TestChromaDBClient_GetCollection(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:         "successful_get_collection",
			collectionID: testCollectionID,
			serverResponse: fmt.Sprintf(`{
				"id": "%s",
				"name": "test_collection",
				"tenant": "%s",
				"database": "%s",
				"metadata": {"test": "value"},
				"log_position": 0,
				"version": 1,
				"configuration_json": {}
			}`, testCollectionID, testTenant, testDatabase),
			statusCode:  200,
			expectError: false,
		},
		{
			name:           "collection_not_found",
			collectionID:   "nonexistent-id",
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
		{
			name:           "unauthorized_access",
			collectionID:   testCollectionID,
			serverResponse: `{"error": "unauthorized", "message": "Access denied"}`,
			statusCode:     401,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			collection, err := client.GetCollection(ctx, tt.collectionID)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if collection == nil {
					t.Fatal("Expected collection to be returned, got nil")
				}

				if collection.ID != testCollectionID {
					t.Errorf("Expected collection ID %q, got %q", testCollectionID, collection.ID)
				}
			}
		})
	}
}

func TestChromaDBClient_ListCollections(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		expectedCount  int
		expectError    bool
	}{
		{
			name: "successful_list_collections",
			serverResponse: fmt.Sprintf(`[
				{
					"id": "%s",
					"name": "collection1",
					"tenant": "%s",
					"database": "%s",
					"metadata": {},
					"log_position": 0,
					"version": 1,
					"configuration_json": {}
				},
				{
					"id": "456e7890-e89b-12d3-a456-426614174001",
					"name": "collection2",
					"tenant": "%s",
					"database": "%s",
					"metadata": {},
					"log_position": 0,
					"version": 1,
					"configuration_json": {}
				}
			]`, testCollectionID, testTenant, testDatabase, testTenant, testDatabase),
			statusCode:    200,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:           "empty_collection_list",
			serverResponse: `[]`,
			statusCode:     200,
			expectedCount:  0,
			expectError:    false,
		},
		{
			name:           "unauthorized_access",
			serverResponse: `{"error": "unauthorized", "message": "Access denied"}`,
			statusCode:     401,
			expectedCount:  0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections", testTenant, testDatabase)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			collections, err := client.ListCollections(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if len(collections) != tt.expectedCount {
					t.Errorf("Expected %d collections, got %d", tt.expectedCount, len(collections))
				}
			}
		})
	}
}

func TestChromaDBClient_DeleteCollection(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:           "successful_delete_collection",
			collectionID:   testCollectionID,
			serverResponse: `{}`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:           "collection_not_found",
			collectionID:   "nonexistent-id",
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
		{
			name:           "unauthorized_delete",
			collectionID:   testCollectionID,
			serverResponse: `{"error": "unauthorized", "message": "Delete permission denied"}`,
			statusCode:     401,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "DELETE" {
					t.Errorf("Expected DELETE method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.DeleteCollection(ctx, tt.collectionID)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_AddDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		request        AddDocumentsRequest
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:         "successful_add_documents",
			collectionID: testCollectionID,
			request: AddDocumentsRequest{
				IDs:        []string{"doc1", "doc2"},
				Embeddings: [][]float64{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
				Documents:  []string{"Document 1", "Document 2"},
				Metadatas:  []map[string]any{{"type": "text"}, {"type": "text"}},
			},
			serverResponse: `{}`,
			statusCode:     201,
			expectError:    false,
		},
		{
			name:         "invalid_data",
			collectionID: testCollectionID,
			request: AddDocumentsRequest{
				IDs:        []string{"doc1"},
				Embeddings: [][]float64{{0.1, 0.2}},
				Documents:  []string{},
			},
			serverResponse: `{"error": "invalid_data", "message": "Mismatched array lengths"}`,
			statusCode:     400,
			expectError:    true,
		},
		{
			name:         "collection_not_found",
			collectionID: "nonexistent-id",
			request: AddDocumentsRequest{
				IDs:        []string{"doc1"},
				Embeddings: [][]float64{{0.1, 0.2, 0.3}},
				Documents:  []string{"Document 1"},
			},
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/add", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Verify request body
				var reqBody AddDocumentsRequest
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if len(reqBody.IDs) != len(tt.request.IDs) {
					t.Errorf("Expected %d IDs, got %d", len(tt.request.IDs), len(reqBody.IDs))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.AddDocuments(ctx, tt.collectionID, tt.request)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_QueryDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		request        QueryRequest
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:         "successful_query",
			collectionID: testCollectionID,
			request: QueryRequest{
				QueryEmbeddings: [][]float64{{0.1, 0.2, 0.3}},
				NResults:        5,
				Include:         []string{"documents", "metadatas", "distances"},
			},
			serverResponse: `{
				"ids": [["doc1", "doc2"]],
				"documents": [["Document 1", "Document 2"]],
				"metadatas": [[{"type": "text"}, {"type": "text"}]],
				"distances": [[0.1, 0.2]],
				"include": ["documents", "metadatas", "distances"]
			}`,
			statusCode:  200,
			expectError: false,
		},
		{
			name:         "empty_query_results",
			collectionID: testCollectionID,
			request: QueryRequest{
				QueryEmbeddings: [][]float64{{0.9, 0.9, 0.9}},
				NResults:        5,
			},
			serverResponse: `{
				"ids": [[]],
				"include": []
			}`,
			statusCode:  200,
			expectError: false,
		},
		{
			name:         "collection_not_found",
			collectionID: "nonexistent-id",
			request: QueryRequest{
				QueryEmbeddings: [][]float64{{0.1, 0.2, 0.3}},
				NResults:        5,
			},
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/query", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			response, err := client.QueryDocuments(ctx, tt.collectionID, tt.request)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && response == nil {
				t.Error("Expected response to be returned, got nil")
			}
		})
	}
}

func TestChromaDBClient_CountDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		serverResponse string
		statusCode     int
		expectedCount  int
		expectError    bool
	}{
		{
			name:           "successful_count",
			collectionID:   testCollectionID,
			serverResponse: `42`,
			statusCode:     200,
			expectedCount:  42,
			expectError:    false,
		},
		{
			name:           "zero_count",
			collectionID:   testCollectionID,
			serverResponse: `0`,
			statusCode:     200,
			expectedCount:  0,
			expectError:    false,
		},
		{
			name:           "collection_not_found",
			collectionID:   "nonexistent-id",
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectedCount:  0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/count", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			count, err := client.CountDocuments(ctx, tt.collectionID)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && count != tt.expectedCount {
				t.Errorf("Expected count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

func TestChromaDBClient_UpdateDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		request        UpdateDocumentsRequest
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:         "successful_update_documents",
			collectionID: testCollectionID,
			request: UpdateDocumentsRequest{
				IDs:        []string{"doc1", "doc2"},
				Embeddings: [][]float64{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
				Documents:  []string{"Updated Document 1", "Updated Document 2"},
				Metadatas:  []map[string]any{{"type": "updated"}, {"type": "updated"}},
			},
			serverResponse: `{}`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:         "document_not_found",
			collectionID: testCollectionID,
			request: UpdateDocumentsRequest{
				IDs:        []string{"nonexistent-doc"},
				Embeddings: [][]float64{{0.1, 0.2, 0.3}},
				Documents:  []string{"Updated Document"},
			},
			serverResponse: `{"error": "not_found", "message": "Document not found"}`,
			statusCode:     404,
			expectError:    true,
		},
		{
			name:         "collection_not_found",
			collectionID: "nonexistent-id",
			request: UpdateDocumentsRequest{
				IDs:        []string{"doc1"},
				Embeddings: [][]float64{{0.1, 0.2, 0.3}},
				Documents:  []string{"Updated Document"},
			},
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/update", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.UpdateDocuments(ctx, tt.collectionID, tt.request)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_DeleteDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		ids            []string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:           "successful_delete_documents",
			collectionID:   testCollectionID,
			ids:            []string{"doc1", "doc2"},
			serverResponse: `{}`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:           "no_documents_to_delete",
			collectionID:   testCollectionID,
			ids:            []string{"nonexistent-doc"},
			serverResponse: `{}`,
			statusCode:     200,
			expectError:    false,
		},
		{
			name:           "collection_not_found",
			collectionID:   "nonexistent-id",
			ids:            []string{"doc1"},
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/delete", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			err := client.DeleteDocuments(ctx, tt.collectionID, tt.ids)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChromaDBClient_GetDocuments(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		collectionID   string
		ids            []string
		include        []string
		serverResponse string
		statusCode     int
		expectError    bool
	}{
		{
			name:         "successful_get_documents",
			collectionID: testCollectionID,
			ids:          []string{"doc1", "doc2"},
			include:      []string{"documents", "metadatas"},
			serverResponse: `{
				"ids": ["doc1", "doc2"],
				"documents": ["Document 1", "Document 2"],
				"metadatas": [{"type": "text"}, {"type": "text"}],
				"include": ["documents", "metadatas"]
			}`,
			statusCode:  200,
			expectError: false,
		},
		{
			name:         "get_documents_basic",
			collectionID: testCollectionID,
			ids:          []string{"doc1"},
			include:      []string{"documents"},
			serverResponse: `{
				"ids": ["doc1"],
				"documents": ["Document 1"],
				"include": ["documents"]
			}`,
			statusCode:  200,
			expectError: false,
		},
		{
			name:         "no_documents_found",
			collectionID: testCollectionID,
			ids:          []string{"nonexistent-doc"},
			include:      []string{},
			serverResponse: `{
				"ids": [],
				"include": []
			}`,
			statusCode:  200,
			expectError: false,
		},
		{
			name:           "collection_not_found",
			collectionID:   "nonexistent-id",
			ids:            []string{"doc1"},
			include:        []string{"documents"},
			serverResponse: `{"error": "not_found", "message": "Collection not found"}`,
			statusCode:     404,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/api/v2/tenants/%s/databases/%s/collections/%s/get", testTenant, testDatabase, tt.collectionID)
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewChromaDBClient(server.URL, testTimeout, testRetries, testTenant, testDatabase)

			response, err := client.GetDocuments(ctx, tt.collectionID, tt.ids, tt.include)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && response == nil {
				t.Error("Expected response to be returned, got nil")
			}
		})
	}
}

func TestNewDocument(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		embedding []float64
		metadata  map[string]any
	}{
		{
			name:      "basic_document_creation",
			content:   "Test document content",
			embedding: []float64{0.1, 0.2, 0.3},
			metadata:  map[string]any{"type": "test"},
		},
		{
			name:      "document_with_empty_metadata",
			content:   "Another test document",
			embedding: []float64{0.4, 0.5, 0.6},
			metadata:  nil,
		},
		{
			name:      "document_with_nil_embedding",
			content:   "Document without embedding",
			embedding: nil,
			metadata:  map[string]any{"category": "unembedded"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument(tt.content, tt.embedding, tt.metadata)

			if doc == nil {
				t.Fatal("Expected document to be created, got nil")
			}

			if doc.ID == "" {
				t.Error("Expected document ID to be generated")
			}

			if doc.Document != tt.content {
				t.Errorf("Expected content %q, got %q", tt.content, doc.Document)
			}

			if len(doc.Embedding) != len(tt.embedding) {
				t.Errorf("Expected embedding length %d, got %d", len(tt.embedding), len(doc.Embedding))
			}

			if tt.metadata != nil && doc.Metadata == nil {
				t.Error("Expected metadata to be set")
			}
		})
	}
}

func TestNewDocumentWithID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		content   string
		embedding []float64
		metadata  map[string]any
	}{
		{
			name:      "document_with_custom_id",
			id:        "custom-doc-id",
			content:   "Test document with custom ID",
			embedding: []float64{0.1, 0.2, 0.3},
			metadata:  map[string]any{"type": "custom"},
		},
		{
			name:      "document_with_empty_id",
			id:        "",
			content:   "Document with empty ID",
			embedding: []float64{0.4, 0.5, 0.6},
			metadata:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocumentWithID(tt.id, tt.content, tt.embedding, tt.metadata)

			if doc == nil {
				t.Fatal("Expected document to be created, got nil")
			}

			if doc.ID != tt.id {
				t.Errorf("Expected document ID %q, got %q", tt.id, doc.ID)
			}

			if doc.Document != tt.content {
				t.Errorf("Expected content %q, got %q", tt.content, doc.Document)
			}

			if len(doc.Embedding) != len(tt.embedding) {
				t.Errorf("Expected embedding length %d, got %d", len(tt.embedding), len(doc.Embedding))
			}
		})
	}
}
