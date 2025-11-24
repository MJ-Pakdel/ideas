#!/bin/bash

# IDAES Docker Deployment Script
set -e

echo "🚀 Starting IDAES Docker deployment with telemetry..."

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Error: Docker is not installed or not in PATH"
    exit 1
fi

# Check Docker Buildx version
BUILDX_VERSION=$(docker buildx version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1 | sed 's/v//')
if [ -n "$BUILDX_VERSION" ]; then
    echo "📋 Docker Buildx version: v$BUILDX_VERSION"
    MAJOR=$(echo $BUILDX_VERSION | cut -d. -f1)
    MINOR=$(echo $BUILDX_VERSION | cut -d. -f2)
    
    if [ "$MAJOR" -eq 0 ] && [ "$MINOR" -lt 17 ]; then
        echo "⚠️  Note: Docker Buildx v$BUILDX_VERSION detected. Using separate build step for compatibility."
        echo "   (Docker Compose build requires Buildx v0.17+)"
    fi
fi

# Check if required files exist
if [ ! -f "docker-compose.yml" ]; then
    echo "❌ Error: docker-compose.yml not found"
    exit 1
fi

if [ ! -f "configs/otel-collector-config.yaml" ]; then
    echo "❌ Error: configs/otel-collector-config.yaml not found"
    exit 1
fi

if [ ! -f "Dockerfile" ]; then
    echo "❌ Error: Dockerfile not found"
    exit 1
fi

# Build and start services
echo "📦 Building Docker image..."
docker build -t idaes:latest .

echo "🚀 Starting Docker services..."
docker compose up -d

echo "⏳ Waiting for services to start..."
sleep 30

# Check service health
echo "🔍 Checking service health..."

# Check ChromaDB
if curl -s http://localhost:8000/api/v1/heartbeat > /dev/null; then
    echo "✅ ChromaDB is healthy"
else
    echo "⚠️  ChromaDB may not be ready yet"
fi

# Check fs-vectorize
if curl -s http://localhost:8081/api/v1/health > /dev/null; then
    echo "✅ fs-vectorize is healthy"
else
    echo "⚠️  fs-vectorize may not be ready yet"
fi

# Check Zipkin
if curl -s http://localhost:9411/health > /dev/null; then
    echo "✅ Zipkin is healthy"
else
    echo "⚠️  Zipkin may not be ready yet"
fi

# Check Nginx
if curl -s http://localhost/nginx-health > /dev/null; then
    echo "✅ Nginx is healthy"
else
    echo "⚠️  Nginx may not be ready yet"
fi

echo ""
echo "🎉 Deployment complete!"
echo ""
echo "📋 Service URLs:"
echo "   • Main Application: http://localhost (via Nginx)"
echo "   • fs-vectorize: http://localhost:8081"
echo "   • ChromaDB: http://localhost:8000"
echo "   • Zipkin (Tracing): http://localhost:9411"
echo "   • Ollama: http://localhost:11434"
echo ""
echo "📊 To view traces:"
echo "   Open http://localhost:9411 in your browser"
echo ""
echo "🔧 To view logs:"
echo "   docker compose logs -f [service-name]"
echo ""
echo "🛑 To stop all services:"
echo "   docker compose down"
echo ""
echo "🗑️  To clean up (remove volumes):"
echo "   docker compose down -v"