# IDAES System Architecture Documentation

## Overview

The Intelligent Document Analysis and Entity Storage (IDAES) system is designed as a microservices architecture for processing documents and extracting entities at scale. This technical documentation outlines the system's core components, APIs, and deployment strategies.

## Core Components

### 1. Document Processing Service

The DocumentProcessor handles incoming documents through a multi-stage pipeline:

```go
type DocumentProcessor struct {
    extractor    EntityExtractor
    storage      StorageManager
    validator    DocumentValidator
    metrics      MetricsCollector
}
```

Key responsibilities:
- Document validation and preprocessing
- Entity extraction coordination
- Result aggregation and storage
- Performance monitoring and logging

### 2. Entity Extraction Engine

The ExtractorEngine provides multiple extraction strategies:

```go
type ExtractorEngine interface {
    ExtractEntities(doc Document) ([]Entity, error)
    GetSupportedTypes() []EntityType
    Configure(config ExtractorConfig) error
}
```

Supported entity types:
- PERSON: Individual names and titles
- ORGANIZATION: Companies, institutions, groups
- LOCATION: Geographic locations and addresses
- DATE: Temporal references and timestamps
- MONEY: Currency amounts and financial data
- PHONE: Telephone and contact numbers
- EMAIL: Electronic mail addresses
- URL: Web addresses and URIs

### 3. Storage Management Layer

The StorageManager handles data persistence:

```go
type StorageManager interface {
    StoreDocument(doc Document) error
    StoreEntities(entities []Entity) error
    QueryEntities(filter EntityFilter) ([]Entity, error)
    GetDocumentByID(id string) (Document, error)
}
```

Storage backends:
- ChromaDB: Vector storage for semantic search
- PostgreSQL: Relational data and metadata
- Redis: Caching and session management
- S3: Document artifact storage

### 4. API Gateway

The APIGateway exposes HTTP endpoints:

```go
type APIGateway struct {
    router       *gin.Engine
    processor    *DocumentProcessor
    auth         AuthMiddleware
    rateLimit    RateLimiter
}
```

Main endpoints:
- POST /api/v1/documents - Upload document for processing
- GET /api/v1/documents/{id} - Retrieve document metadata
- GET /api/v1/entities - Query extracted entities
- POST /api/v1/search - Semantic search interface

## System Integration

### Message Queue Architecture

The system uses message queues for asynchronous processing:

```yaml
queues:
  document_processing:
    type: "redis"
    workers: 4
    retry_policy:
      max_attempts: 3
      backoff: "exponential"
  
  entity_extraction:
    type: "rabbitmq"
    workers: 8
    priority_levels: 3
```

### Database Schema

Core entities and relationships:

```sql
-- Documents table
CREATE TABLE documents (
    id UUID PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    content_type VARCHAR(100),
    size_bytes BIGINT,
    created_at TIMESTAMP DEFAULT NOW(),
    processed_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending'
);

-- Entities table
CREATE TABLE entities (
    id UUID PRIMARY KEY,
    document_id UUID REFERENCES documents(id),
    text VARCHAR(1000) NOT NULL,
    label VARCHAR(50) NOT NULL,
    confidence DECIMAL(3,2),
    start_pos INTEGER,
    end_pos INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Cross-references table
CREATE TABLE cross_references (
    id UUID PRIMARY KEY,
    source_entity_id UUID REFERENCES entities(id),
    target_entity_id UUID REFERENCES entities(id),
    relationship_type VARCHAR(100),
    confidence DECIMAL(3,2),
    context TEXT
);
```

### Configuration Management

Environment-specific configuration:

```yaml
# config/production.yaml
server:
  port: 8080
  host: "0.0.0.0"
  timeout: 30s

database:
  postgres:
    host: "postgres.internal"
    port: 5432
    database: "idaes_prod"
    ssl_mode: "require"
  
  chromadb:
    host: "chromadb.internal"
    port: 8000
    collection: "documents"

extraction:
  batch_size: 50
  max_concurrent: 10
  timeout: 120s
  
edge_optimization:
  memory_limit: 500MB
  chunk_size: 1000
  concurrent_chunks: 2
```

## Deployment Architecture

### Container Orchestration

The system deploys using Docker and Kubernetes:

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: idaes-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: idaes-api
  template:
    metadata:
      labels:
        app: idaes-api
    spec:
      containers:
      - name: api
        image: idaes:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: idaes-secrets
              key: database-url
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Monitoring and Observability

Comprehensive monitoring stack:

```yaml
# monitoring/prometheus.yaml
global:
  scrape_interval: 15s
  
scrape_configs:
  - job_name: 'idaes-api'
    static_configs:
      - targets: ['idaes-api:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s

  - job_name: 'chromadb'
    static_configs:
      - targets: ['chromadb:8000']
```

OpenTelemetry integration:

```go
// Tracing configuration
func setupTracing() {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
    ))
    if err != nil {
        log.Fatal(err)
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("idaes-api"),
            semconv.ServiceVersionKey.String("v1.0.0"),
        )),
    )
    
    otel.SetTracerProvider(tp)
}
```

### Security Architecture

Multi-layered security approach:

```go
// Authentication middleware
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.JSON(401, gin.H{"error": "missing authorization header"})
            c.Abort()
            return
        }
        
        claims, err := validateJWT(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Set("permissions", claims.Permissions)
        c.Next()
    }
}
```

## Performance Optimization

### Caching Strategy

Multi-level caching implementation:

```go
type CacheManager struct {
    l1Cache    *sync.Map          // In-memory cache
    l2Cache    *redis.Client      // Redis cache
    l3Cache    *s3.Client         // S3 cold storage
}

func (cm *CacheManager) Get(key string) (interface{}, error) {
    // Check L1 cache first
    if value, ok := cm.l1Cache.Load(key); ok {
        return value, nil
    }
    
    // Check L2 cache
    value, err := cm.l2Cache.Get(context.Background(), key).Result()
    if err == nil {
        cm.l1Cache.Store(key, value)
        return value, nil
    }
    
    // Check L3 cache
    return cm.getFromS3(key)
}
```

### Memory Management

Intelligent memory optimization for edge devices:

```go
type MemoryManager struct {
    maxMemory     int64
    currentUsage  int64
    mutex         sync.RWMutex
    allocations   map[string]int64
}

func (mm *MemoryManager) AllocateMemory(size int64, component string) error {
    mm.mutex.Lock()
    defer mm.mutex.Unlock()
    
    if mm.currentUsage+size > mm.maxMemory {
        return fmt.Errorf("memory allocation would exceed limit")
    }
    
    mm.currentUsage += size
    mm.allocations[component] = size
    return nil
}
```

## API Documentation

### REST API Endpoints

Complete API reference:

```yaml
openapi: 3.0.0
info:
  title: IDAES API
  version: 1.0.0
  description: Intelligent Document Analysis and Entity Storage API

paths:
  /api/v1/documents:
    post:
      summary: Upload document for processing
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                options:
                  type: object
                  properties:
                    extraction_type:
                      type: string
                      enum: [standard, intelligent, semantic]
      responses:
        '200':
          description: Document uploaded successfully
          content:
            application/json:
              schema:
                type: object
                properties:
                  document_id:
                    type: string
                  status:
                    type: string
                  estimated_completion:
                    type: string

  /api/v1/entities:
    get:
      summary: Query extracted entities
      parameters:
        - name: document_id
          in: query
          schema:
            type: string
        - name: entity_type
          in: query
          schema:
            type: string
        - name: confidence_min
          in: query
          schema:
            type: number
            minimum: 0
            maximum: 1
      responses:
        '200':
          description: Entities retrieved successfully
          content:
            application/json:
              schema:
                type: object
                properties:
                  entities:
                    type: array
                    items:
                      $ref: '#/components/schemas/Entity'
                  total:
                    type: integer
                  page:
                    type: integer

components:
  schemas:
    Entity:
      type: object
      properties:
        id:
          type: string
        text:
          type: string
        label:
          type: string
        confidence:
          type: number
        start_pos:
          type: integer
        end_pos:
          type: integer
        metadata:
          type: object
```

This technical documentation provides a comprehensive overview of the IDAES system architecture, including detailed code examples, configuration specifications, and deployment guidelines for production environments.