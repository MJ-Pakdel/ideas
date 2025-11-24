# IDAES Docker Deployment

This directory contains the Docker configuration for deploying the IDAES (Intelligent Document Analysis and Extraction System) with full telemetry support.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                              IDAES Docker Stack                            │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐   │
│  │  Nginx  │    │  IDAES  │    │ChromaDB │    │ Ollama  │    │ Zipkin  │   │
│  │  :80    │◄──►│  :8081  │◄──►│  :8000  │    │ :11434  │    │  :9411  │   │
│  │         │    │         │    │         │    │         │    │         │   │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘   │
│       │              │              │              │              │        │
│       │              │              │              │              │        │
│       │              └──────────────┼──────────────┘              │        │
│       │                             │                             │        │
│       │         ┌─────────────────────────────────────────────────┘        │
│       │         │                   │                                      │
│       │         │         ┌─────────────────┐                              │
│       │         │         │ OTEL Collector  │                              │
│       │         │         │    (internal)   │                              │
│       │         │         └─────────────────┘                              │
│       │         │                   │                                      │
│       └─────────┼───────────────────┘                                      │
│                 │                                                          │
│  ┌──────────────┴──────────────────────────────────────────────────────┐   │
│  │                        idaes-network                                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                            │
│  Persistent Volumes:                                                       │
│  • chromadb_data    • ollama_data    • fs_vectorize_data    • nginx_logs   │
└────────────────────────────────────────────────────────────────────────────┘
```

The deployment includes the following services:

- **IDAES**: Main application (port 8081)
- **ChromaDB**: Vector database with telemetry (port 8000)
- **Ollama**: Language model service (port 11434)
- **OpenTelemetry Collector**: Telemetry collection (internal)
- **Zipkin**: Distributed tracing UI (port 9411)
- **Nginx**: Reverse proxy and load balancer (port 80)

## Telemetry

The deployment includes comprehensive telemetry:

- **Distributed Tracing**: ChromaDB operations are traced via OpenTelemetry
- **Metrics Collection**: System metrics collected via OTEL collector
- **Trace Visualization**: Zipkin UI for viewing distributed traces

### Telemetry Flow

```
ChromaDB → OpenTelemetry Collector → Zipkin
```

ChromaDB sends telemetry data to the OTEL collector on port 4317 (gRPC), which then forwards traces to Zipkin for visualization.

## ChromaDB Configuration

The IDAES application supports configurable ChromaDB tenant and database organization:

### Configuration Flags

- `--chroma-tenant`: ChromaDB tenant name (default: "default_tenant")
- `--chroma-database`: ChromaDB database name (default: "default_database")

### Meta Collections

On startup, IDAES automatically creates essential meta collections if they don't exist:

- `documents`: Main document chunks with embeddings for semantic search
- `entities`: Extracted entities and their relationships across documents
- `citations`: Academic citations and cross-references between documents
- `topics`: Topic analysis and clustering results for content organization
- `metadata`: Document metadata and classifications for content management

### Example Usage

```bash
# Use custom tenant and database
./main -chroma-tenant="research_project" -chroma-database="academic_papers"

# Use with docker
docker run idaes:latest -chroma-tenant="myorg" -chroma-database="documents"
```

## Quick Start

### Production Deployment (Recommended)

1. **Deploy the stack:**
   ```bash
   ./scripts/deploy.sh
   ```

### Development Deployment (with live build)

1. **Deploy the development stack:**

   ```bash
   ./scripts/deploy-dev.sh
   # or
   make docker-dev-up
   ```

2. **Access services:**
   - Main application: http://localhost
   - IDAES WebUI: http://localhost:8081
   - ChromaDB API: http://localhost:8000
   - Zipkin tracing: http://localhost:9411
   - Ollama API: http://localhost:11434

## Deployment Options

### Production Mode (`docker-compose.yml`)

- Uses pre-built `idaes:latest` image
- Faster startup, suitable for production
- Compatible with older Docker Buildx versions
- Requires `docker build` step before compose up
- Uses external Ollama service (requires separate setup)

### Ollama Integration Mode (`docker-compose-ollama.yml`)

- **Recommended for self-contained deployments**
- Includes integrated Ollama container with persistent models
- Pre-configured with `llama3.2:1b` and `nomic-embed-text` models
- Automatic model downloading on first startup
- Persistent model storage survives container restarts
- No external Ollama setup required

## Manual Deployment

### Production Mode

```bash
# Build the image first
docker build -t idaes:latest .

# Start all services
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v
```

### Ollama Integration Mode (Recommended)

```bash
# Build and start all services with integrated Ollama
docker build -t idaes:latest .
docker compose -f docker-compose-ollama.yml up -d

# View logs (models download on first startup)
docker compose -f docker-compose-ollama.yml logs -f

# Stop services (models persist in volume)
docker compose -f docker-compose-ollama.yml down

# Stop and remove all data including models
docker compose -f docker-compose-ollama.yml down -v
```

## Make Targets

### Production Targets

- `make docker-build` - Build Docker image
- `make docker-up` - Start services (builds image first)
- `make docker-down` - Stop services
- `make docker-deploy` - Full deployment with health checks

### Development Targets

- `make docker-dev-up` - Start development services with build
- `make docker-dev-down` - Stop development services
- `make docker-dev-logs` - View development logs

### Utility Targets

- `make docker-health` - Run health checks
- `make docker-status` - Show service status
- `make docker-clean` - Clean up resources

## Configuration Files

- `docker-compose.yml`: Production service definitions (external Ollama)
- `docker-compose-ollama.yml`: Self-contained deployment with integrated Ollama
- `Dockerfile`: IDAES application container build
- `configs/otel-collector-config.yaml`: OpenTelemetry collector configuration
- `docker/nginx.conf`: Nginx reverse proxy configuration

## Monitoring and Debugging

### View Service Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f chromadb
docker-compose logs -f fs-vectorize
docker-compose logs -f zipkin
```

### Check Service Health

```bash
# ChromaDB
curl http://localhost:8000/api/v1/heartbeat

# fs-vectorize
curl http://localhost:8081/api/v1/health

# Zipkin
curl http://localhost:9411/health
```

### Viewing Traces

1. Open Zipkin UI: http://localhost:9411
2. Click "Run Query" to see recent traces
3. Click on individual traces to see detailed timing information

## Volumes

The following Docker volumes persist data:

- `chromadb_data`: ChromaDB vector store data
- `fs_vectorize_data`: IDAES application data
- `nginx_logs`: Nginx access and error logs
- `ollama_data`: Ollama model files (docker-compose-ollama.yml only)

## Network

All services communicate via the `idaes-network` bridge network, providing:

- Service discovery by container name
- Isolated network communication
- Simplified inter-service connections

## Troubleshooting

### Docker Buildx Compatibility Issues

If you see "compose build requires buildx 0.17 or later":

```bash
# Use production deployment (separates build and run)
./scripts/deploy.sh

# Or use development mode if you have newer buildx
./scripts/deploy-dev.sh

# Check your buildx version
docker buildx version
```

### Services Won't Start

```bash
# Check Docker system
docker system df
docker system prune

# Restart with fresh build
docker-compose down -v
docker-compose up --build
```

### ChromaDB Connection Issues

```bash
# Check ChromaDB logs
docker-compose logs chromadb

# Test direct connection
curl http://localhost:8000/api/v1/heartbeat
```

### Telemetry Not Working

```bash
# Check OTEL collector logs
docker-compose logs otel-collector

# Check Zipkin logs
docker-compose logs zipkin

# Verify OTEL endpoint in ChromaDB
docker-compose exec chromadb env | grep CHROMA_OPEN_TELEMETRY
```

## Development

For development, you can override specific services:

```bash
# Run only ChromaDB and telemetry for local development
docker-compose up chromadb zipkin otel-collector -d

# Build and run only the IDAES app
docker-compose up --build fs-vectorize
```
