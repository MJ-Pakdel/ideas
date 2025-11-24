// Package clients provides implementations of external service clients.package clients

// This package contains clients for LLM services (Ollama), DOI resolution
// services (CrossRef), and other external APIs used by the IDAES system.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/example/idaes/internal/interfaces"
)

// OllamaClient implements the LLMClient interface for Ollama
type OllamaClient struct {
	baseURL        string
	chatModel      string
	embeddingModel string
	httpClient     *http.Client
	maxRetries     int
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, chatModel, embeddingModel string, timeout time.Duration, maxRetries int) *OllamaClient {
	return &OllamaClient{
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
	}
}

// Complete generates text completion using the chat model
func (c *OllamaClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
	request := map[string]any{
		"model":  c.chatModel,
		"prompt": prompt,
		"stream": false,
	}

	if options != nil {
		if options.Temperature > 0 {
			request["options"] = map[string]any{
				"temperature": options.Temperature,
			}
		}
		if options.MaxTokens > 0 {
			if opts, ok := request["options"].(map[string]any); ok {
				opts["num_predict"] = options.MaxTokens
			} else {
				request["options"] = map[string]any{
					"num_predict": options.MaxTokens,
				}
			}
		}
	}

	body, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var response GenerateResponse
	err = c.makeRequest(ctx, "POST", "/api/generate", body, &response)
	if err != nil {
		return "", fmt.Errorf("failed to complete: %w", err)
	}

	return response.Response, nil
}

// Embed generates embeddings for text using the embedding model
func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error) {
	request := map[string]any{
		"model":  c.embeddingModel,
		"prompt": text,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var response EmbeddingResponse
	err = c.makeRequest(ctx, "POST", "/api/embeddings", body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to embed: %w", err)
	}

	return response.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))

	for i, text := range texts {
		embedding, err := c.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// Health checks if the Ollama service is healthy
func (c *OllamaClient) Health(ctx context.Context) error {
	var response any
	err := c.makeRequest(ctx, "GET", "/api/tags", nil, &response)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// GetModelInfo returns information about the models
func (c *OllamaClient) GetModelInfo() interfaces.ModelInfo {
	return interfaces.ModelInfo{
		Name:         c.chatModel,
		Provider:     "ollama",
		Version:      "unknown", // Ollama doesn't provide version info in API
		ContextSize:  4096,      // Default context size
		EmbeddingDim: 384,       // Default for nomic-embed-text
	}
}

// Close closes the client connection
func (c *OllamaClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// makeRequest makes an HTTP request to the Ollama API with retries
func (c *OllamaClient) makeRequest(ctx context.Context, method, path string, body []byte, response any) error {
	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with millisecond delays for faster tests
			waitTime := time.Duration(attempt) * 10 * time.Millisecond
			slog.DebugContext(ctx, "Retrying request", "attempt", attempt, "wait", waitTime)

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
			slog.DebugContext(ctx, "Request failed", "attempt", attempt, "error", err)
			continue
		}
		if resp == nil {
			lastErr = fmt.Errorf("received nil response")
			slog.DebugContext(ctx, "Request returned nil response", "attempt", attempt)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
			slog.DebugContext(ctx, "Request returned error status", "status", resp.StatusCode, "response", string(bodyBytes))
			continue
		}

		if response != nil {
			if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
				lastErr = err
				slog.DebugContext(ctx, "Failed to decode response", "error", err)
				continue
			}
		}

		return nil
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// GenerateResponse represents Ollama's generate API response
type GenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
}

// EmbeddingResponse represents Ollama's embedding API response
type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// TagsResponse represents Ollama's tags API response
type TagsResponse struct {
	Models []ModelDetail `json:"models"`
}

// ModelDetail represents details about an Ollama model
type ModelDetail struct {
	Name       string         `json:"name"`
	Size       int64          `json:"size"`
	Digest     string         `json:"digest"`
	ModifiedAt time.Time      `json:"modified_at"`
	Details    map[string]any `json:"details,omitempty"`
}

// DOIClient implements DOI resolution using CrossRef
type DOIClient struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	rateLimit  time.Duration
	lastCall   time.Time
}

// NewDOIClient creates a new DOI resolution client
func NewDOIClient(baseURL, userAgent string, timeout, rateLimit time.Duration) *DOIClient {
	return &DOIClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		rateLimit: rateLimit,
	}
}

// ResolveDOI resolves a DOI to citation metadata
func (d *DOIClient) ResolveDOI(ctx context.Context, doi string) (*DOIMetadata, error) {
	// Respect rate limit
	if d.rateLimit > 0 {
		elapsed := time.Since(d.lastCall)
		if elapsed < d.rateLimit {
			wait := d.rateLimit - elapsed
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
		d.lastCall = time.Now()
	}

	// Clean DOI
	doi = strings.TrimPrefix(doi, "https://doi.org/")
	doi = strings.TrimPrefix(doi, "http://dx.doi.org/")
	doi = strings.TrimPrefix(doi, "doi:")

	url := fmt.Sprintf("%s/%s", d.baseURL, doi)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DOI resolution failed with status %d", resp.StatusCode)
	}

	var metadata DOIMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &metadata, nil
}

// DOIMetadata represents metadata from CrossRef
type DOIMetadata struct {
	DOI       string   `json:"DOI"`
	Title     []string `json:"title"`
	Author    []Author `json:"author"`
	Published struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"published-print"`
	ContainerTitle []string `json:"container-title"`
	Volume         string   `json:"volume"`
	Issue          string   `json:"issue"`
	Page           string   `json:"page"`
	Publisher      string   `json:"publisher"`
	URL            string   `json:"URL"`
}

// Author represents an author from CrossRef metadata
type Author struct {
	Given  string `json:"given"`
	Family string `json:"family"`
	ORCID  string `json:"ORCID,omitempty"`
}

// FullName returns the author's full name
func (a Author) FullName() string {
	if a.Given == "" {
		return a.Family
	}
	if a.Family == "" {
		return a.Given
	}
	return fmt.Sprintf("%s %s", a.Given, a.Family)
}
