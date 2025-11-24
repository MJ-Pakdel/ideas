# IDAES Test Data

This directory contains test files used for validating different aspects of the IDAES document analysis system.

## Test Files

### Basic Testing

- **`test_doc.txt`** - Simple test document with minimal content
  - Size: 43 bytes
  - Purpose: Basic functionality testing and quick validation
  - Contains: Simple text content for basic document processing tests

### Entity Extraction Testing

- **`test_timeout.txt`** - Comprehensive test document for timeout and entity extraction
  - Size: 874 bytes  
  - Purpose: Testing timeout handling and comprehensive entity extraction
  - Contains multiple entity types:
    - **Person**: John Smith
    - **Organization**: Microsoft Corporation
    - **Date**: December 15, 2024
    - **Email**: john.smith@microsoft.com
    - **Location**: Seattle, Washington
    - **Metric**: 25 seconds processing time
  - Use case: Validating that the pipeline can handle longer processing times without timeouts

### Citation Extraction Testing

- **`citation_test.txt`** - Document with academic citations
  - Size: 120 bytes
  - Purpose: Testing citation extraction functionality
  - Contains: Academic citation in standard format - "Smith et al. (2023) 'Advanced Analysis Methods'"
  - Use case: Validating citation detection and extraction algorithms

## Usage Examples

### Via Scripts
```bash
# Test with timeout document (most comprehensive)
curl -X POST -F "file=@testdata/test_timeout.txt" http://localhost:8081/api/v1/documents/upload

# Test citation extraction
curl -X POST -F "file=@testdata/citation_test.txt" http://localhost:8081/api/v1/documents/upload

# Quick basic test
curl -X POST -F "file=@testdata/test_doc.txt" http://localhost:8081/api/v1/documents/upload
```

### Via End-to-End Tests
```bash
# Run comprehensive e2e tests using these files
./scripts/run-e2e-test.sh
```

## Expected Analysis Results

### test_timeout.txt
- **Entities**: 5-7 entities (person, organization, date, email, location, metrics)
- **Processing Time**: 20-40 seconds (includes LLM entity extraction)
- **Citations**: 0 (no academic citations)
- **Topics**: General business/project management themes

### citation_test.txt  
- **Entities**: 2-3 entities (authors, publication)
- **Processing Time**: 5-15 seconds
- **Citations**: 1 academic citation
- **Topics**: Research/academic themes

### test_doc.txt
- **Entities**: 0-1 entities 
- **Processing Time**: < 5 seconds
- **Citations**: 0
- **Topics**: General/minimal

## Adding New Test Files

When adding new test data:

1. **Naming Convention**: Use descriptive names like `entity_complex.txt`, `citation_multiple.txt`, `topic_analysis.txt`
2. **Size Considerations**: 
   - Small files (< 100 bytes): Quick tests
   - Medium files (100-1000 bytes): Standard validation
   - Large files (> 1000 bytes): Stress testing
3. **Content Guidelines**:
   - Include diverse entity types for entity extraction testing
   - Use proper citation formats for citation testing
   - Add clear topic indicators for topic analysis testing
4. **Documentation**: Update this README with the new file's purpose and expected results

## Integration with Testing

These files are used by:
- Manual testing via curl commands
- Automated end-to-end tests
- Development validation during pipeline fixes
- Performance benchmarking
- Feature validation for extractors