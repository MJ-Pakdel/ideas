#!/bin/bash

# setup-ollama-models.sh
# Script to download required Ollama models for IDAES

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}IDAES Ollama Model Setup${NC}"
echo "=============================="
echo

# Check if Docker Compose is running
if ! docker compose ps | grep -q "ollama.*Up"; then
    echo -e "${RED}Error: Ollama container is not running${NC}"
    echo "Please run: docker compose up -d"
    exit 1
fi

echo -e "${YELLOW}Downloading required models...${NC}"
echo

# Set OLLAMA_HOST to use the exposed port
export OLLAMA_HOST=http://localhost:11434

# Models required by IDAES
INTELLIGENCE_MODEL="llama3.2:1b"
EMBEDDING_MODEL="nomic-embed-text"

echo -e "Pulling ${INTELLIGENCE_MODEL} (for entity extraction and analysis)..."
if ollama pull "$INTELLIGENCE_MODEL"; then
    echo -e "${GREEN}✓ Successfully pulled ${INTELLIGENCE_MODEL}${NC}"
else
    echo -e "${RED}✗ Failed to pull ${INTELLIGENCE_MODEL}${NC}"
    exit 1
fi

echo
echo -e "Pulling ${EMBEDDING_MODEL} (for text embeddings)..."
if ollama pull "$EMBEDDING_MODEL"; then
    echo -e "${GREEN}✓ Successfully pulled ${EMBEDDING_MODEL}${NC}"
else
    echo -e "${RED}✗ Failed to pull ${EMBEDDING_MODEL}${NC}"
    exit 1
fi

echo
echo -e "${GREEN}Model setup complete!${NC}"
echo
echo "Available models:"
ollama list

echo
echo -e "${YELLOW}Note: You may need to restart the IDAES service for it to detect the models:${NC}"
echo "docker compose restart idaes"