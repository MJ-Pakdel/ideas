#!/bin/bash

# IDAES Health Check Script
set -e

echo "🔍 IDAES Health Check"
echo "===================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check service health
check_service() {
    local service_name="$1"
    local url="$2"
    local expected_status="$3"
    
    echo -n "Checking $service_name... "
    
    if curl -s --max-time 5 "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Healthy${NC}"
        return 0
    else
        echo -e "${RED}❌ Unhealthy${NC}"
        return 1
    fi
}

# Function to check if container is running
check_container() {
    local container_name="$1"
    
    if docker ps --format "table {{.Names}}" | grep -q "^${container_name}$"; then
        echo -e "${GREEN}✅ Running${NC}"
        return 0
    else
        echo -e "${RED}❌ Not running${NC}"
        return 1
    fi
}

echo "Container Status:"
echo "=================="
echo -n "idaes-chromadb: "; check_container "idaes-chromadb"
echo -n "idaes-ollama: "; check_container "idaes-ollama" 
echo -n "idaes: "; check_container "idaes"
echo -n "idaes-zipkin: "; check_container "idaes-zipkin"
echo -n "idaes-otel-collector: "; check_container "idaes-otel-collector"
echo -n "idaes-nginx: "; check_container "idaes-nginx"
echo ""

echo "Service Health:"
echo "==============="
check_service "ChromaDB" "http://localhost:8000/api/v1/heartbeat"
check_service "Ollama" "http://localhost:11434/api/health"
check_service "fs-vectorize" "http://localhost:8081/api/v1/health"
check_service "Zipkin" "http://localhost:9411/health"
check_service "Nginx" "http://localhost/nginx-health"
echo ""

echo "Service URLs:"
echo "============="
echo "• Main Application: http://localhost"
echo "• fs-vectorize WebUI: http://localhost:8081"
echo "• ChromaDB API: http://localhost:8000"
echo "• Zipkin Tracing: http://localhost:9411"
echo "• Ollama API: http://localhost:11434"
echo ""

echo "Docker Compose Status:"
echo "======================"
docker compose ps
echo ""

echo "Volume Usage:"
echo "============="
docker volume ls | grep idaes || echo "No IDAES volumes found"
echo ""

echo "Network Information:"
echo "==================="
docker network ls | grep idaes || echo "No IDAES networks found"