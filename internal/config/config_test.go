package config

import (
	"testing"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// TestDefaultConfig tests the creation of default configuration
func TestDefaultConfig(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name   string
		verify func(*testing.T, *Config)
	}{
		{
			name: "default config structure",
			verify: func(t *testing.T, config *Config) {
				if config == nil {
					t.Fatal("DefaultConfig() returned nil")
				}
			},
		},
		{
			name: "server defaults",
			verify: func(t *testing.T, config *Config) {
				server := config.Server
				if server.Host != "0.0.0.0" {
					t.Errorf("Server.Host = %v, want 0.0.0.0", server.Host)
				}
				if server.Port != 8080 {
					t.Errorf("Server.Port = %v, want 8080", server.Port)
				}
				if server.ReadTimeout != 30*time.Second {
					t.Errorf("Server.ReadTimeout = %v, want 30s", server.ReadTimeout)
				}
				if server.WriteTimeout != 30*time.Second {
					t.Errorf("Server.WriteTimeout = %v, want 30s", server.WriteTimeout)
				}
				if server.IdleTimeout != 60*time.Second {
					t.Errorf("Server.IdleTimeout = %v, want 60s", server.IdleTimeout)
				}
				if !server.EnableCORS {
					t.Error("Server.EnableCORS should be true")
				}
				if server.StaticDir != "web/static" {
					t.Errorf("Server.StaticDir = %v, want web/static", server.StaticDir)
				}
				if server.TemplateDir != "web/templates" {
					t.Errorf("Server.TemplateDir = %v, want web/templates", server.TemplateDir)
				}
			},
		},
		{
			name: "LLM defaults",
			verify: func(t *testing.T, config *Config) {
				llm := config.LLM
				if llm.Provider != "ollama" {
					t.Errorf("LLM.Provider = %v, want ollama", llm.Provider)
				}
				if llm.BaseURL != "http://localhost:11434" {
					t.Errorf("LLM.BaseURL = %v, want http://localhost:11434", llm.BaseURL)
				}
				if llm.ChatModel != "llama3.2" {
					t.Errorf("LLM.ChatModel = %v, want llama3.2", llm.ChatModel)
				}
				if llm.EmbeddingModel != "nomic-embed-text" {
					t.Errorf("LLM.EmbeddingModel = %v, want nomic-embed-text", llm.EmbeddingModel)
				}
				if llm.Temperature != 0.1 {
					t.Errorf("LLM.Temperature = %v, want 0.1", llm.Temperature)
				}
				if llm.MaxTokens != 2048 {
					t.Errorf("LLM.MaxTokens = %v, want 2048", llm.MaxTokens)
				}
				if llm.Timeout != 60*time.Second {
					t.Errorf("LLM.Timeout = %v, want 60s", llm.Timeout)
				}
				if llm.RetryAttempts != 3 {
					t.Errorf("LLM.RetryAttempts = %v, want 3", llm.RetryAttempts)
				}
				if llm.ConcurrentCalls != 4 {
					t.Errorf("LLM.ConcurrentCalls = %v, want 4", llm.ConcurrentCalls)
				}
			},
		},
		{
			name: "storage defaults",
			verify: func(t *testing.T, config *Config) {
				storage := config.Storage
				if storage.Type != "chromadb" {
					t.Errorf("Storage.Type = %v, want chromadb", storage.Type)
				}
				if storage.Host != "localhost" {
					t.Errorf("Storage.Host = %v, want localhost", storage.Host)
				}
				if storage.Port != 8000 {
					t.Errorf("Storage.Port = %v, want 8000", storage.Port)
				}
				if storage.Database != "idaes" {
					t.Errorf("Storage.Database = %v, want idaes", storage.Database)
				}
				if storage.DefaultCollection != "documents" {
					t.Errorf("Storage.DefaultCollection = %v, want documents", storage.DefaultCollection)
				}
				if storage.MaxConnections != 10 {
					t.Errorf("Storage.MaxConnections = %v, want 10", storage.MaxConnections)
				}
				if storage.Timeout != 30*time.Second {
					t.Errorf("Storage.Timeout = %v, want 30s", storage.Timeout)
				}
				if storage.RetryAttempts != 3 {
					t.Errorf("Storage.RetryAttempts = %v, want 3", storage.RetryAttempts)
				}
				if !storage.AutoCreateCollections {
					t.Error("Storage.AutoCreateCollections should be true")
				}
				if storage.EmbeddingFunction != "default" {
					t.Errorf("Storage.EmbeddingFunction = %v, want default", storage.EmbeddingFunction)
				}
				if storage.DistanceFunction != "cosine" {
					t.Errorf("Storage.DistanceFunction = %v, want cosine", storage.DistanceFunction)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.verify(t, config)
		})
	}
}

// TestConfigValidate tests configuration validation
func TestConfigValidate(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid server port - negative",
			config: func() *Config {
				c := DefaultConfig()
				c.Server.Port = -1
				return c
			}(),
			wantErr:   true,
			errSubstr: "invalid server port",
		},
		{
			name: "invalid server port - zero",
			config: func() *Config {
				c := DefaultConfig()
				c.Server.Port = 0
				return c
			}(),
			wantErr:   true,
			errSubstr: "invalid server port",
		},
		{
			name: "invalid server port - too high",
			config: func() *Config {
				c := DefaultConfig()
				c.Server.Port = 65536
				return c
			}(),
			wantErr:   true,
			errSubstr: "invalid server port",
		},
		{
			name: "empty LLM provider",
			config: func() *Config {
				c := DefaultConfig()
				c.LLM.Provider = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "LLM provider is required",
		},
		{
			name: "empty LLM base URL",
			config: func() *Config {
				c := DefaultConfig()
				c.LLM.BaseURL = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "LLM base URL is required",
		},
		{
			name: "empty LLM chat model",
			config: func() *Config {
				c := DefaultConfig()
				c.LLM.ChatModel = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "LLM chat model is required",
		},
		{
			name: "empty LLM embedding model",
			config: func() *Config {
				c := DefaultConfig()
				c.LLM.EmbeddingModel = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "LLM embedding model is required",
		},
		{
			name: "empty storage type",
			config: func() *Config {
				c := DefaultConfig()
				c.Storage.Type = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "storage type is required",
		},
		{
			name: "empty storage host",
			config: func() *Config {
				c := DefaultConfig()
				c.Storage.Host = ""
				return c
			}(),
			wantErr:   true,
			errSubstr: "storage host is required",
		},
		{
			name: "invalid storage port - negative",
			config: func() *Config {
				c := DefaultConfig()
				c.Storage.Port = -1
				return c
			}(),
			wantErr:   true,
			errSubstr: "invalid storage port",
		},
		{
			name: "invalid storage port - too high",
			config: func() *Config {
				c := DefaultConfig()
				c.Storage.Port = 65536
				return c
			}(),
			wantErr:   true,
			errSubstr: "invalid storage port",
		},
		{
			name: "invalid batch size",
			config: func() *Config {
				c := DefaultConfig()
				c.Analysis.BatchSize = 0
				return c
			}(),
			wantErr:   true,
			errSubstr: "batch size must be positive",
		},
		{
			name: "invalid max concurrent analyses",
			config: func() *Config {
				c := DefaultConfig()
				c.Performance.MaxConcurrentAnalyses = 0
				return c
			}(),
			wantErr:   true,
			errSubstr: "max concurrent analyses must be positive",
		},
		{
			name: "invalid max document size",
			config: func() *Config {
				c := DefaultConfig()
				c.Performance.MaxDocumentSize = 0
				return c
			}(),
			wantErr:   true,
			errSubstr: "max document size must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errSubstr != "" && !containsString(err.Error(), tt.errSubstr) {
					t.Errorf("Config.Validate() error = %v, want error containing %v", err, tt.errSubstr)
				}
			}
		})
	}
}

// TestLLMConfigToInterfaceConfig tests LLM config conversion
func TestLLMConfigToInterfaceConfig(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name   string
		config LLMConfig
		want   *interfaces.LLMConfig
	}{
		{
			name: "basic conversion",
			config: LLMConfig{
				Provider:       "ollama",
				BaseURL:        "http://localhost:11434",
				ChatModel:      "llama3.2",
				EmbeddingModel: "nomic-embed-text",
				Temperature:    0.1,
				MaxTokens:      2048,
				Timeout:        60 * time.Second,
				RetryAttempts:  3,
			},
			want: &interfaces.LLMConfig{
				Provider:       "ollama",
				BaseURL:        "http://localhost:11434",
				Model:          "llama3.2",
				EmbeddingModel: "nomic-embed-text",
				Temperature:    0.1,
				MaxTokens:      2048,
				Timeout:        60 * time.Second,
				RetryAttempts:  3,
			},
		},
		{
			name:   "empty config",
			config: LLMConfig{},
			want: &interfaces.LLMConfig{
				Provider:       "",
				BaseURL:        "",
				Model:          "",
				EmbeddingModel: "",
				Temperature:    0,
				MaxTokens:      0,
				Timeout:        0,
				RetryAttempts:  0,
			},
		},
		{
			name: "custom values",
			config: LLMConfig{
				Provider:       "openai",
				BaseURL:        "https://api.openai.com/v1",
				ChatModel:      "gpt-4",
				EmbeddingModel: "text-embedding-ada-002",
				Temperature:    0.7,
				MaxTokens:      4096,
				Timeout:        120 * time.Second,
				RetryAttempts:  5,
			},
			want: &interfaces.LLMConfig{
				Provider:       "openai",
				BaseURL:        "https://api.openai.com/v1",
				Model:          "gpt-4",
				EmbeddingModel: "text-embedding-ada-002",
				Temperature:    0.7,
				MaxTokens:      4096,
				Timeout:        120 * time.Second,
				RetryAttempts:  5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ToInterfaceConfig()

			if got == nil {
				t.Fatal("ToInterfaceConfig() returned nil")
			}

			if got.Provider != tt.want.Provider {
				t.Errorf("Provider = %v, want %v", got.Provider, tt.want.Provider)
			}
			if got.BaseURL != tt.want.BaseURL {
				t.Errorf("BaseURL = %v, want %v", got.BaseURL, tt.want.BaseURL)
			}
			if got.Model != tt.want.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.want.Model)
			}
			if got.EmbeddingModel != tt.want.EmbeddingModel {
				t.Errorf("EmbeddingModel = %v, want %v", got.EmbeddingModel, tt.want.EmbeddingModel)
			}
			if got.Temperature != tt.want.Temperature {
				t.Errorf("Temperature = %v, want %v", got.Temperature, tt.want.Temperature)
			}
			if got.MaxTokens != tt.want.MaxTokens {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.want.MaxTokens)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
			if got.RetryAttempts != tt.want.RetryAttempts {
				t.Errorf("RetryAttempts = %v, want %v", got.RetryAttempts, tt.want.RetryAttempts)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestEntityExtractionConfigToInterfaceConfig tests entity extraction config conversion
func TestEntityExtractionConfigToInterfaceConfig(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name      string
		config    EntityExtractionConfig
		llmConfig *interfaces.LLMConfig
		verify    func(*testing.T, *interfaces.EntityExtractionConfig)
	}{
		{
			name: "basic conversion",
			config: EntityExtractionConfig{
				Enabled:        true,
				DefaultMethod:  types.ExtractorMethodRegex,
				MinConfidence:  0.8,
				MaxEntities:    100,
				IncludeContext: true,
				Deduplicate:    true,
				EntityTypes:    []types.EntityType{types.EntityTypePerson, types.EntityTypeOrganization},
				CacheEnabled:   true,
				CacheTTL:       5 * time.Minute,
			},
			llmConfig: &interfaces.LLMConfig{Provider: "ollama"},
			verify: func(t *testing.T, config *interfaces.EntityExtractionConfig) {
				if !config.Enabled {
					t.Error("Enabled should be true")
				}
				if config.Method != types.ExtractorMethodRegex {
					t.Errorf("Method = %v, want %v", config.Method, types.ExtractorMethodRegex)
				}
				if config.MinConfidence != 0.8 {
					t.Errorf("MinConfidence = %v, want 0.8", config.MinConfidence)
				}
				if config.MaxEntities != 100 {
					t.Errorf("MaxEntities = %v, want 100", config.MaxEntities)
				}
				if !config.IncludeContext {
					t.Error("IncludeContext should be true")
				}
				if !config.Deduplicate {
					t.Error("Deduplicate should be true")
				}
				if len(config.EntityTypes) != 2 {
					t.Errorf("EntityTypes length = %v, want 2", len(config.EntityTypes))
				}
				if !config.CacheEnabled {
					t.Error("CacheEnabled should be true")
				}
				if config.CacheTTL != 5*time.Minute {
					t.Errorf("CacheTTL = %v, want 5m", config.CacheTTL)
				}
				if config.LLMConfig == nil {
					t.Error("LLMConfig should not be nil")
				}
			},
		},
		{
			name:      "empty config",
			config:    EntityExtractionConfig{},
			llmConfig: nil,
			verify: func(t *testing.T, config *interfaces.EntityExtractionConfig) {
				if config.Enabled {
					t.Error("Enabled should be false")
				}
				if config.MinConfidence != 0 {
					t.Errorf("MinConfidence = %v, want 0", config.MinConfidence)
				}
				if config.LLMConfig != nil {
					t.Error("LLMConfig should be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ToInterfaceConfig(tt.llmConfig)
			if got == nil {
				t.Fatal("ToInterfaceConfig() returned nil")
			}
			tt.verify(t, got)
		})
	}
}

// TestCitationExtractionConfigToInterfaceConfig tests citation extraction config conversion
func TestCitationExtractionConfigToInterfaceConfig(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name      string
		config    CitationExtractionConfig
		llmConfig *interfaces.LLMConfig
		verify    func(*testing.T, *interfaces.CitationExtractionConfig)
	}{
		{
			name: "basic conversion",
			config: CitationExtractionConfig{
				Enabled:            true,
				DefaultMethod:      types.ExtractorMethodRegex,
				MinConfidence:      0.7,
				MaxCitations:       50,
				Formats:            []types.CitationFormat{types.CitationFormatAPA, types.CitationFormatMLA},
				EnrichWithDOI:      true,
				EnrichWithMetadata: true,
				ValidateReferences: true,
				DOIConfig: DOIConfig{
					Enabled:      true,
					CrossRefURL:  "https://api.crossref.org/works/",
					Timeout:      30 * time.Second,
					UserAgent:    "IDAES/1.0",
					RateLimit:    time.Second,
					CacheEnabled: true,
					MaxCacheSize: 1000,
				},
				CacheEnabled: true,
				CacheTTL:     10 * time.Minute,
			},
			llmConfig: &interfaces.LLMConfig{Provider: "ollama"},
			verify: func(t *testing.T, config *interfaces.CitationExtractionConfig) {
				if !config.Enabled {
					t.Error("Enabled should be true")
				}
				if config.Method != types.ExtractorMethodRegex {
					t.Errorf("Method = %v, want %v", config.Method, types.ExtractorMethodRegex)
				}
				if config.MinConfidence != 0.7 {
					t.Errorf("MinConfidence = %v, want 0.7", config.MinConfidence)
				}
				if config.MaxCitations != 50 {
					t.Errorf("MaxCitations = %v, want 50", config.MaxCitations)
				}
				if len(config.Formats) != 2 {
					t.Errorf("Formats length = %v, want 2", len(config.Formats))
				}
				if !config.EnrichWithDOI {
					t.Error("EnrichWithDOI should be true")
				}
				if !config.EnrichWithMetadata {
					t.Error("EnrichWithMetadata should be true")
				}
				if !config.ValidateReferences {
					t.Error("ValidateReferences should be true")
				}
				if config.DOIConfig == nil {
					t.Fatal("DOIConfig should not be nil")
				}
				if !config.DOIConfig.Enabled {
					t.Error("DOIConfig.Enabled should be true")
				}
				if config.DOIConfig.CrossRefURL != "https://api.crossref.org/works/" {
					t.Errorf("DOIConfig.CrossRefURL = %v, want https://api.crossref.org/works/", config.DOIConfig.CrossRefURL)
				}
				if !config.CacheEnabled {
					t.Error("CacheEnabled should be true")
				}
				if config.LLMConfig == nil {
					t.Error("LLMConfig should not be nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ToInterfaceConfig(tt.llmConfig)
			if got == nil {
				t.Fatal("ToInterfaceConfig() returned nil")
			}
			tt.verify(t, got)
		})
	}
}

// TestTopicExtractionConfigToInterfaceConfig tests topic extraction config conversion
func TestTopicExtractionConfigToInterfaceConfig(t *testing.T) {
	ctx := t.Context()
	_ = ctx // Context available for future use if needed

	tests := []struct {
		name      string
		config    TopicExtractionConfig
		llmConfig *interfaces.LLMConfig
		verify    func(*testing.T, *interfaces.TopicExtractionConfig)
	}{
		{
			name: "basic conversion",
			config: TopicExtractionConfig{
				Enabled:          true,
				DefaultMethod:    "statistical",
				NumTopics:        10,
				MinConfidence:    0.6,
				MaxKeywords:      20,
				EnableClustering: true,
			},
			llmConfig: &interfaces.LLMConfig{Provider: "ollama"},
			verify: func(t *testing.T, config *interfaces.TopicExtractionConfig) {
				if !config.Enabled {
					t.Error("Enabled should be true")
				}
				if config.Method != "statistical" {
					t.Errorf("Method = %v, want statistical", config.Method)
				}
				if config.NumTopics != 10 {
					t.Errorf("NumTopics = %v, want 10", config.NumTopics)
				}
				if config.MinConfidence != 0.6 {
					t.Errorf("MinConfidence = %v, want 0.6", config.MinConfidence)
				}
				if config.MaxKeywords != 20 {
					t.Errorf("MaxKeywords = %v, want 20", config.MaxKeywords)
				}
				if !config.EnableClustering {
					t.Error("EnableClustering should be true")
				}
			},
		},
		{
			name:      "empty config",
			config:    TopicExtractionConfig{},
			llmConfig: nil,
			verify: func(t *testing.T, config *interfaces.TopicExtractionConfig) {
				if config.Enabled {
					t.Error("Enabled should be false")
				}
				if config.NumTopics != 0 {
					t.Errorf("NumTopics = %v, want 0", config.NumTopics)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ToInterfaceConfig(tt.llmConfig)
			if got == nil {
				t.Fatal("ToInterfaceConfig() returned nil")
			}
			tt.verify(t, got)
		})
	}
}
