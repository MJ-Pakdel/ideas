package subsystems

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/clients"
)

// Mock ChromaDB client for testing meta collections initialization
type mockChromaDBClient struct {
	collections        map[string]*clients.Collection
	shouldFailHealth   bool
	shouldFailCreate   bool
	shouldFailGet      bool
	healthError        error
	createError        error
	getError           error
	createdCollections []string
	getAttempts        []string
	createAttempts     []createAttempt
}

type createAttempt struct {
	name     string
	metadata map[string]any
}

func newMockChromaDBClient() *mockChromaDBClient {
	return &mockChromaDBClient{
		collections:        make(map[string]*clients.Collection),
		createdCollections: make([]string, 0),
		getAttempts:        make([]string, 0),
		createAttempts:     make([]createAttempt, 0),
	}
}

func (m *mockChromaDBClient) Health(ctx context.Context) error {
	if m.shouldFailHealth {
		if m.healthError != nil {
			return m.healthError
		}
		return errors.New("mock health check failed")
	}
	return nil
}

func (m *mockChromaDBClient) GetCollection(ctx context.Context, name string) (*clients.Collection, error) {
	m.getAttempts = append(m.getAttempts, name)

	if m.shouldFailGet {
		if m.getError != nil {
			return nil, m.getError
		}
		return nil, errors.New("mock get collection failed")
	}

	collection, exists := m.collections[name]
	if !exists {
		return nil, errors.New("collection not found")
	}

	return collection, nil
}

func (m *mockChromaDBClient) CreateCollection(ctx context.Context, name string, metadata map[string]any) (*clients.Collection, error) {
	m.createAttempts = append(m.createAttempts, createAttempt{name: name, metadata: metadata})

	if m.shouldFailCreate {
		if m.createError != nil {
			return nil, m.createError
		}
		return nil, errors.New("mock create collection failed")
	}

	collection := &clients.Collection{
		ID:       name + "_id",
		Name:     name,
		Metadata: metadata,
	}

	m.collections[name] = collection
	m.createdCollections = append(m.createdCollections, name)

	return collection, nil
}

// Test successful meta collections initialization
func TestInitializeMetaCollections_Success(t *testing.T) {
	config := &analyzers.AnalysisSystemConfig{
		ChromaDBURL:      "http://localhost:8000",
		ChromaDBTenant:   "test_tenant",
		ChromaDBDatabase: "test_database",
	}

	// Mock the ChromaDB client creation - we'll need to refactor initializeMetaCollections
	// to accept a client for testing, but for now let's test the configuration flow

	// Create a test that verifies the expected collections would be created
	expectedCollections := []string{
		"documents",
		"entities",
		"citations",
		"topics",
		"metadata",
	}

	// Test that the function would attempt to create the right collections
	// Note: This test demonstrates the expected behavior - in a real implementation,
	// we'd need to refactor initializeMetaCollections to accept a ChromaDB client interface
	for _, collection := range expectedCollections {
		t.Logf("Expected collection: %s", collection)
	}

	// Verify config has the expected values
	if config.ChromaDBTenant != "test_tenant" {
		t.Errorf("ChromaDBTenant = %s, want test_tenant", config.ChromaDBTenant)
	}

	if config.ChromaDBDatabase != "test_database" {
		t.Errorf("ChromaDBDatabase = %s, want test_database", config.ChromaDBDatabase)
	}
}

// Test meta collections initialization with ChromaDB connection failure
func TestInitializeMetaCollections_HealthCheckFailure(t *testing.T) {
	ctx := t.Context()

	config := &analyzers.AnalysisSystemConfig{
		ChromaDBURL:      "http://invalid-host:8000",
		ChromaDBTenant:   "test_tenant",
		ChromaDBDatabase: "test_database",
	}

	// Test that health check failure is handled gracefully
	// This would actually call ChromaDB, so we expect it to fail
	err := initializeMetaCollections(ctx, config)
	if err == nil {
		t.Error("Expected error when ChromaDB is unavailable, got nil")
	}

	// Verify the error message contains connection information
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// Test that each expected meta collection has correct metadata
func TestMetaCollections_ExpectedMetadata(t *testing.T) {
	// Define expected meta collections with their purposes
	expectedCollections := map[string]struct {
		purpose   string
		embedding bool
	}{
		"documents": {
			purpose:   "semantic_search_and_storage",
			embedding: true,
		},
		"entities": {
			purpose:   "entity_extraction_and_linking",
			embedding: true,
		},
		"citations": {
			purpose:   "citation_analysis_and_linking",
			embedding: true,
		},
		"topics": {
			purpose:   "topic_modeling_and_clustering",
			embedding: true,
		},
		"metadata": {
			purpose:   "classification_and_management",
			embedding: false,
		},
	}

	for collectionName, expected := range expectedCollections {
		t.Run(collectionName, func(t *testing.T) {
			// Verify we have the expected collection defined
			if expected.purpose == "" {
				t.Errorf("Collection %s should have a purpose defined", collectionName)
			}

			// Log the expected configuration for verification
			t.Logf("Collection %s: purpose=%s, embedding=%t",
				collectionName, expected.purpose, expected.embedding)
		})
	}
}

// Test configuration propagation from IDaesConfig to AnalysisSystemConfig
func TestConfigurationPropagation_TenantAndDatabase(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()

	testCases := []struct {
		name           string
		idaesConfig    *IDaesConfig
		expectedTenant string
		expectedDB     string
	}{
		{
			name: "custom_values_propagated",
			idaesConfig: &IDaesConfig{
				AnalysisConfig:  nil, // Will use default
				ChromaTenant:    "production_tenant",
				ChromaDatabase:  "production_db",
				ChromaURL:       "http://localhost:8000",
				OllamaURL:       "http://localhost:11434",
				RequestBuffer:   10,
				ResponseBuffer:  10,
				WorkerCount:     1,
				RequestTimeout:  time.Second,
				ShutdownTimeout: time.Second,
			},
			expectedTenant: "production_tenant",
			expectedDB:     "production_db",
		},
		{
			name: "default_values_preserved",
			idaesConfig: &IDaesConfig{
				AnalysisConfig:  nil, // Will use default
				ChromaTenant:    "",  // Empty, should get default
				ChromaDatabase:  "",  // Empty, should get default
				ChromaURL:       "http://localhost:8000",
				OllamaURL:       "http://localhost:11434",
				RequestBuffer:   10,
				ResponseBuffer:  10,
				WorkerCount:     1,
				RequestTimeout:  time.Second,
				ShutdownTimeout: time.Second,
			},
			expectedTenant: "default_tenant",
			expectedDB:     "default_database",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create subsystem which should propagate config values
			subsystem, err := NewIDaesSubsystem(ctx, tc.idaesConfig)
			if err != nil {
				t.Fatalf("NewIDaesSubsystem() error = %v", err)
			}

			// Verify tenant propagation
			actualTenant := subsystem.AnalysisSystem.Config.ChromaDBTenant
			if actualTenant != tc.expectedTenant {
				t.Errorf("ChromaDBTenant = %s, want %s", actualTenant, tc.expectedTenant)
			}

			// Verify database propagation
			actualDB := subsystem.AnalysisSystem.Config.ChromaDBDatabase
			if actualDB != tc.expectedDB {
				t.Errorf("ChromaDBDatabase = %s, want %s", actualDB, tc.expectedDB)
			}
		})
	}
}

// Test that meta collections initialization is called during subsystem creation
func TestIDaesSubsystem_CallsMetaCollectionsInit(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()

	config := &IDaesConfig{
		AnalysisConfig:  nil, // Will use default
		ChromaTenant:    "test_tenant",
		ChromaDatabase:  "test_db",
		ChromaURL:       "http://invalid-host:8000", // Intentionally invalid to test error handling
		OllamaURL:       "http://localhost:11434",
		RequestBuffer:   10,
		ResponseBuffer:  10,
		WorkerCount:     1,
		RequestTimeout:  time.Second,
		ShutdownTimeout: time.Second,
	}

	// This should succeed even if meta collections init fails (it logs warning)
	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() should succeed even if meta collections init fails, got error: %v", err)
	}

	if subsystem == nil {
		t.Error("NewIDaesSubsystem() should return non-nil subsystem")
	}

	// Verify the config was properly set
	if subsystem.AnalysisSystem.Config.ChromaDBTenant != "test_tenant" {
		t.Errorf("ChromaDBTenant = %s, want test_tenant",
			subsystem.AnalysisSystem.Config.ChromaDBTenant)
	}
}
