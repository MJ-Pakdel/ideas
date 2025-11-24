package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
)

// Test constants for consistent test scenarios
var (
	testChatModel      = "llama3.2"
	testEmbeddingModel = "nomic-embed-text"
	testLLMTimeout     = 2 * time.Second // Reduced for faster tests
	testLLMRetries     = 1               // Reduced for faster tests
	testPrompt         = "Test prompt for completion"
	testEmbedText      = "Test text for embedding generation"
	testTemperature    = 0.8
	testMaxTokens      = 100
)

func TestNewOllamaClient(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		chatModel      string
		embeddingModel string
		timeout        time.Duration
		maxRetries     int
		expectedURL    string
	}{
		{
			name:           "basic_client_creation",
			baseURL:        "http://localhost:11434",
			chatModel:      testChatModel,
			embeddingModel: testEmbeddingModel,
			timeout:        testLLMTimeout,
			maxRetries:     testLLMRetries,
			expectedURL:    "http://localhost:11434",
		},
		{
			name:           "baseURL_with_trailing_slash",
			baseURL:        "http://localhost:11434/",
			chatModel:      testChatModel,
			embeddingModel: testEmbeddingModel,
			timeout:        testLLMTimeout,
			maxRetries:     testLLMRetries,
			expectedURL:    "http://localhost:11434",
		},
		{
			name:           "custom_models_and_settings",
			baseURL:        "https://ollama.example.com",
			chatModel:      "custom-chat-model",
			embeddingModel: "custom-embedding-model",
			timeout:        10 * time.Second,
			maxRetries:     5,
			expectedURL:    "https://ollama.example.com",
		},
		{
			name:           "zero_retries",
			baseURL:        "http://localhost:11434",
			chatModel:      testChatModel,
			embeddingModel: testEmbeddingModel,
			timeout:        testLLMTimeout,
			maxRetries:     0,
			expectedURL:    "http://localhost:11434",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOllamaClient(tt.baseURL, tt.chatModel, tt.embeddingModel, tt.timeout, tt.maxRetries)

			if client == nil {
				t.Fatal("Expected client to be created, got nil")
			}

			if client.baseURL != tt.expectedURL {
				t.Errorf("Expected baseURL %q, got %q", tt.expectedURL, client.baseURL)
			}

			if client.chatModel != tt.chatModel {
				t.Errorf("Expected chatModel %q, got %q", tt.chatModel, client.chatModel)
			}

			if client.embeddingModel != tt.embeddingModel {
				t.Errorf("Expected embeddingModel %q, got %q", tt.embeddingModel, client.embeddingModel)
			}

			if client.maxRetries != tt.maxRetries {
				t.Errorf("Expected maxRetries %d, got %d", tt.maxRetries, client.maxRetries)
			}

			if client.httpClient == nil {
				t.Error("Expected httpClient to be initialized")
			}

			if client.httpClient.Timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, client.httpClient.Timeout)
			}
		})
	}
}

func TestOllamaClient_Complete(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name            string
		prompt          string
		options         *interfaces.LLMOptions
		serverResponse  string
		statusCode      int
		expectError     bool
		expectedPath    string
		validateRequest bool
	}{
		{
			name:    "successful_completion_basic",
			prompt:  testPrompt,
			options: nil,
			serverResponse: `{
				"model": "llama3.2",
				"created_at": "2023-12-07T09:30:00Z",
				"response": "This is a test response from the LLM.",
				"done": true,
				"total_duration": 5000000000,
				"load_duration": 400000000,
				"prompt_eval_count": 50,
				"prompt_eval_duration": 3600000000,
				"eval_count": 25,
				"eval_duration": 1000000000
			}`,
			statusCode:      200,
			expectError:     false,
			expectedPath:    "/api/generate",
			validateRequest: true,
		},
		{
			name:   "completion_with_temperature",
			prompt: testPrompt,
			options: &interfaces.LLMOptions{
				Temperature: testTemperature,
			},
			serverResponse: `{
				"model": "llama3.2",
				"response": "Temperature-controlled response.",
				"done": true
			}`,
			statusCode:      200,
			expectError:     false,
			expectedPath:    "/api/generate",
			validateRequest: true,
		},
		{
			name:   "completion_with_max_tokens",
			prompt: testPrompt,
			options: &interfaces.LLMOptions{
				MaxTokens: testMaxTokens,
			},
			serverResponse: `{
				"model": "llama3.2",
				"response": "Token-limited response.",
				"done": true
			}`,
			statusCode:      200,
			expectError:     false,
			expectedPath:    "/api/generate",
			validateRequest: true,
		},
		{
			name:   "completion_with_all_options",
			prompt: testPrompt,
			options: &interfaces.LLMOptions{
				Temperature: testTemperature,
				MaxTokens:   testMaxTokens,
				TopP:        0.9,
				TopK:        50,
			},
			serverResponse: `{
				"model": "llama3.2",
				"response": "Fully configured response.",
				"done": true
			}`,
			statusCode:      200,
			expectError:     false,
			expectedPath:    "/api/generate",
			validateRequest: true,
		},
		{
			name:            "server_error",
			prompt:          testPrompt,
			options:         nil,
			serverResponse:  `{"error": "model not found"}`,
			statusCode:      404,
			expectError:     true,
			expectedPath:    "/api/generate",
			validateRequest: false,
		},
		{
			name:            "invalid_json_response",
			prompt:          testPrompt,
			options:         nil,
			serverResponse:  `invalid json response`,
			statusCode:      200,
			expectError:     true,
			expectedPath:    "/api/generate",
			validateRequest: false,
		},
		{
			name:    "empty_prompt",
			prompt:  "",
			options: nil,
			serverResponse: `{
				"model": "llama3.2",
				"response": "Empty prompt response.",
				"done": true
			}`,
			statusCode:      200,
			expectError:     false,
			expectedPath:    "/api/generate",
			validateRequest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.expectedPath {
					t.Errorf("Expected path %q, got %q", tt.expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				if tt.validateRequest {
					// Validate request body
					var reqBody map[string]any
					if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
						t.Errorf("Failed to decode request body: %v", err)
					}

					if model, ok := reqBody["model"].(string); !ok || model != testChatModel {
						t.Errorf("Expected model %q, got %v", testChatModel, reqBody["model"])
					}

					if prompt, ok := reqBody["prompt"].(string); !ok || prompt != tt.prompt {
						t.Errorf("Expected prompt %q, got %v", tt.prompt, reqBody["prompt"])
					}

					if stream, ok := reqBody["stream"].(bool); !ok || stream != false {
						t.Error("Expected stream to be false")
					}

					// Validate options if provided
					if tt.options != nil {
						if options, ok := reqBody["options"].(map[string]any); ok {
							if tt.options.Temperature > 0 {
								if temp, exists := options["temperature"]; !exists || temp != tt.options.Temperature {
									t.Errorf("Expected temperature %v, got %v", tt.options.Temperature, temp)
								}
							}
							if tt.options.MaxTokens > 0 {
								if maxTokens, exists := options["num_predict"]; !exists || maxTokens != float64(tt.options.MaxTokens) {
									t.Errorf("Expected num_predict %v, got %v", tt.options.MaxTokens, maxTokens)
								}
							}
						} else if tt.options.Temperature > 0 || tt.options.MaxTokens > 0 {
							t.Error("Expected options to be present in request")
						}
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

			response, err := client.Complete(ctx, tt.prompt, tt.options)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && response == "" {
				t.Error("Expected non-empty response")
			}
		})
	}
}

func TestOllamaClient_Embed(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		text           string
		serverResponse string
		statusCode     int
		expectError    bool
		expectedPath   string
	}{
		{
			name: "successful_embedding",
			text: testEmbedText,
			serverResponse: `{
				"embedding": [0.1, 0.2, 0.3, 0.4, 0.5]
			}`,
			statusCode:   200,
			expectError:  false,
			expectedPath: "/api/embeddings",
		},
		{
			name: "large_embedding_vector",
			text: "Large text for embedding",
			serverResponse: `{
				"embedding": [` + strings.Repeat("0.1,", 383) + `0.1]
			}`,
			statusCode:   200,
			expectError:  false,
			expectedPath: "/api/embeddings",
		},
		{
			name: "empty_text",
			text: "",
			serverResponse: `{
				"embedding": [0.0, 0.0, 0.0]
			}`,
			statusCode:   200,
			expectError:  false,
			expectedPath: "/api/embeddings",
		},
		{
			name:           "server_error",
			text:           testEmbedText,
			serverResponse: `{"error": "embedding model not available"}`,
			statusCode:     503,
			expectError:    true,
			expectedPath:   "/api/embeddings",
		},
		{
			name:           "invalid_json_response",
			text:           testEmbedText,
			serverResponse: `invalid json`,
			statusCode:     200,
			expectError:    true,
			expectedPath:   "/api/embeddings",
		},
		{
			name: "malformed_embedding_response",
			text: testEmbedText,
			serverResponse: `{
				"embedding": "invalid_format"
			}`,
			statusCode:   200,
			expectError:  true,
			expectedPath: "/api/embeddings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.expectedPath {
					t.Errorf("Expected path %q, got %q", tt.expectedPath, r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Validate request body for successful cases
				if !tt.expectError || tt.statusCode == 200 {
					var reqBody map[string]any
					if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
						t.Errorf("Failed to decode request body: %v", err)
					}

					if model, ok := reqBody["model"].(string); !ok || model != testEmbeddingModel {
						t.Errorf("Expected model %q, got %v", testEmbeddingModel, reqBody["model"])
					}

					if prompt, ok := reqBody["prompt"].(string); !ok || prompt != tt.text {
						t.Errorf("Expected prompt %q, got %v", tt.text, reqBody["prompt"])
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

			embedding, err := client.Embed(ctx, tt.text)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && len(embedding) == 0 {
				t.Error("Expected non-empty embedding vector")
			}
		})
	}
}

func TestOllamaClient_EmbedBatch(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name         string
		texts        []string
		expectError  bool
		expectedSize int
	}{
		{
			name:         "successful_batch_embedding",
			texts:        []string{"Text 1", "Text 2", "Text 3"},
			expectError:  false,
			expectedSize: 3,
		},
		{
			name:         "single_text_batch",
			texts:        []string{"Single text"},
			expectError:  false,
			expectedSize: 1,
		},
		{
			name:         "empty_text_in_batch",
			texts:        []string{"Text 1", "", "Text 3"},
			expectError:  false,
			expectedSize: 3,
		},
		{
			name:         "empty_batch",
			texts:        []string{},
			expectError:  false,
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				w.Write([]byte(`{"embedding": [0.1, 0.2, 0.3, 0.4, 0.5]}`))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

			embeddings, err := client.EmbedBatch(ctx, tt.texts)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if len(embeddings) != tt.expectedSize {
					t.Errorf("Expected %d embeddings, got %d", tt.expectedSize, len(embeddings))
				}

				if callCount != len(tt.texts) {
					t.Errorf("Expected %d API calls, got %d", len(tt.texts), callCount)
				}

				for i, embedding := range embeddings {
					if len(embedding) == 0 {
						t.Errorf("Expected non-empty embedding for text %d", i)
					}
				}
			}
		})
	}
}

func TestOllamaClient_Health(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		expectError    bool
		expectedPath   string
	}{
		{
			name: "successful_health_check",
			serverResponse: `{
				"models": [
					{
						"name": "llama3.2:latest",
						"size": 2000000000,
						"digest": "sha256:abc123",
						"modified_at": "2023-12-07T09:30:00Z"
					}
				]
			}`,
			statusCode:   200,
			expectError:  false,
			expectedPath: "/api/tags",
		},
		{
			name:           "empty_models_list",
			serverResponse: `{"models": []}`,
			statusCode:     200,
			expectError:    false,
			expectedPath:   "/api/tags",
		},
		{
			name:           "server_error",
			serverResponse: `{"error": "internal server error"}`,
			statusCode:     500,
			expectError:    true,
			expectedPath:   "/api/tags",
		},
		{
			name:           "service_unavailable",
			serverResponse: `{"error": "service temporarily unavailable"}`,
			statusCode:     503,
			expectError:    true,
			expectedPath:   "/api/tags",
		},
		{
			name:           "invalid_json_response",
			serverResponse: `invalid json response`,
			statusCode:     200,
			expectError:    true,
			expectedPath:   "/api/tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.expectedPath {
					t.Errorf("Expected path %q, got %q", tt.expectedPath, r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

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

func TestOllamaClient_GetModelInfo(t *testing.T) {
	tests := []struct {
		name             string
		chatModel        string
		expectedName     string
		expectedProvider string
	}{
		{
			name:             "basic_model_info",
			chatModel:        testChatModel,
			expectedName:     testChatModel,
			expectedProvider: "ollama",
		},
		{
			name:             "custom_model_info",
			chatModel:        "custom-model:v2.0",
			expectedName:     "custom-model:v2.0",
			expectedProvider: "ollama",
		},
		{
			name:             "empty_model_name",
			chatModel:        "",
			expectedName:     "",
			expectedProvider: "ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOllamaClient("http://localhost:11434", tt.chatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

			modelInfo := client.GetModelInfo()

			if modelInfo.Name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, modelInfo.Name)
			}

			if modelInfo.Provider != tt.expectedProvider {
				t.Errorf("Expected provider %q, got %q", tt.expectedProvider, modelInfo.Provider)
			}

			if modelInfo.Version == "" {
				// Version should be set to "unknown" as per implementation
				if modelInfo.Version != "unknown" {
					t.Errorf("Expected version to be 'unknown', got %q", modelInfo.Version)
				}
			}

			if modelInfo.ContextSize <= 0 {
				t.Errorf("Expected positive context size, got %d", modelInfo.ContextSize)
			}

			if modelInfo.EmbeddingDim <= 0 {
				t.Errorf("Expected positive embedding dimension, got %d", modelInfo.EmbeddingDim)
			}
		})
	}
}

func TestOllamaClient_Close(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", testChatModel, testEmbeddingModel, testLLMTimeout, testLLMRetries)

	err := client.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got: %v", err)
	}

	// Should be able to call Close multiple times without error
	err = client.Close()
	if err != nil {
		t.Errorf("Expected no error on second close, got: %v", err)
	}
}

func TestNewDOIClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		userAgent   string
		timeout     time.Duration
		rateLimit   time.Duration
		expectedURL string
	}{
		{
			name:        "basic_doi_client_creation",
			baseURL:     "https://api.crossref.org/works",
			userAgent:   "Test DOI Client",
			timeout:     10 * time.Second,
			rateLimit:   1 * time.Second,
			expectedURL: "https://api.crossref.org/works",
		},
		{
			name:        "baseURL_with_trailing_slash",
			baseURL:     "https://api.crossref.org/works/",
			userAgent:   "Test DOI Client",
			timeout:     10 * time.Second,
			rateLimit:   1 * time.Second,
			expectedURL: "https://api.crossref.org/works",
		},
		{
			name:        "no_rate_limit",
			baseURL:     "https://api.crossref.org/works",
			userAgent:   "Test DOI Client",
			timeout:     10 * time.Second,
			rateLimit:   0,
			expectedURL: "https://api.crossref.org/works",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewDOIClient(tt.baseURL, tt.userAgent, tt.timeout, tt.rateLimit)

			if client == nil {
				t.Fatal("Expected DOI client to be created, got nil")
			}

			if client.baseURL != tt.expectedURL {
				t.Errorf("Expected baseURL %q, got %q", tt.expectedURL, client.baseURL)
			}

			if client.userAgent != tt.userAgent {
				t.Errorf("Expected userAgent %q, got %q", tt.userAgent, client.userAgent)
			}

			if client.rateLimit != tt.rateLimit {
				t.Errorf("Expected rateLimit %v, got %v", tt.rateLimit, client.rateLimit)
			}

			if client.httpClient == nil {
				t.Error("Expected httpClient to be initialized")
			}

			if client.httpClient.Timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, client.httpClient.Timeout)
			}
		})
	}
}

func TestDOIClient_ResolveDOI(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name            string
		doi             string
		serverResponse  string
		statusCode      int
		expectError     bool
		expectedDOI     string
		validateRequest bool
	}{
		{
			name: "successful_doi_resolution",
			doi:  "10.1038/nature12373",
			serverResponse: `{
				"DOI": "10.1038/nature12373",
				"title": ["Example Research Paper"],
				"author": [
					{
						"given": "John",
						"family": "Doe",
						"ORCID": "http://orcid.org/0000-0000-0000-0000"
					}
				],
				"published-print": {
					"date-parts": [[2023, 12, 7]]
				},
				"container-title": ["Nature"],
				"volume": "123",
				"issue": "4567",
				"page": "123-456",
				"publisher": "Nature Publishing Group",
				"URL": "https://doi.org/10.1038/nature12373"
			}`,
			statusCode:      200,
			expectError:     false,
			expectedDOI:     "10.1038/nature12373",
			validateRequest: true,
		},
		{
			name: "doi_with_https_prefix",
			doi:  "https://doi.org/10.1038/nature12373",
			serverResponse: `{
				"DOI": "10.1038/nature12373",
				"title": ["Example Research Paper"],
				"author": []
			}`,
			statusCode:      200,
			expectError:     false,
			expectedDOI:     "10.1038/nature12373",
			validateRequest: true,
		},
		{
			name: "doi_with_http_prefix",
			doi:  "http://dx.doi.org/10.1038/nature12373",
			serverResponse: `{
				"DOI": "10.1038/nature12373",
				"title": ["Example Research Paper"],
				"author": []
			}`,
			statusCode:      200,
			expectError:     false,
			expectedDOI:     "10.1038/nature12373",
			validateRequest: true,
		},
		{
			name: "doi_with_doi_prefix",
			doi:  "doi:10.1038/nature12373",
			serverResponse: `{
				"DOI": "10.1038/nature12373",
				"title": ["Example Research Paper"],
				"author": []
			}`,
			statusCode:      200,
			expectError:     false,
			expectedDOI:     "10.1038/nature12373",
			validateRequest: true,
		},
		{
			name:            "doi_not_found",
			doi:             "10.1038/nonexistent",
			serverResponse:  `{"error": "DOI not found"}`,
			statusCode:      404,
			expectError:     true,
			validateRequest: false,
		},
		{
			name:            "server_error",
			doi:             "10.1038/nature12373",
			serverResponse:  `{"error": "internal server error"}`,
			statusCode:      500,
			expectError:     true,
			validateRequest: false,
		},
		{
			name:            "invalid_json_response",
			doi:             "10.1038/nature12373",
			serverResponse:  `invalid json response`,
			statusCode:      200,
			expectError:     true,
			validateRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Expected path should be based on the DOI in the test case
				expectedDOI := "10.1038/nature12373"
				if tt.name == "doi_not_found" {
					expectedDOI = "10.1038/nonexistent"
				}
				expectedPath := "/" + expectedDOI
				if !strings.HasSuffix(r.URL.Path, expectedPath) {
					t.Errorf("Expected path to end with %q, got %q", expectedPath, r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				if tt.validateRequest {
					if accept := r.Header.Get("Accept"); accept != "application/json" {
						t.Errorf("Expected Accept header 'application/json', got %q", accept)
					}

					if userAgent := r.Header.Get("User-Agent"); userAgent == "" {
						t.Error("Expected User-Agent header to be set")
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			client := NewDOIClient(server.URL, "Test DOI Client", 10*time.Second, 0) // No rate limit for tests

			metadata, err := client.ResolveDOI(ctx, tt.doi)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if metadata == nil {
					t.Fatal("Expected metadata to be returned, got nil")
				}

				if metadata.DOI != tt.expectedDOI {
					t.Errorf("Expected DOI %q, got %q", tt.expectedDOI, metadata.DOI)
				}
			}
		})
	}
}

func TestAuthor_FullName(t *testing.T) {
	tests := []struct {
		name         string
		author       Author
		expectedName string
	}{
		{
			name: "full_name_with_both_given_and_family",
			author: Author{
				Given:  "John",
				Family: "Doe",
			},
			expectedName: "John Doe",
		},
		{
			name: "only_family_name",
			author: Author{
				Given:  "",
				Family: "Doe",
			},
			expectedName: "Doe",
		},
		{
			name: "only_given_name",
			author: Author{
				Given:  "John",
				Family: "",
			},
			expectedName: "John",
		},
		{
			name: "empty_names",
			author: Author{
				Given:  "",
				Family: "",
			},
			expectedName: "",
		},
		{
			name: "with_orcid",
			author: Author{
				Given:  "John",
				Family: "Doe",
				ORCID:  "http://orcid.org/0000-0000-0000-0000",
			},
			expectedName: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullName := tt.author.FullName()
			if fullName != tt.expectedName {
				t.Errorf("Expected full name %q, got %q", tt.expectedName, fullName)
			}
		})
	}
}

func TestOllamaClient_RetryLogic(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		maxRetries     int
		serverBehavior func(attempt int) (int, string)
		expectError    bool
		expectedCalls  int
	}{
		{
			name:       "success_on_first_attempt",
			maxRetries: 3,
			serverBehavior: func(attempt int) (int, string) {
				return 200, `{"response": "success"}`
			},
			expectError:   false,
			expectedCalls: 1,
		},
		{
			name:       "success_on_second_attempt",
			maxRetries: 3,
			serverBehavior: func(attempt int) (int, string) {
				if attempt == 0 {
					return 500, `{"error": "server error"}`
				}
				return 200, `{"response": "success after retry"}`
			},
			expectError:   false,
			expectedCalls: 2,
		},
		{
			name:       "fail_after_all_retries",
			maxRetries: 2,
			serverBehavior: func(attempt int) (int, string) {
				return 500, `{"error": "persistent server error"}`
			},
			expectError:   true,
			expectedCalls: 3, // initial + 2 retries
		},
		{
			name:       "no_retries_configured",
			maxRetries: 0,
			serverBehavior: func(attempt int) (int, string) {
				return 500, `{"error": "server error"}`
			},
			expectError:   true,
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				statusCode, response := tt.serverBehavior(callCount)
				callCount++

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)
				w.Write([]byte(response))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, 10*time.Millisecond, tt.maxRetries)

			_, err := client.Complete(ctx, testPrompt, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d calls, got %d", tt.expectedCalls, callCount)
			}
		})
	}
}

func TestOllamaClient_ContextCancellation(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name              string
		cancelAfter       time.Duration
		serverDelay       time.Duration
		expectError       bool
		expectCancelError bool
	}{
		{
			name:              "context_cancelled_before_request",
			cancelAfter:       0, // Cancel immediately
			serverDelay:       100 * time.Millisecond,
			expectError:       true,
			expectCancelError: true,
		},
		{
			name:              "context_cancelled_during_request",
			cancelAfter:       50 * time.Millisecond,
			serverDelay:       200 * time.Millisecond,
			expectError:       true,
			expectCancelError: true,
		},
		{
			name:              "request_completes_before_cancellation",
			cancelAfter:       200 * time.Millisecond,
			serverDelay:       50 * time.Millisecond,
			expectError:       false,
			expectCancelError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.serverDelay)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				w.Write([]byte(`{"response": "success"}`))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, 1*time.Second, 0)

			ctx, cancel := context.WithCancel(ctx)

			// Cancel context after specified duration
			if tt.cancelAfter > 0 {
				go func() {
					time.Sleep(tt.cancelAfter)
					cancel()
				}()
			} else {
				cancel() // Cancel immediately
			}

			_, err := client.Complete(ctx, testPrompt, nil)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectCancelError && err != nil {
				if err != context.Canceled && !strings.Contains(err.Error(), "context canceled") {
					t.Errorf("Expected context cancellation error, got: %v", err)
				}
			}
		})
	}
}

func TestOllamaClient_EmbedBatch_ErrorHandling(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name        string
		texts       []string
		failOnIndex int // Which embedding request should fail
		expectError bool
	}{
		{
			name:        "fail_on_first_embedding",
			texts:       []string{"Text 1", "Text 2", "Text 3"},
			failOnIndex: 0,
			expectError: true,
		},
		{
			name:        "fail_on_middle_embedding",
			texts:       []string{"Text 1", "Text 2", "Text 3"},
			failOnIndex: 1,
			expectError: true,
		},
		{
			name:        "fail_on_last_embedding",
			texts:       []string{"Text 1", "Text 2", "Text 3"},
			failOnIndex: 2,
			expectError: true,
		},
		{
			name:        "all_embeddings_succeed",
			texts:       []string{"Text 1", "Text 2"},
			failOnIndex: -1, // No failure
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { callCount++ }()

				w.Header().Set("Content-Type", "application/json")

				if callCount == tt.failOnIndex {
					w.WriteHeader(500)
					w.Write([]byte(`{"error": "embedding failed"}`))
					return
				}

				w.WriteHeader(200)
				w.Write([]byte(`{"embedding": [0.1, 0.2, 0.3]}`))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, testChatModel, testEmbeddingModel, testLLMTimeout, 0)

			embeddings, err := client.EmbedBatch(ctx, tt.texts)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && len(embeddings) != len(tt.texts) {
				t.Errorf("Expected %d embeddings, got %d", len(tt.texts), len(embeddings))
			}
		})
	}
}

func TestDOIClient_RateLimit(t *testing.T) {
	ctx := t.Context()
	callTimes := make([]time.Time, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"DOI": "10.1038/nature12373",
			"title": ["Example Paper"],
			"author": []
		}`))
	}))
	defer server.Close()

	rateLimit := 100 * time.Millisecond
	client := NewDOIClient(server.URL, "Test DOI Client", 5*time.Second, rateLimit)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		_, err := client.ResolveDOI(ctx, "10.1038/nature12373")
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Check that rate limiting was applied (with some tolerance for timing precision)
	minExpected := rateLimit - (5 * time.Millisecond) // 5ms tolerance

	if len(callTimes) >= 2 {
		timeDiff := callTimes[1].Sub(callTimes[0])
		if timeDiff < minExpected {
			t.Errorf("Expected at least %v between calls, got %v", minExpected, timeDiff)
		}
	}

	if len(callTimes) >= 3 {
		timeDiff := callTimes[2].Sub(callTimes[1])
		if timeDiff < minExpected {
			t.Errorf("Expected at least %v between calls, got %v", minExpected, timeDiff)
		}
	}
}
