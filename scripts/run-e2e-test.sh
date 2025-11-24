#!/bin/bash

# Citation Intelligence E2E Test Runner
# This script sets up and runs the end-to-end citation intelligence test

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
IDAES_URL=${IDAES_URL:-"http://localhost:8081"}
CHROMA_URL=${CHROMA_URL:-"http://localhost:8000"}
OLLAMA_URL=${OLLAMA_URL:-"http://spark:11434"}
SKIP_DOCKER_CHECK=${SKIP_DOCKER_CHECK:-"false"}

echo -e "${BLUE}Citation Intelligence E2E Test Runner${NC}"
echo "=================================="
echo

# Function to check if a service is ready
check_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    local attempt=1
    
    echo -n "Checking $name ($url)... "
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Ready${NC}"
            return 0
        fi
        
        if [ $attempt -eq $max_attempts ]; then
            echo -e "${RED}✗ Failed${NC}"
            return 1
        fi
        
        sleep 2
        attempt=$((attempt + 1))
    done
}

# Check if Docker Compose is running
if [ "$SKIP_DOCKER_CHECK" != "true" ]; then
    echo "Checking Docker Compose services..."
    
    if ! docker compose ps | grep -q "Up"; then
        echo -e "${YELLOW}Warning: Docker Compose services don't appear to be running${NC}"
        echo "Please run: docker compose up -d"
        echo
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
    echo
fi

# Check service health
echo "Waiting for services to be ready..."
check_service "$IDAES_URL/api/v1/health" "IDAES"
check_service "$CHROMA_URL/api/v2/heartbeat" "ChromaDB"  
check_service "$OLLAMA_URL/api/tags" "Ollama"
echo

# Run the test
echo "Running Citation Intelligence E2E Test..."
echo "========================================"

export INTEGRATION_TESTS=1
export IDAES_URL="$IDAES_URL"
export CHROMA_URL="$CHROMA_URL" 
export OLLAMA_URL="$OLLAMA_URL"

if go test ./internal/example -v -run TestCitationIntelligenceE2E; then
    echo
    echo -e "${GREEN}✓ Test completed successfully!${NC}"
    exit 0
else
    echo
    echo -e "${RED}✗ Test failed${NC}"
    exit 1
fi