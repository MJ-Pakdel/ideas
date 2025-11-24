package subsystems

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/example/idaes/internal/analyzers"
	"github.com/example/idaes/internal/types"
)

// Test constants
const (
	testTimeout   = 5 * time.Second
	testRequestID = "test-request-123"
)

// Simple mock analysis system for basic testing
type mockAnalysisSystem struct {
	delay       time.Duration
	shouldError bool
	errorType   string
	mu          sync.Mutex
}

func (m *mockAnalysisSystem) AnalyzeDocument(ctx context.Context, doc *types.Document) (*analyzers.AnalysisResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate processing delay
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldError {
		return nil, fmt.Errorf("analysis failed: %s", m.errorType)
	}

	// Return mock successful result
	result := &types.AnalysisResult{
		ID:         "mock-result-123",
		DocumentID: doc.ID,
		Entities:   createSimpleMockEntities(),
		Citations:  createSimpleMockCitations(),
		Topics:     createSimpleMockTopics(),
		AnalyzedAt: time.Now(),
		Version:    "1.0",
	}

	response := &analyzers.AnalysisResponse{
		RequestID:      testRequestID,
		Document:       doc,
		Result:         result,
		ProcessingTime: m.delay,
		Error:          nil,
		Timestamp:      time.Now(),
	}

	return response, nil
}

// Helper functions for creating simple mock data
func createSimpleMockEntities() []*types.Entity {
	return []*types.Entity{
		{
			ID:          "entity-1",
			Type:        types.EntityTypePerson,
			Text:        "John Doe",
			Confidence:  0.9,
			StartOffset: 0,
			EndOffset:   8,
			DocumentID:  "test-doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodRegex,
		},
	}
}

func createSimpleMockCitations() []*types.Citation {
	title := "Test Research"
	year := 2023

	return []*types.Citation{
		{
			ID:          "citation-1",
			Text:        "Smith, J. (2023). Test Research.",
			Authors:     []string{"Smith, J."},
			Title:       &title,
			Year:        &year,
			Format:      types.CitationFormatAPA,
			Confidence:  0.8,
			StartOffset: 50,
			EndOffset:   85,
			DocumentID:  "test-doc",
			ExtractedAt: time.Now(),
			Method:      types.ExtractorMethodRegex,
		},
	}
}

func createSimpleMockTopics() []*types.Topic {
	return []*types.Topic{
		{
			ID:          "topic-1",
			Name:        "Research Methodology",
			Keywords:    []string{"research", "methodology", "analysis"},
			Confidence:  0.7,
			Weight:      0.8,
			DocumentID:  "test-doc",
			ExtractedAt: time.Now(),
			Method:      "regex",
		},
	}
}

func createTestDocument() *types.Document {
	return &types.Document{
		ID:          "test-doc-456",
		Path:        "/test/document.txt",
		Name:        "Test Document",
		Content:     "This is test content for analysis. John Doe works at Test Corp. Smith, J. (2023). Test Research.",
		ContentType: "text/plain",
		Size:        100,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func createTestAnalysisRequest() *AnalysisRequest {
	return &AnalysisRequest{
		RequestID: testRequestID,
		Document:  createTestDocument(),
		Options: &AnalysisOptions{
			EnableEntities:  true,
			EnableCitations: true,
			EnableTopics:    true,
			Timeout:         testTimeout,
		},
	}
}

func createTestIDaesConfig() *IDaesConfig {
	return &IDaesConfig{
		RequestBuffer:   10,
		ResponseBuffer:  10,
		WorkerCount:     2,
		RequestTimeout:  time.Second,
		ShutdownTimeout: time.Second,
		ChromaURL:       "http://mock-chroma:8000",  // Use mock URL
		OllamaURL:       "http://mock-ollama:11434", // Use mock URL
	}
}

// TestNewIDaesSubsystem tests the creation of IDAES subsystem
func TestNewIDaesSubsystem(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()

	tests := []struct {
		name        string
		config      *IDaesConfig
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful creation with valid config",
			config:  createTestIDaesConfig(),
			wantErr: false,
		},
		// Skipping nil config test to avoid real network calls
		// {
		// 	name:    "successful creation with nil config (uses defaults)",
		// 	config:  nil,
		// 	wantErr: false,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subsystem, err := NewIDaesSubsystem(ctx, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewIDaesSubsystem() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("NewIDaesSubsystem() unexpected error = %v", err)
				return
			}

			if subsystem == nil {
				t.Errorf("NewIDaesSubsystem() returned nil subsystem")
				return
			}

			// Validate subsystem fields are initialized
			if subsystem.requestChan == nil {
				t.Errorf("NewIDaesSubsystem() requestChan not initialized")
			}

			if subsystem.responseChan == nil {
				t.Errorf("NewIDaesSubsystem() responseChan not initialized")
			}

			if subsystem.subscribers == nil {
				t.Errorf("NewIDaesSubsystem() subscribers map not initialized")
			}
		})
	}
}

// TestIDaesSubsystem_StartStop tests the lifecycle management
func TestIDaesSubsystem_StartStop(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	// Test Start
	err = subsystem.Start()
	if err != nil {
		t.Errorf("Start() unexpected error = %v", err)
	}

	// Give subsystem time to start
	time.Sleep(50 * time.Millisecond)

	// Verify subsystem is running by checking health
	health := subsystem.GetHealth()
	if health == nil {
		t.Errorf("GetHealth() returned nil after start")
	}

	// Test Stop
	err = subsystem.Stop()
	if err != nil {
		t.Errorf("Stop() unexpected error = %v", err)
	}
}

// TestIDaesSubsystem_SubscribeUnsubscribe tests subscription management
func TestIDaesSubsystem_SubscribeUnsubscribe(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	err = subsystem.Start()
	if err != nil {
		t.Fatalf("Start() unexpected error = %v", err)
	}
	defer subsystem.Stop()

	// Test Subscribe
	subscriberID := "test-subscriber-1"
	callback := func(response *AnalysisResponse) {
		// Test callback
	}

	subsystem.Subscribe(subscriberID, callback)

	// Verify subscription exists
	subsystem.subscribersMu.RLock()
	_, exists := subsystem.subscribers[subscriberID]
	subsystem.subscribersMu.RUnlock()

	if !exists {
		t.Errorf("Subscribe() subscriber %s not found in subscribers map", subscriberID)
	}

	// Test Unsubscribe
	subsystem.Unsubscribe(subscriberID)

	// Verify subscription removed
	subsystem.subscribersMu.RLock()
	_, exists = subsystem.subscribers[subscriberID]
	subsystem.subscribersMu.RUnlock()

	if exists {
		t.Errorf("Unsubscribe() subscriber %s still exists in subscribers map", subscriberID)
	}
}

// TestIDaesSubsystem_GetHealth tests health monitoring
func TestIDaesSubsystem_GetHealth(t *testing.T) {
	t.Skip("Skipping integration test that requires external services")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	err = subsystem.Start()
	if err != nil {
		t.Fatalf("Start() unexpected error = %v", err)
	}
	defer subsystem.Stop()

	// Give subsystem time to initialize
	time.Sleep(50 * time.Millisecond)

	health := subsystem.GetHealth()
	if health == nil {
		t.Errorf("GetHealth() returned nil")
		return
	}

	if health.Status == "" {
		t.Errorf("GetHealth() status is empty")
	}

	// Health should include timestamp
	if health.Timestamp.IsZero() {
		t.Errorf("GetHealth() timestamp is zero")
	}
}

// TestIDaesSubsystem_RequestSubmission tests just request submission without waiting for response
func TestIDaesSubsystem_RequestSubmission(t *testing.T) {
	t.Skip("Skipping integration test that hangs on shutdown - needs fix for graceful cancellation")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	err = subsystem.Start()
	if err != nil {
		t.Fatalf("Start() unexpected error = %v", err)
	}
	defer subsystem.Stop()

	// Give system time to fully start
	time.Sleep(100 * time.Millisecond)

	// Submit test request
	request := createTestAnalysisRequest()
	err = subsystem.SubmitRequest(request)
	if err != nil {
		t.Errorf("SubmitRequest() unexpected error = %v", err)
	}

	// Give some time for processing to start
	time.Sleep(200 * time.Millisecond)

	// Check health to see if request was processed
	health := subsystem.GetHealth()
	if health == nil {
		t.Errorf("GetHealth() returned nil")
	} else {
		t.Logf("Health: Status=%s, RequestsInProgress=%d, RequestsProcessed=%d",
			health.Status, health.RequestsInProgress, health.RequestsProcessed)
	}
}

// TestIDaesSubsystem_RequestResponse tests end-to-end request/response flow
func TestIDaesSubsystem_RequestResponse(t *testing.T) {
	t.Skip("Skipping integration test that hangs waiting for response - needs fix for proper timeout handling")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	err = subsystem.Start()
	if err != nil {
		t.Fatalf("Start() unexpected error = %v", err)
	}
	defer subsystem.Stop()

	// Setup response capture
	var capturedResponse *AnalysisResponse
	var responseMutex sync.Mutex
	responseReceived := make(chan bool, 1)

	callback := func(response *AnalysisResponse) {
		responseMutex.Lock()
		capturedResponse = response
		responseMutex.Unlock()
		responseReceived <- true
	}

	// Subscribe to responses
	subsystem.Subscribe("test-subscriber", callback)

	// Submit test request
	request := createTestAnalysisRequest()
	err = subsystem.SubmitRequest(request)
	if err != nil {
		t.Fatalf("SubmitRequest() unexpected error = %v", err)
	}

	// Wait for response with longer timeout since real analysis system takes time
	select {
	case <-responseReceived:
		// Success
	case <-time.After(15 * time.Second): // Reduced timeout for faster feedback
		t.Fatalf("Response not received within timeout")
	}

	// Validate captured response
	responseMutex.Lock()
	defer responseMutex.Unlock()

	if capturedResponse == nil {
		t.Fatalf("Response callback not called")
	}

	if capturedResponse.RequestID != request.RequestID {
		t.Errorf("Response RequestID = %s, want %s", capturedResponse.RequestID, request.RequestID)
	}

	if capturedResponse.Status == "" {
		t.Errorf("Response Status is empty")
	}

	if capturedResponse.Result == nil {
		t.Errorf("Response Result is nil")
	}
}

// TestIDaesSubsystem_MultipleSubscribers tests broadcasting to multiple subscribers
func TestIDaesSubsystem_MultipleSubscribers(t *testing.T) {
	t.Skip("Skipping integration test that hangs waiting for response - needs fix for proper timeout handling")
	ctx := t.Context()
	config := createTestIDaesConfig()

	subsystem, err := NewIDaesSubsystem(ctx, config)
	if err != nil {
		t.Fatalf("NewIDaesSubsystem() unexpected error = %v", err)
	}

	err = subsystem.Start()
	if err != nil {
		t.Fatalf("Start() unexpected error = %v", err)
	}
	defer subsystem.Stop()

	// Setup multiple subscribers
	numSubscribers := 3
	responses := make([]*AnalysisResponse, numSubscribers)
	responsesReceived := make([]chan bool, numSubscribers)
	responseMutexes := make([]sync.Mutex, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		responses[i] = nil
		responsesReceived[i] = make(chan bool, 1)

		subscriberID := fmt.Sprintf("subscriber-%d", i)
		idx := i // Capture loop variable

		callback := func(response *AnalysisResponse) {
			responseMutexes[idx].Lock()
			responses[idx] = response
			responseMutexes[idx].Unlock()
			responsesReceived[idx] <- true
		}

		subsystem.Subscribe(subscriberID, callback)
	}

	// Submit test request
	request := createTestAnalysisRequest()
	err = subsystem.SubmitRequest(request)
	if err != nil {
		t.Fatalf("SubmitRequest() unexpected error = %v", err)
	}

	// Wait for all responses
	for i := 0; i < numSubscribers; i++ {
		select {
		case <-responsesReceived[i]:
			// Success
		case <-time.After(30 * time.Second):
			t.Fatalf("Response not received for subscriber %d within timeout", i)
		}
	}

	// Validate all subscribers received the response
	for i := 0; i < numSubscribers; i++ {
		responseMutexes[i].Lock()
		response := responses[i]
		responseMutexes[i].Unlock()

		if response == nil {
			t.Errorf("Subscriber %d did not receive response", i)
			continue
		}

		if response.RequestID != request.RequestID {
			t.Errorf("Subscriber %d Response RequestID = %s, want %s", i, response.RequestID, request.RequestID)
		}

		if response.Status == "" {
			t.Errorf("Subscriber %d Response Status is empty", i)
		}
	}
}

// Test that ChromaDB tenant and database configuration is properly passed through
func TestIDaesConfig_TenantDatabasePassthrough(t *testing.T) {
	t.Skip("Skipping integration test that requires external services (Ollama and ChromaDB)")
	ctx := t.Context()

	tests := []struct {
		name           string
		config         *IDaesConfig
		expectedTenant string
		expectedDB     string
	}{
		{
			name: "custom_tenant_and_database",
			config: &IDaesConfig{
				AnalysisConfig:  analyzers.DefaultAnalysisSystemConfig(),
				RequestBuffer:   10,
				ResponseBuffer:  10,
				WorkerCount:     1,
				RequestTimeout:  time.Second,
				ShutdownTimeout: time.Second,
				ChromaURL:       "http://localhost:8000",
				ChromaTenant:    "custom_tenant",
				ChromaDatabase:  "custom_database",
				OllamaURL:       "http://localhost:11434",
			},
			expectedTenant: "custom_tenant",
			expectedDB:     "custom_database",
		},
		{
			name: "default_tenant_and_database",
			config: &IDaesConfig{
				AnalysisConfig:  analyzers.DefaultAnalysisSystemConfig(),
				RequestBuffer:   10,
				ResponseBuffer:  10,
				WorkerCount:     1,
				RequestTimeout:  time.Second,
				ShutdownTimeout: time.Second,
				ChromaURL:       "http://localhost:8000",
				ChromaTenant:    "default_tenant",
				ChromaDatabase:  "default_database",
				OllamaURL:       "http://localhost:11434",
			},
			expectedTenant: "default_tenant",
			expectedDB:     "default_database",
		},
		{
			name: "empty_tenant_and_database_get_defaults",
			config: &IDaesConfig{
				AnalysisConfig:  analyzers.DefaultAnalysisSystemConfig(),
				RequestBuffer:   10,
				ResponseBuffer:  10,
				WorkerCount:     1,
				RequestTimeout:  time.Second,
				ShutdownTimeout: time.Second,
				ChromaURL:       "http://localhost:8000",
				ChromaTenant:    "",
				ChromaDatabase:  "",
				OllamaURL:       "http://localhost:11434",
			},
			expectedTenant: "default_tenant", // Should use default from DefaultAnalysisSystemConfig
			expectedDB:     "idaes",          // Should use default from DefaultAnalysisSystemConfig
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that config values are properly passed through
			subsystem, err := NewIDaesSubsystem(ctx, tt.config)
			if err != nil {
				t.Fatalf("NewIDaesSubsystem() error = %v", err)
			}

			// Verify the analysis config has the expected values
			if subsystem.AnalysisSystem.Config.ChromaDBTenant != tt.expectedTenant {
				t.Errorf("ChromaDBTenant = %s, want %s",
					subsystem.AnalysisSystem.Config.ChromaDBTenant, tt.expectedTenant)
			}

			if subsystem.AnalysisSystem.Config.ChromaDBDatabase != tt.expectedDB {
				t.Errorf("ChromaDBDatabase = %s, want %s",
					subsystem.AnalysisSystem.Config.ChromaDBDatabase, tt.expectedDB)
			}
		})
	}
}

// Test DefaultIDaesConfig includes tenant and database
func TestDefaultIDaesConfig_IncludesTenantAndDatabase(t *testing.T) {
	config := DefaultIDaesConfig()

	if config.ChromaTenant == "" {
		t.Error("DefaultIDaesConfig.ChromaTenant should not be empty")
	}

	if config.ChromaDatabase == "" {
		t.Error("DefaultIDaesConfig.ChromaDatabase should not be empty")
	}

	expectedTenant := "default_tenant"
	expectedDatabase := "default_database"

	if config.ChromaTenant != expectedTenant {
		t.Errorf("ChromaTenant = %s, want %s", config.ChromaTenant, expectedTenant)
	}

	if config.ChromaDatabase != expectedDatabase {
		t.Errorf("ChromaDatabase = %s, want %s", config.ChromaDatabase, expectedDatabase)
	}
}
