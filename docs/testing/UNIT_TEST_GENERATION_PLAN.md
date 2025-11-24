# Unit Test Generation Plan for IDAES Project

## Overview

This document outlines a comprehensive plan to generate unit tests for all externalized (capitalized) functions and methods in the IDAES (Intelligent Document Analysis Engine System) project. The goal is to achieve comprehensive test coverage for all public APIs and ensure code reliability.

## Current Test Coverage Status

- **Overall Project Coverage**: 6.6%
- **Best Covered Package**: `internal/types` (40.0%)
- **Moderate Coverage**: `internal/analyzers` (12.1% - with some failing tests)
- **Zero Coverage**: All other packages

## Testing Strategy

### Phase 1: Core Types and Utilities (High Priority)
**Target: Achieve 90%+ coverage for foundational components**

#### 1.1 internal/types Package
- **Status**: Currently 40% covered
- **Remaining Functions to Test**:
  - `NewAnalysisResult(documentID string) *AnalysisResult`
  - `NewError(code, message, details string) *ErrorInfo`
  - `ParseUUID(s string) (uuid.UUID, error)`

**Test Plan**:
- Create comprehensive tests for analysis result creation with various scenarios
- Test error creation with different error types and validation
- Test UUID parsing with valid/invalid formats and edge cases

#### 1.2 internal/config Package
- **Status**: 0% coverage
- **Functions to Test**:
  - `DefaultConfig() *Config`
  - `(c *Config) Validate() error`
  - `(c *EntityExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.EntityExtractionConfig`
  - `(c *CitationExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.CitationExtractionConfig`
  - `(c *TopicExtractionConfig) ToInterfaceConfig(llmConfig *interfaces.LLMConfig) *interfaces.TopicExtractionConfig`
  - `(c *LLMConfig) ToInterfaceConfig() *interfaces.LLMConfig`

**Test Plan**:
- Test default configuration creation and validation
- Test configuration validation with valid/invalid configurations
- Test interface config conversions with various input scenarios
- Mock LLM config dependencies for conversion tests

### Phase 2: Client Layer (High Priority)
**Target: Achieve 85%+ coverage for external integrations**

#### 2.1 internal/clients/llm.go Package
- **Status**: 0% coverage
- **Functions to Test**:
  - `NewOllamaClient(baseURL, chatModel, embeddingModel string, timeout time.Duration, maxRetries int) *OllamaClient`
  - `(c *OllamaClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error)`
  - `(c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error)`
  - `(c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)`
  - `(c *OllamaClient) Health(ctx context.Context) error`
  - `(c *OllamaClient) GetModelInfo() interfaces.ModelInfo`
  - `(c *OllamaClient) Close() error`
  - `NewDOIClient(baseURL, userAgent string, timeout, rateLimit time.Duration) *DOIClient`
  - `(d *DOIClient) ResolveDOI(ctx context.Context, doi string) (*DOIMetadata, error)`
  - `(a Author) FullName() string`

**Test Plan**:
- Mock HTTP server for testing Ollama client interactions
- Test client creation with various configurations
- Test completion with different prompts and options
- Test embedding with single text and batch operations
- Test health checks and error handling
- Test DOI resolution with valid/invalid DOIs
- Test rate limiting and timeout behaviors

#### 2.2 internal/clients/chromadb.go Package
- **Status**: 0% coverage
- **Functions to Test**:
  - `NewChromaDBClient(baseURL string, timeout time.Duration, maxRetries int, tenant, database string) *ChromaDBClient`
  - `(c *ChromaDBClient) Health(ctx context.Context) error`
  - `(c *ChromaDBClient) Healthcheck(ctx context.Context) error` 
  - `(c *ChromaDBClient) Version(ctx context.Context) (string, error)`
  - `(c *ChromaDBClient) CreateCollection(ctx context.Context, name string, metadata map[string]interface{}) (*Collection, error)`
  - `(c *ChromaDBClient) GetCollection(ctx context.Context, collectionID string) (*Collection, error)`
  - `(c *ChromaDBClient) ListCollections(ctx context.Context) ([]Collection, error)`
  - `(c *ChromaDBClient) DeleteCollection(ctx context.Context, collectionID string) error`
  - `(c *ChromaDBClient) AddDocuments(ctx context.Context, collectionID string, request AddDocumentsRequest) error`
  - `(c *ChromaDBClient) UpdateDocuments(ctx context.Context, collectionID string, request UpdateDocumentsRequest) error`
  - `(c *ChromaDBClient) QueryDocuments(ctx context.Context, collectionID string, request QueryRequest) (*QueryResponse, error)`
  - `(c *ChromaDBClient) DeleteDocuments(ctx context.Context, collectionID string, ids []string) error`

**Test Plan**:
- Mock ChromaDB HTTP v2 API for testing (updated from v1)
- Test client creation with tenant/database configuration
- Test collection CRUD operations using collection IDs
- Test document operations (add, update, query, delete)
- Test error handling and retry logic
- Test timeout and context cancellation
- Test new v2 endpoints: healthcheck, version

### Phase 3: Storage Layer (High Priority)
**Target: Achieve 80%+ coverage for data persistence**

#### 3.1 internal/storage/chromadb.go Package
- **Status**: 0% coverage
- **Functions to Test**:
  - `NewChromaStorageManager(...) *ChromaStorageManager`
  - `(s *ChromaStorageManager) Initialize(ctx context.Context) error`
  - `(s *ChromaStorageManager) StoreDocument(ctx context.Context, doc *types.Document) error`
  - `(s *ChromaStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, doc *types.Document) error`
  - `(s *ChromaStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error)`
  - `(s *ChromaStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error)`
  - `(s *ChromaStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error)`
  - `(s *ChromaStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error)`
  - `(s *ChromaStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error`
  - `(s *ChromaStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error`
  - `(s *ChromaStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error`
  - `(s *ChromaStorageManager) DeleteDocument(ctx context.Context, id string) error`
  - `(s *ChromaStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error`
  - `(s *ChromaStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error)`
  - `(s *ChromaStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error)`
  - `(s *ChromaStorageManager) Close() error`
  - `(s *ChromaStorageManager) Health(ctx context.Context) error`

**Test Plan**:
- Mock ChromaDB client for storage manager testing
- Test storage manager initialization and configuration
- Test document storage and retrieval operations
- Test entity, citation, and topic storage
- Test search functionality with various queries
- Test collection management operations
- Test error handling and cleanup
- Test health checks and statistics

### Phase 4: Extraction Layer (Medium Priority)
**Target: Achieve 75%+ coverage for document processing**

#### 4.1 Extractor Factory (internal/extractors/factory.go)
- **Functions to Test**:
  - `NewExtractorFactory(ctx context.Context) *ExtractorFactory`
  - `(f *ExtractorFactory) CreateEntityExtractor(...) interfaces.EntityExtractor`
  - `(f *ExtractorFactory) CreateCitationExtractor(...) interfaces.CitationExtractor`
  - `(f *ExtractorFactory) CreateTopicExtractor(...) interfaces.TopicExtractor`
  - All validation and configuration methods

#### 4.2 Entity Extractors
- **Regex Entity Extractor (internal/extractors/entity_regex.go)**:
  - `NewRegexEntityExtractor(config *interfaces.EntityExtractionConfig) *RegexEntityExtractor`
  - All interface methods (Extract, Validate, ExtractEntities, etc.)

- **LLM Entity Extractor (internal/extractors/entity_llm.go)**:
  - `NewLLMEntityExtractor(config *interfaces.EntityExtractionConfig, llmClient interfaces.LLMClient) *LLMEntityExtractor`
  - All interface methods

#### 4.3 Citation and Topic Extractors
- Similar comprehensive testing for citation and topic extractors

**Test Plan**:
- Mock LLM clients for extractor testing
- Test extractor creation with various configurations
- Test extraction with different document types and content
- Test validation logic and error handling
- Test confidence scoring and filtering
- Test performance with large documents

### Phase 5: Analysis Engine and Extraction Layer (Medium Priority) ✅ COMPLETED
**Target: Achieve 70%+ coverage for core analysis logic**
**Status: ✅ Achieved 67.9% coverage for extraction layer + Major architecture refactoring**

**COMPLETED WORK:**
- ✅ **Extraction Layer Testing**: Comprehensive testing of all extractors (67.9% coverage)
  - Factory pattern testing for entity, citation, and topic extractors
  - RegexEntityExtractor and LLMEntityExtractor testing with realistic test data
  - Citation and topic extraction testing with performance validation
  - Mock LLM client infrastructure for testing LLM-dependent components
- ✅ **Architecture Refactoring**: Complete separation of IDAES and Web subsystems
  - Created new `internal/subsystems/` package with channel-based communication
  - IDAES subsystem as standalone analysis engine with subscriber pattern
  - Web subsystem subscribing to IDAES via callback functions
  - Command-line flag configuration for independent subsystem startup

**PARTIALLY COMPLETED:**
- ⚠️ Analysis Engine core components (12.1% coverage, some failing tests)

#### 5.1 Factory and System Management
- **Functions to Test**:
  - `NewAnalysisSystemFactory() *AnalysisSystemFactory`
  - `(f *AnalysisSystemFactory) CreateSystem(ctx context.Context, config *AnalysisSystemConfig) (*AnalysisSystem, error)`
  - `DefaultAnalysisSystemConfig() *AnalysisSystemConfig`
  - `(s *AnalysisSystem) Start(ctx context.Context) error`
  - `(s *AnalysisSystem) Stop(ctx context.Context) error`
  - `(s *AnalysisSystem) AnalyzeDocument(ctx context.Context, document *types.Document) (*AnalysisResponse, error)`
  - `(s *AnalysisSystem) GetSystemStatus() map[string]interface{}`
  - `(s *AnalysisSystem) GetSystemContext() context.Context`
  - `(s *AnalysisSystem) IsShutdownInitiated() bool`

#### 5.2 Orchestrator and Pipeline Management
- **Functions to Test**:
  - `NewAnalysisOrchestrator(...) *AnalysisOrchestrator`
  - `DefaultOrchestratorConfig() *OrchestratorConfig`
  - `(o *AnalysisOrchestrator) AddPipeline(...) error`
  - `(o *AnalysisOrchestrator) Start(ctx context.Context) error`
  - `(o *AnalysisOrchestrator) Stop(ctx context.Context) error`
  - `(o *AnalysisOrchestrator) SubmitRequest(...) error`
  - `(o *AnalysisOrchestrator) GetResponse(...) (*AnalysisResponse, error)`
  - `(o *AnalysisOrchestrator) GetMetrics() *OrchestratorMetrics`
  - `(o *AnalysisOrchestrator) GetWorkerStatus() []*WorkerStatus`

#### 5.3 Concurrency Management
- **Functions to Test**:
  - `NewCancellationCoordinator(...) *CancellationCoordinator`
  - All cancellation coordinator methods
  - `NewTimeoutManager(...) *TimeoutManager`
  - All timeout manager methods
  - `NewSystemCancellationManager(...) *SystemCancellationManager`
  - All system cancellation manager methods

**Test Plan**:
- Test system lifecycle (start, stop, graceful shutdown)
- Test document analysis end-to-end with mocked dependencies
- Test orchestrator with multiple pipelines and workers
- Test concurrency coordination and timeout handling
- Test error scenarios and recovery
- Fix existing failing integration tests

### Phase 6: Subsystems and Web Layer (Medium Priority)
**Target: Achieve 75%+ coverage for new subsystem architecture and HTTP API**

#### 6.1 IDAES Subsystem (internal/subsystems/idaes_subsystem.go)
- **Functions to Test**:
  - `NewIDaesSubsystem(ctx context.Context, config *IDaesConfig) (*IDaesSubsystem, error)`
  - `(i *IDaesSubsystem) Start() error`
  - `(i *IDaesSubsystem) Stop() error`
  - `(i *IDaesSubsystem) SubmitRequest(request *AnalysisRequest) error`
  - `(i *IDaesSubsystem) Subscribe(subscriberID string, callback ResponseCallback)`
  - `(i *IDaesSubsystem) Unsubscribe(subscriberID string)`
  - `(i *IDaesSubsystem) GetHealth() *HealthStatus`
  - `DefaultIDaesConfig() *IDaesConfig`
  - Internal methods: `requestProcessor()`, `processRequest()`, `responseBroadcaster()`, `healthMonitor()`

#### 6.2 Web Subsystem (internal/subsystems/web_subsystem.go)
- **Functions to Test**:
  - `NewWebSubsystem(ctx context.Context, config *WebSubsystemConfig, idaesSubsystem *IDaesSubsystem) (*WebSubsystem, error)`
  - `(w *WebSubsystem) Start() error`
  - `(w *WebSubsystem) Stop() error`
  - `(w *WebSubsystem) GetHealth() map[string]interface{}`
  - `DefaultWebSubsystemConfig() *WebSubsystemConfig`

#### 6.3 IDAES Adapter (internal/subsystems/idaes_adapter.go)
- **Functions to Test**:
  - `NewIDaesAdapter(idaesClient *IDaesClient, timeout time.Duration) *IDaesAdapter`
  - `(a *IDaesAdapter) AnalyzeDocument(ctx context.Context, document *types.Document) (*analyzers.AnalysisResponse, error)`
  - `(a *IDaesAdapter) Health() *HealthStatus`
  - `(a *IDaesAdapter) Close() error`
  - `NewAnalysisSystemAdapter(analysisSystem *analyzers.AnalysisSystem) *AnalysisSystemAdapter`
  - `convertExtractorUsed(input map[string]interface{}) map[string]types.ExtractorMethod`

#### 6.4 IDAES Client (internal/subsystems/web_subsystem.go - IDaesClient)
- **Functions to Test**:
  - `(i *IDaesClient) SubmitAnalysisRequest(request *AnalysisRequest) error`
  - `(i *IDaesClient) GetResponse(timeout time.Duration) (*AnalysisResponse, error)`
  - `(i *IDaesClient) GetResponseAsync() (*AnalysisResponse, bool)`
  - `(i *IDaesClient) GetHealth() *HealthStatus`
  - `(i *IDaesClient) handleResponse(response *AnalysisResponse)`

#### 6.5 Modified Web Server (internal/web/web.go)
- **Updated Functions to Test**:
  - `NewServer(ctx context.Context, cfg *config.ServerConfig) (*Server, error)` (legacy mode)
  - `NewServerWithAnalysisService(ctx context.Context, cfg *config.ServerConfig, analysisService AnalysisService) (*Server, error)` (new mode)
  - Modified HTTP handlers that now use AnalysisService interface
  - AnalysisService interface compliance

#### 6.6 WebSocket Management (unchanged from original plan)
- **Functions to Test**:
  - `(m *WebSocketConnectionManager) RegisterConnection(conn *websocket.Conn)`
  - `(m *WebSocketConnectionManager) UnregisterConnection(conn *websocket.Conn)`
  - `(m *WebSocketConnectionManager) Broadcast(message interface{})`
  - `(m *WebSocketConnectionManager) GetConnectionCount() int`
  - `(m *WebSocketConnectionManager) Stop()`

**Test Plan**:
- **Subsystem Integration Testing**: Test IDAES → Web subsystem communication via channels
- **Channel Communication**: Test request/response flow through channels
- **Subscriber Pattern**: Test callback registration and push notifications
- **Health Monitoring**: Test health status aggregation across subsystems
- **Error Propagation**: Test error handling through channel communication
- **Adapter Pattern**: Test both IDAES adapter and AnalysisSystem adapter
- **Graceful Shutdown**: Test coordinated shutdown of both subsystems
- **HTTP Handlers**: Test web handlers with new AnalysisService interface
- **WebSocket Management**: Test real-time communication (unchanged)
- **Configuration**: Test command-line flag configuration and subsystem startup
- **Mock Testing**: Comprehensive mocking of IDAES subsystem for web testing

**Priority Test Scenarios**:
1. **Integration Tests**: End-to-end request flow from web → IDAES → response
2. **Concurrent Subscription**: Multiple subscribers to same IDAES subsystem
3. **Error Scenarios**: Network failures, timeouts, malformed requests
4. **Performance**: Channel buffer management, response latency
5. **Lifecycle Management**: Startup order, shutdown coordination

### Phase 7: Pipeline and Metrics (Lower Priority)
**Target: Achieve 60%+ coverage for pipeline components**

#### 7.1 Pipeline Components
- Test all pipeline stage analyzers
- Test metrics collection and broadcasting
- Test channel-based orchestration

#### 7.2 Worker Management
- Test worker supervision and health monitoring
- Test request distribution and load balancing

## Test Implementation Guidelines

### Code Requirements (Based on User Feedback)
1. **No third-party test libraries** - Use only Go standard `testing` package
2. **Table-driven tests** - All tests should use table-driven patterns
3. **Context management**:
   - Use `ctx := t.Context()` for test functions
   - Use `ctx := b.Context()` for benchmark functions
   - **Never use `context.Background()`** in tests
   - Declare root context **once at the beginning** of each test function
4. **External dependency mocking**:
   - Mock Ollama and ChromaDB services
   - Create manual interface implementations for mocking
   - Use `httptest` for HTTP service mocking

### Testing Standards
1. **Use Go standard testing library only** - No third-party test libraries like testify
2. **Follow table-driven test patterns** for multiple test cases
3. **Test both success and failure scenarios** for each function
4. **Use dependency injection** for easier mocking
5. **Implement setup and teardown** for complex test scenarios
6. **Use `t.Context()` or `b.Context()`** for test contexts, not `context.Background()`
7. **Declare root context once** at the beginning of each test function

### Mock Strategy
1. **Create interface mocks manually** for all external dependencies:
   - LLM clients (Ollama)
   - ChromaDB clients
   - Storage managers
   - HTTP clients
2. **Use context for timeout testing** with `t.Context()` or `b.Context()`
3. **Mock time operations** for deterministic testing
4. **Mock external services** (Ollama, ChromaDB) using httptest or dedicated mock servers

### Test Organization
1. **One test file per source file** (e.g., `config_test.go` for `config.go`)
2. **Group related tests** in subtests using `t.Run()`
3. **Use descriptive test names** that explain the scenario
4. **Include benchmarks** for performance-critical functions
5. **Use table-driven tests** with comprehensive test cases
6. **Declare context once** at the beginning: `ctx := t.Context()` or `ctx := b.Context()`

### Coverage Goals
- **Phase 1**: 90%+ coverage
- **Phase 2**: 85%+ coverage  
- **Phase 3**: 80%+ coverage
- **Phase 4**: 75%+ coverage
- **Phase 5**: 70%+ coverage (✅ Achieved 67.9% for extractors)
- **Phase 6**: 75%+ coverage (Updated for new subsystems architecture)
- **Phase 7**: 60%+ coverage

### Test Template Examples

#### Table-Driven Test Template
```go
func TestFunctionName(t *testing.T) {
    ctx := t.Context() // Declare context once at beginning
    
    tests := []struct {
        name     string
        input    InputType
        want     OutputType
        wantErr  bool
    }{
        {
            name:    "valid input",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   invalidInput,
            want:    OutputType{},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(ctx, tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("FunctionName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Benchmark Template
```go
func BenchmarkFunctionName(b *testing.B) {
    ctx := b.Context() // Use b.Context() for benchmarks
    
    // Setup code here
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = FunctionName(ctx, input)
    }
}
```

#### Mock Interface Template
```go
// Mock implementation without external libraries
type mockLLMClient struct {
    completeFunc    func(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error)
    embedFunc       func(ctx context.Context, text string) ([]float64, error)
    healthFunc      func(ctx context.Context) error
}

func (m *mockLLMClient) Complete(ctx context.Context, prompt string, options *interfaces.LLMOptions) (string, error) {
    if m.completeFunc != nil {
        return m.completeFunc(ctx, prompt, options)
    }
    return "", nil
}

func (m *mockLLMClient) Embed(ctx context.Context, text string) ([]float64, error) {
    if m.embedFunc != nil {
        return m.embedFunc(ctx, text)
    }
    return []float64{0.1, 0.2, 0.3}, nil
}

func (m *mockLLMClient) Health(ctx context.Context) error {
    if m.healthFunc != nil {
        return m.healthFunc(ctx)
    }
    return nil
}
```

## Implementation Timeline

### Week 1-2: Foundation (Phase 1)
- Complete `internal/types` package testing
- Complete `internal/config` package testing
- Set up testing infrastructure and CI integration

### Week 3-4: Client Layer (Phase 2)
- Complete LLM client testing
- Complete ChromaDB client testing
- Implement comprehensive mocking framework

### Week 5-6: Storage Layer (Phase 3)
- Complete storage manager testing
- Integration tests with mocked clients

### Week 7-8: Extraction Layer (Phase 4)
- Complete all extractor testing
- Performance benchmarks for extraction

### Week 9-10: Analysis Engine (Phase 5)
- Complete factory and system testing
- Fix existing failing tests
- Concurrency and timeout testing

### Week 11-12: Subsystems and Web Layer (Phase 6)
- Complete IDAES subsystem testing (channel communication, health monitoring)
- Complete Web subsystem testing (subscription patterns, adapter integration)
- Test new architecture integration (IDAES ↔ Web communication)
- Complete modified HTTP handler testing with AnalysisService interface
- WebSocket functionality testing (unchanged)
- End-to-end subsystem communication testing

### Week 13-14: Pipeline and Cleanup (Phase 7)
- Complete remaining components
- Integration testing
- Documentation and cleanup

## Success Metrics

1. **Overall test coverage > 75%**
2. **All critical path functions covered**
3. **Zero failing tests in CI**
4. **Performance benchmarks established**
5. **Documentation for complex test scenarios**

## Notes for Implementation

### Key Requirements (User Specified)
- **No third-party test libraries** - Use only Go standard `testing` package
- **Table-driven tests mandatory** for all test functions
- **Context management**: Use `ctx := t.Context()` or `ctx := b.Context()`, never `context.Background()`
- **Single context declaration** at the beginning of each test function
- **Mock external dependencies** (Ollama, ChromaDB) manually

### Implementation Best Practices
- **Mock external services** using `httptest` or dedicated mock servers
- **Use build tags** to separate integration tests from unit tests
- **Implement test helpers** for common setup/teardown operations
- **Consider property-based testing** for complex data structures using Go standard library
- **Add race detection** to concurrent code tests using `-race` flag
- **Include examples** in test code for documentation purposes
- **Manual assertion functions** instead of third-party assertion libraries

### File Organization
- Each source file gets a corresponding `_test.go` file
- Use meaningful test function names that describe the scenario
- Group related tests using `t.Run()` subtests
- Include benchmarks for performance-critical functions
- Separate unit tests from integration tests using build tags