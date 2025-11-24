// Package config provides configuration management for the IDAES system.package config

// This package handles loading, validation, and management of all system
// configuration including extraction methods, LLM clients, storage, and
// analysis pipelines.
package config

import (
	"fmt"
	"time"

	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// Config represents the complete system configuration
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`

	// LLM configuration
	LLM LLMConfig `yaml:"llm" json:"llm"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage" json:"storage"`

	// Analysis configuration
	Analysis AnalysisConfig `yaml:"analysis" json:"analysis"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Performance configuration
	Performance PerformanceConfig `yaml:"performance" json:"performance"`
}

// ServerConfig contains web server configuration
type ServerConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	EnableCORS   bool          `yaml:"enable_cors" json:"enable_cors"`
	StaticDir    string        `yaml:"static_dir" json:"static_dir"`
	TemplateDir  string        `yaml:"template_dir" json:"template_dir"`
}

// LLMConfig contains LLM service configuration
type LLMConfig struct {
	Provider        string        `yaml:"provider" json:"provider"`
	BaseURL         string        `yaml:"base_url" json:"base_url"`
	ChatModel       string        `yaml:"chat_model" json:"chat_model"`
	EmbeddingModel  string        `yaml:"embedding_model" json:"embedding_model"`
	Temperature     float64       `yaml:"temperature" json:"temperature"`
	MaxTokens       int           `yaml:"max_tokens" json:"max_tokens"`
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`
	RetryAttempts   int           `yaml:"retry_attempts" json:"retry_attempts"`
	ConcurrentCalls int           `yaml:"concurrent_calls" json:"concurrent_calls"`
}

// StorageConfig contains storage system configuration
type StorageConfig struct {
	Type              string `yaml:"type" json:"type"` // "chromadb"
	Host              string `yaml:"host" json:"host"`
	Port              int    `yaml:"port" json:"port"`
	Database          string `yaml:"database" json:"database"`
	DefaultCollection string `yaml:"default_collection" json:"default_collection"`

	// Connection settings
	MaxConnections int           `yaml:"max_connections" json:"max_connections"`
	Timeout        time.Duration `yaml:"timeout" json:"timeout"`
	RetryAttempts  int           `yaml:"retry_attempts" json:"retry_attempts"`

	// Collection settings
	AutoCreateCollections bool   `yaml:"auto_create_collections" json:"auto_create_collections"`
	EmbeddingFunction     string `yaml:"embedding_function" json:"embedding_function"`
	DistanceFunction      string `yaml:"distance_function" json:"distance_function"`
}

// AnalysisConfig contains analysis pipeline configuration
type AnalysisConfig struct {
	// Entity extraction
	EntityExtraction EntityExtractionConfig `yaml:"entity_extraction" json:"entity_extraction"`

	// Citation extraction
	CitationExtraction CitationExtractionConfig `yaml:"citation_extraction" json:"citation_extraction"`

	// Topic extraction
	TopicExtraction TopicExtractionConfig `yaml:"topic_extraction" json:"topic_extraction"`

	// Document classification
	DocumentClassification DocumentClassificationConfig `yaml:"document_classification" json:"document_classification"`

	// Analysis settings
	EnableSummary       bool `yaml:"enable_summary" json:"enable_summary"`
	EnableRelationships bool `yaml:"enable_relationships" json:"enable_relationships"`
	ParallelProcessing  bool `yaml:"parallel_processing" json:"parallel_processing"`
	BatchSize           int  `yaml:"batch_size" json:"batch_size"`
}

// EntityExtractionConfig contains entity extraction configuration
type EntityExtractionConfig struct {
	Enabled        bool                  `yaml:"enabled" json:"enabled"`
	DefaultMethod  types.ExtractorMethod `yaml:"default_method" json:"default_method"`
	MinConfidence  float64               `yaml:"min_confidence" json:"min_confidence"`
	MaxEntities    int                   `yaml:"max_entities" json:"max_entities"`
	IncludeContext bool                  `yaml:"include_context" json:"include_context"`
	Deduplicate    bool                  `yaml:"deduplicate" json:"deduplicate"`
	EntityTypes    []types.EntityType    `yaml:"entity_types" json:"entity_types"`

	// Caching
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// CitationExtractionConfig contains citation extraction configuration
type CitationExtractionConfig struct {
	Enabled            bool                   `yaml:"enabled" json:"enabled"`
	DefaultMethod      types.ExtractorMethod  `yaml:"default_method" json:"default_method"`
	MinConfidence      float64                `yaml:"min_confidence" json:"min_confidence"`
	MaxCitations       int                    `yaml:"max_citations" json:"max_citations"`
	Formats            []types.CitationFormat `yaml:"formats" json:"formats"`
	EnrichWithDOI      bool                   `yaml:"enrich_with_doi" json:"enrich_with_doi"`
	EnrichWithMetadata bool                   `yaml:"enrich_with_metadata" json:"enrich_with_metadata"`
	ValidateReferences bool                   `yaml:"validate_references" json:"validate_references"`

	// DOI configuration
	DOIConfig DOIConfig `yaml:"doi_config" json:"doi_config"`

	// Caching
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// TopicExtractionConfig contains topic extraction configuration
type TopicExtractionConfig struct {
	Enabled          bool    `yaml:"enabled" json:"enabled"`
	DefaultMethod    string  `yaml:"default_method" json:"default_method"` // "statistical", "semantic", "llm"
	NumTopics        int     `yaml:"num_topics" json:"num_topics"`
	MinConfidence    float64 `yaml:"min_confidence" json:"min_confidence"`
	MaxKeywords      int     `yaml:"max_keywords" json:"max_keywords"`
	EnableClustering bool    `yaml:"enable_clustering" json:"enable_clustering"`

	// Caching
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// DocumentClassificationConfig contains document classification configuration
type DocumentClassificationConfig struct {
	Enabled       bool    `yaml:"enabled" json:"enabled"`
	DefaultMethod string  `yaml:"default_method" json:"default_method"` // "statistical", "llm"
	MinConfidence float64 `yaml:"min_confidence" json:"min_confidence"`

	// Caching
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

// DOIConfig contains DOI resolution configuration
type DOIConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	CrossRefURL  string        `yaml:"crossref_url" json:"crossref_url"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	UserAgent    string        `yaml:"user_agent" json:"user_agent"`
	RateLimit    time.Duration `yaml:"rate_limit" json:"rate_limit"`
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	MaxCacheSize int           `yaml:"max_cache_size" json:"max_cache_size"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`   // "debug", "info", "warn", "error"
	Format string `yaml:"format" json:"format"` // "json", "text"
	Output string `yaml:"output" json:"output"` // "stdout", "stderr", file path
}

// PerformanceConfig contains performance tuning configuration
type PerformanceConfig struct {
	MaxConcurrentAnalyses int           `yaml:"max_concurrent_analyses" json:"max_concurrent_analyses"`
	AnalysisTimeout       time.Duration `yaml:"analysis_timeout" json:"analysis_timeout"`
	MaxDocumentSize       int64         `yaml:"max_document_size" json:"max_document_size"`
	EnableMetrics         bool          `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsInterval       time.Duration `yaml:"metrics_interval" json:"metrics_interval"`

	// Memory management
	MaxMemoryUsage int64         `yaml:"max_memory_usage" json:"max_memory_usage"`
	GCInterval     time.Duration `yaml:"gc_interval" json:"gc_interval"`

	// Caching
	GlobalCacheEnabled bool          `yaml:"global_cache_enabled" json:"global_cache_enabled"`
	GlobalCacheSize    int           `yaml:"global_cache_size" json:"global_cache_size"`
	GlobalCacheTTL     time.Duration `yaml:"global_cache_ttl" json:"global_cache_ttl"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			EnableCORS:   true,
			StaticDir:    "web/static",
			TemplateDir:  "web/templates",
		},
		LLM: LLMConfig{
			Provider:        "ollama",
			BaseURL:         "http://localhost:11434",
			ChatModel:       "llama3.2",
			EmbeddingModel:  "nomic-embed-text",
			Temperature:     0.1,
			MaxTokens:       2048,
			Timeout:         60 * time.Second,
			RetryAttempts:   3,
			ConcurrentCalls: 4,
		},
		Storage: StorageConfig{
			Type:                  "chromadb",
			Host:                  "localhost",
			Port:                  8000,
			Database:              "idaes",
			DefaultCollection:     "documents",
			MaxConnections:        10,
			Timeout:               30 * time.Second,
			RetryAttempts:         3,
			AutoCreateCollections: true,
			EmbeddingFunction:     "default",
			DistanceFunction:      "cosine",
		},
		Analysis: AnalysisConfig{
			EntityExtraction: EntityExtractionConfig{
				Enabled:        true,
				DefaultMethod:  types.ExtractorMethodLLM,
				MinConfidence:  0.7,
				MaxEntities:    100,
				IncludeContext: true,
				Deduplicate:    true,
				EntityTypes: []types.EntityType{
					types.EntityTypePerson,
					types.EntityTypeOrganization,
					types.EntityTypeLocation,
					types.EntityTypeConcept,
				},
				CacheEnabled: true,
				CacheTTL:     1 * time.Hour,
			},
			CitationExtraction: CitationExtractionConfig{
				Enabled:       true,
				DefaultMethod: types.ExtractorMethodLLM,
				MinConfidence: 0.7,
				MaxCitations:  50,
				Formats: []types.CitationFormat{
					types.CitationFormatAPA,
					types.CitationFormatMLA,
					types.CitationFormatChicago,
					types.CitationFormatIEEE,
				},
				EnrichWithDOI:      true,
				EnrichWithMetadata: true,
				ValidateReferences: true,
				DOIConfig: DOIConfig{
					Enabled:      true,
					CrossRefURL:  "https://api.crossref.org/works/",
					Timeout:      10 * time.Second,
					UserAgent:    "IDAES/1.0",
					RateLimit:    1 * time.Second,
					CacheEnabled: true,
					MaxCacheSize: 1000,
				},
				CacheEnabled: true,
				CacheTTL:     1 * time.Hour,
			},
			TopicExtraction: TopicExtractionConfig{
				Enabled:          true,
				DefaultMethod:    "llm",
				NumTopics:        10,
				MinConfidence:    0.6,
				MaxKeywords:      20,
				EnableClustering: true,
				CacheEnabled:     true,
				CacheTTL:         2 * time.Hour,
			},
			DocumentClassification: DocumentClassificationConfig{
				Enabled:       true,
				DefaultMethod: "llm",
				MinConfidence: 0.7,
				CacheEnabled:  true,
				CacheTTL:      4 * time.Hour,
			},
			EnableSummary:       true,
			EnableRelationships: true,
			ParallelProcessing:  true,
			BatchSize:           10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Performance: PerformanceConfig{
			MaxConcurrentAnalyses: 4,
			AnalysisTimeout:       5 * time.Minute,
			MaxDocumentSize:       50 * 1024 * 1024, // 50MB
			EnableMetrics:         true,
			MetricsInterval:       1 * time.Minute,
			MaxMemoryUsage:        2 * 1024 * 1024 * 1024, // 2GB
			GCInterval:            5 * time.Minute,
			GlobalCacheEnabled:    true,
			GlobalCacheSize:       1000,
			GlobalCacheTTL:        1 * time.Hour,
		},
	}
}

// ToEntityExtractionConfig converts to interfaces.EntityExtractionConfig
func (c *EntityExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.EntityExtractionConfig {
	return &interfaces.EntityExtractionConfig{
		Enabled:        c.Enabled,
		Method:         c.DefaultMethod,
		MinConfidence:  c.MinConfidence,
		MaxEntities:    c.MaxEntities,
		IncludeContext: c.IncludeContext,
		Deduplicate:    c.Deduplicate,
		EntityTypes:    c.EntityTypes,
		PrimaryMethod:  types.ExtractorMethodLLM, // Default to LLM for intelligent analysis
		FallbackMethod: types.ExtractorMethodLLM, // No fallback needed - LLM only
		HybridStrategy: "",                       // No hybrid strategy
		LLMConfig:      llmConfig,
		CacheEnabled:   c.CacheEnabled,
		CacheTTL:       c.CacheTTL,
	}
}

// ToCitationExtractionConfig converts to interfaces.CitationExtractionConfig
func (c *CitationExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.CitationExtractionConfig {
	return &interfaces.CitationExtractionConfig{
		Enabled:            c.Enabled,
		Method:             c.DefaultMethod,
		MinConfidence:      c.MinConfidence,
		MaxCitations:       c.MaxCitations,
		Formats:            c.Formats,
		EnrichWithDOI:      c.EnrichWithDOI,
		EnrichWithMetadata: c.EnrichWithMetadata,
		ValidateReferences: c.ValidateReferences,
		PrimaryMethod:      types.ExtractorMethodLLM, // Default to LLM for intelligent analysis
		FallbackMethod:     types.ExtractorMethodLLM, // No fallback needed - LLM only
		HybridStrategy:     "",                       // No hybrid strategy
		LLMConfig:          llmConfig,
		DOIConfig: &interfaces.DOIConfig{
			Enabled:      c.DOIConfig.Enabled,
			CrossRefURL:  c.DOIConfig.CrossRefURL,
			Timeout:      c.DOIConfig.Timeout,
			UserAgent:    c.DOIConfig.UserAgent,
			RateLimit:    c.DOIConfig.RateLimit,
			CacheEnabled: c.DOIConfig.CacheEnabled,
			MaxCacheSize: c.DOIConfig.MaxCacheSize,
		},
		CacheEnabled: c.CacheEnabled,
		CacheTTL:     c.CacheTTL,
	}
}

// ToTopicExtractionConfig converts to interfaces.TopicExtractionConfig
func (c *TopicExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.TopicExtractionConfig {
	return &interfaces.TopicExtractionConfig{
		Enabled:          c.Enabled,
		Method:           c.DefaultMethod,
		NumTopics:        c.NumTopics,
		MinConfidence:    c.MinConfidence,
		MaxKeywords:      c.MaxKeywords,
		EnableClustering: c.EnableClustering,
		LLMConfig:        llmConfig,
		CacheEnabled:     c.CacheEnabled,
		CacheTTL:         c.CacheTTL,
	}
}

// ToLLMConfig converts to interfaces.LLMConfig
func (c *LLMConfig) ToInterfaceConfig() *interfaces.LLMConfig {
	return &interfaces.LLMConfig{
		Provider:       c.Provider,
		BaseURL:        c.BaseURL,
		Model:          c.ChatModel,
		EmbeddingModel: c.EmbeddingModel,
		Temperature:    c.Temperature,
		MaxTokens:      c.MaxTokens,
		Timeout:        c.Timeout,
		RetryAttempts:  c.RetryAttempts,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate LLM config
	if c.LLM.Provider == "" {
		return fmt.Errorf("LLM provider is required")
	}
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("LLM base URL is required")
	}
	if c.LLM.ChatModel == "" {
		return fmt.Errorf("LLM chat model is required")
	}
	if c.LLM.EmbeddingModel == "" {
		return fmt.Errorf("LLM embedding model is required")
	}

	// Validate storage config
	if c.Storage.Type == "" {
		return fmt.Errorf("storage type is required")
	}
	if c.Storage.Host == "" {
		return fmt.Errorf("storage host is required")
	}
	if c.Storage.Port <= 0 || c.Storage.Port > 65535 {
		return fmt.Errorf("invalid storage port: %d", c.Storage.Port)
	}

	// Validate analysis config
	if c.Analysis.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	// Validate performance config
	if c.Performance.MaxConcurrentAnalyses <= 0 {
		return fmt.Errorf("max concurrent analyses must be positive")
	}
	if c.Performance.MaxDocumentSize <= 0 {
		return fmt.Errorf("max document size must be positive")
	}

	return nil
}
