# End-to-End Citation Intelligence Test

This directory contains an end-to-end test that verifies the citation intelligence feature of IDAES using real Docker services.

## Test Overview

The `TestCitationIntelligenceE2E` test demonstrates the citation intelligence functionality:

### Phase 1: Document Upload
- Uploads two documents via the web interface:
  - **Document 1** (citing document): Research paper on climate change impacts that cites Document 2
  - **Document 2** (cited document): "Agricultural Resilience in a Changing Climate" by Johnson et al. (2022)

### Phase 2: Citation Intelligence Verification
- Performs a semantic search with a query that would normally only match Document 1
- Verifies that both documents are returned due to citation intelligence
- Checks that citations are properly extracted from Document 1

## Prerequisites

1. **Docker and Docker Compose** must be installed
2. **Docker services must be running**: 
   ```bash
   cd $HOME/workspace/idaes
   docker compose up -d
   ```
3. **Wait for services to be ready** (may take a few minutes for Ollama to download models)

## Running the Test

### Option 1: Run with environment variable
```bash
cd $HOME/workspace/idaes
INTEGRATION_TESTS=1 go test ./internal/example -v -run TestCitationIntelligenceE2E
```

### Option 2: Run with custom service URLs
```bash
cd $HOME/workspace/idaes
INTEGRATION_TESTS=1 \
IDAES_URL=http://localhost:8081 \
CHROMA_URL=http://localhost:8000 \
OLLAMA_URL=http://localhost:11434 \
go test ./internal/example -v -run TestCitationIntelligenceE2E
```

## Test Behavior

- **Skip by default**: The test skips automatically if `INTEGRATION_TESTS` environment variable is not set
- **Service health checks**: Waits for all services (IDAES, ChromaDB, Ollama) to be ready before proceeding
- **Document analysis**: Waits for document analysis to complete after upload
- **Citation extraction**: Verifies that citations are extracted from the citing document
- **Semantic search**: Tests both direct document matching and citation intelligence
- **Cleanup**: Automatically deletes test documents after completion

## Expected Results

### Current Implementation Status
- ✅ **Document Upload**: Should work with real IDAES service
- ✅ **Document Analysis**: Should extract entities, citations, and topics
- ⚠️ **Citation Intelligence**: May not be fully implemented yet
- ⚠️ **Semantic Search**: API endpoints may return "Not Implemented"

### Success Criteria
1. Documents upload successfully
2. Document analysis completes
3. Citations are extracted from Document 1
4. Semantic search returns relevant results
5. Citation intelligence includes cited documents in search results (future implementation)

## Troubleshooting

### Common Issues
1. **Services not ready**: Ensure Docker Compose is running and all services are healthy
2. **Port conflicts**: Check that ports 8000, 8081, and 11434 are available
3. **Ollama models**: First run may take time to download required models
4. **API endpoints**: Some endpoints may return "Not Implemented" - this is expected

### Debug Commands
```bash
# Check service health
curl http://localhost:8081/api/v1/health
curl http://localhost:8000/api/v1/heartbeat
curl http://localhost:11434/api/tags

# Check Docker services
docker compose ps
docker compose logs
```

## Test Document Content

### Document 1 (Citing Document)
- Title: "Research on Climate Change Impacts"
- Contains multiple citations to Document 2
- Used to test citation extraction

### Document 2 (Cited Document)  
- Title: "Agricultural Resilience in a Changing Climate"
- Authors: Johnson, A., Smith, B., Williams, C.
- Year: 2022
- Used to test citation intelligence

## Future Enhancements

This test framework can be extended to test:
- Cross-collection citation relationships
- Citation network analysis
- Advanced search with citation context
- Multi-document citation graphs
- Citation recommendation systems