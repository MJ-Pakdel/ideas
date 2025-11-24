# IDAES Analysis Pipeline

The IDAES (Intelligent Document Analysis and Extraction System) analysis pipeline provides a comprehensive, modular system for document analysis with adaptive strategy selection.

## Architecture Overview

The analysis pipeline consists of several key components:

### 1. Analysis Pipeline (`internal/analyzers/pipeline.go`)
- **Purpose**: Orchestrates document analysis using multiple extraction strategies
- **Features**: 
  - Concurrent processing of entities, citations, and topics
  - Adaptive strategy selection with fallback mechanisms
  - Configurable timeouts, retries, and quality thresholds
  - Comprehensive metrics and performance tracking

### 2. Analysis Orchestrator (`internal/analyzers/orchestrator.go`)
- **Purpose**: Manages multiple analysis pipelines with queue-based processing
- **Features**:
  - Worker pool management with configurable limits
  - Priority-based request handling
  - Load balancing across multiple pipelines
  - Health monitoring and automatic recovery

### 3. Analysis System Factory (`internal/analyzers/factory.go`)
- **Purpose**: Creates and configures complete analysis systems
- **Features**:
  - One-stop factory for system initialization
  - Default configurations with customization options
  - Automatic component wiring and dependency injection

## Core Features

### Strategy Pattern Implementation
- **Entity Extraction**: LLM-based intelligent analysis
- **Citation Extraction**: Regex-based pattern matching with format detection
- **Topic Extraction**: Statistical and LLM-based approaches
- **Runtime Selection**: Choose optimal strategy based on document type and requirements

### Adaptive Processing
- **Fallback Mechanisms**: Automatic fallback to alternative methods on failure
- **Quality Thresholds**: Configurable confidence scores and validation requirements
- **Performance Optimization**: Concurrent processing with timeout management

### Comprehensive Monitoring
- **Pipeline Metrics**: Processing times, success rates, extractor usage
- **Orchestrator Metrics**: Queue depth, worker utilization, throughput
- **System Health**: Automated health checks for all components

## Configuration

### Analysis Configuration
```go
config := &AnalysisConfig{
    PreferredEntityMethod:     types.ExtractorMethodLLM,
    PreferredCitationMethod:   types.ExtractorMethodRegex,
    PreferredTopicMethod:      types.ExtractorMethodLLM,
    MaxConcurrentAnalyses:     10,
    AnalysisTimeout:           5 * time.Minute,
    MinConfidenceScore:        0.7,
    RequireValidation:         true,
    FallbackOnFailure:         true,
    EnableEntityExtraction:    true,
    EnableCitationExtraction:  true,
    EnableTopicExtraction:     true,
    StoreResults:              true,
    GenerateEmbeddings:        true,
}
```

### Orchestrator Configuration
```go
config := &OrchestratorConfig{
    MaxWorkers:          10,
    QueueSize:           1000,
    ProcessingTimeout:   10 * time.Minute,
    MaxRetries:          3,
    EnablePriority:      true,
    LoadBalancing:       "least_busy",
    MetricsInterval:     30 * time.Second,
}
```

## Usage Examples

### Basic Document Analysis
```go
// Create analysis system
factory := analyzers.NewAnalysisSystemFactory(logger)
system, err := factory.CreateSystem(ctx, nil) // Uses defaults
if err != nil {
    return err
}

// Start the system
if err := system.Start(ctx); err != nil {
    return err
}

// Analyze a document
document := types.NewDocument("/path/to/doc", "Title", content, "text/plain")
response, err := system.AnalyzeDocument(ctx, document)
if err != nil {
    return err
}

// Access results
entities := response.Result.Entities
citations := response.Result.Citations
topics := response.Result.Topics
```

### Advanced Pipeline Configuration
```go
// Custom configuration
config := analyzers.DefaultAnalysisSystemConfig()
config.PipelineConfig.PreferredEntityMethod = types.ExtractorMethodLLM
config.PipelineConfig.MinConfidenceScore = 0.8
config.OrchestratorConfig.MaxWorkers = 20

// Create system with custom config
system, err := factory.CreateSystem(ctx, config)
```

### Queue-Based Processing
```go
// Submit multiple requests
for _, doc := range documents {
    request := &analyzers.AnalysisRequest{
        Document:  doc,
        RequestID: fmt.Sprintf("req_%s", doc.ID),
        Priority:  1,
    }
    
    if err := system.Orchestrator.SubmitRequest(request); err != nil {
        log.Printf("Failed to submit request: %v", err)
    }
}

// Process responses
for i := 0; i < len(documents); i++ {
    response, ok := system.Orchestrator.GetResponse()
    if !ok {
        break
    }
    
    // Handle response
    processAnalysisResponse(response)
}
```

## Monitoring and Metrics

### Pipeline Metrics
```go
metrics := system.Pipeline.GetMetrics()
fmt.Printf("Total analyses: %d\n", metrics.TotalAnalyses)
fmt.Printf("Success rate: %.2f%%\n", 
    float64(metrics.SuccessfulAnalyses)/float64(metrics.TotalAnalyses)*100)
fmt.Printf("Average processing time: %v\n", metrics.AverageProcessingTime)
```

### System Status
```go
status := system.GetSystemStatus()
fmt.Printf("System status: %+v\n", status)
```

### Worker Monitoring
```go
workers := system.Orchestrator.GetWorkerStatus()
for _, worker := range workers {
    fmt.Printf("Worker %d: Active=%t, Handled=%d\n", 
        worker.ID, worker.IsActive, worker.RequestsHandled)
}
```

## Error Handling

The system provides comprehensive error handling at multiple levels:

1. **Request Level**: Individual analysis failures don't affect other requests
2. **Extractor Level**: Fallback mechanisms for failed extractors
3. **Pipeline Level**: Timeout and retry management
4. **System Level**: Health monitoring and automatic recovery

### Error Recovery
```go
// Automatic fallback configuration
config.FallbackOnFailure = true

// Custom error handling
response, err := system.AnalyzeDocument(ctx, document)
if err != nil {
    // System-level error
    handleSystemError(err)
} else if response.Error != nil {
    // Analysis-level error
    handleAnalysisError(response.Error)
} else {
    // Success
    processResults(response.Result)
}
```

## Performance Optimization

### Concurrent Processing
- Entity, citation, and topic extraction run concurrently
- Configurable worker pools for high-throughput processing
- Load balancing across multiple analysis pipelines

### Caching
- Built-in caching support for expensive operations
- Configurable cache TTL and size limits
- LRU eviction policies

### Resource Management
- Automatic cleanup and resource management
- Graceful shutdown with pending request completion
- Memory-efficient processing for large documents

## Integration Points

### Storage Integration
- Automatic result storage to ChromaDB
- Support for multiple collection types
- Embedding generation and storage

### LLM Integration
- Ollama client for local LLM processing
- Embedding generation for semantic search
- Configurable model selection and parameters

### Web Interface
- REST API endpoints for document upload and analysis
- Real-time status monitoring
- Historical analysis results

## Testing

The analysis pipeline includes comprehensive testing:

```bash
# Run all tests
go test ./internal/analyzers/...

# Run with coverage
go test -cover ./internal/analyzers/...

# Run integration tests
go test ./integration/...
```

## Contributing

When extending the analysis pipeline:

1. **Add New Extractors**: Implement the extractor interfaces in `internal/extractors/`
2. **Custom Strategies**: Add new strategy types to `internal/types/common.go`
3. **Pipeline Extensions**: Extend the pipeline configuration for new features
4. **Testing**: Add comprehensive tests for new functionality

See the existing extractor implementations for patterns and best practices.