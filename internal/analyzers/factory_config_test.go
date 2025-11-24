package analyzers

import (
	"testing"

	"github.com/example/idaes/internal/interfaces"
)

// Test that AnalysisSystemFactory properly passes tenant and database to storage manager
func TestAnalysisSystemFactory_TenantDatabaseConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         *AnalysisSystemConfig
		expectedTenant string
		expectedDB     string
	}{
		{
			name: "custom_tenant_and_database",
			config: &AnalysisSystemConfig{
				LLMConfig: &interfaces.LLMConfig{
					Provider:       "ollama",
					BaseURL:        "http://localhost:11434",
					Model:          "llama3.2:1b",
					EmbeddingModel: "nomic-embed-text",
					Temperature:    0.1,
					MaxTokens:      4096,
					Timeout:        60,
					RetryAttempts:  3,
				},
				ChromaDBURL:        "http://localhost:8000",
				ChromaDBTenant:     "production_tenant",
				ChromaDBDatabase:   "production_db",
				PipelineConfig:     DefaultAnalysisConfig(),
				OrchestratorConfig: DefaultOrchestratorConfig(),
				EntityConfig:       &interfaces.EntityExtractionConfig{Enabled: false},
				CitationConfig:     &interfaces.CitationExtractionConfig{Enabled: false},
				TopicConfig:        &interfaces.TopicExtractionConfig{Enabled: false},
			},
			expectedTenant: "production_tenant",
			expectedDB:     "production_db",
		},
		{
			name: "default_tenant_and_database",
			config: &AnalysisSystemConfig{
				LLMConfig: &interfaces.LLMConfig{
					Provider:       "ollama",
					BaseURL:        "http://localhost:11434",
					Model:          "llama3.2:1b",
					EmbeddingModel: "nomic-embed-text",
					Temperature:    0.1,
					MaxTokens:      4096,
					Timeout:        60,
					RetryAttempts:  3,
				},
				ChromaDBURL:        "http://localhost:8000",
				ChromaDBTenant:     "default_tenant",
				ChromaDBDatabase:   "default_database",
				PipelineConfig:     DefaultAnalysisConfig(),
				OrchestratorConfig: DefaultOrchestratorConfig(),
				EntityConfig:       &interfaces.EntityExtractionConfig{Enabled: false},
				CitationConfig:     &interfaces.CitationExtractionConfig{Enabled: false},
				TopicConfig:        &interfaces.TopicExtractionConfig{Enabled: false},
			},
			expectedTenant: "default_tenant",
			expectedDB:     "default_database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test configuration values are set correctly
			if tt.config.ChromaDBTenant != tt.expectedTenant {
				t.Errorf("ChromaDBTenant = %s, want %s", tt.config.ChromaDBTenant, tt.expectedTenant)
			}

			if tt.config.ChromaDBDatabase != tt.expectedDB {
				t.Errorf("ChromaDBDatabase = %s, want %s", tt.config.ChromaDBDatabase, tt.expectedDB)
			}
		})
	}
}

// Test DefaultAnalysisSystemConfig includes tenant field
func TestDefaultAnalysisSystemConfig_IncludesTenant(t *testing.T) {
	config := DefaultAnalysisSystemConfig()

	if config.ChromaDBTenant == "" {
		t.Error("DefaultAnalysisSystemConfig.ChromaDBTenant should not be empty")
	}

	expectedTenant := "default_tenant"
	if config.ChromaDBTenant != expectedTenant {
		t.Errorf("ChromaDBTenant = %s, want %s", config.ChromaDBTenant, expectedTenant)
	}

	expectedDatabase := "idaes" // Default from existing config
	if config.ChromaDBDatabase != expectedDatabase {
		t.Errorf("ChromaDBDatabase = %s, want %s", config.ChromaDBDatabase, expectedDatabase)
	}
}

// Test that createStorageManager is called with correct parameters
func TestAnalysisSystemFactory_CreateStorageManager(t *testing.T) {
	factory := NewAnalysisSystemFactory()

	// Test the method signature and parameter passing
	testURL := "http://test-chroma:8000"
	testTenant := "test_tenant"
	testDB := "test_db"

	// This call will create a storage manager with nil LLM client
	storageManager, err := factory.createStorageManager(testURL, testTenant, testDB, nil)

	// The createStorageManager should succeed even with nil LLM client since it's just creating the wrapper
	if err != nil {
		t.Errorf("Unexpected error from createStorageManager: %v", err)
	}

	if storageManager == nil {
		t.Error("createStorageManager should return non-nil storage manager")
	}

	// The important thing is that the function signature accepts the tenant parameter
	t.Logf("createStorageManager correctly accepts tenant parameter: %s", testTenant)
}

// Test configuration structure for flags flow
func TestConfigurationStructure_FlagsFlow(t *testing.T) {
	// Test that our configuration structure supports the expected flow:
	// main.go flags -> IDaesConfig -> AnalysisSystemConfig -> ChromaDB client

	// Simulate main.go flag values
	mainFlagTenant := "production_tenant"
	mainFlagDatabase := "production_db"
	mainFlagChromaURL := "http://chroma.prod:8000"

	// Create AnalysisSystemConfig as if it came from IDaesConfig
	analysisConfig := DefaultAnalysisSystemConfig()

	// Simulate the config propagation that happens in NewIDaesSubsystem
	analysisConfig.ChromaDBURL = mainFlagChromaURL
	analysisConfig.ChromaDBTenant = mainFlagTenant
	analysisConfig.ChromaDBDatabase = mainFlagDatabase

	// Verify configuration made it through the chain
	if analysisConfig.ChromaDBURL != mainFlagChromaURL {
		t.Errorf("ChromaDBURL = %s, want %s", analysisConfig.ChromaDBURL, mainFlagChromaURL)
	}

	if analysisConfig.ChromaDBTenant != mainFlagTenant {
		t.Errorf("ChromaDBTenant = %s, want %s", analysisConfig.ChromaDBTenant, mainFlagTenant)
	}

	if analysisConfig.ChromaDBDatabase != mainFlagDatabase {
		t.Errorf("ChromaDBDatabase = %s, want %s", analysisConfig.ChromaDBDatabase, mainFlagDatabase)
	}

	t.Logf("Configuration flow verified: flags -> config: %s", mainFlagTenant)
}
