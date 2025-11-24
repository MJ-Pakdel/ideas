# IDAES Extractor Architecture Decision

## Overview
This document explains the architectural decision to implement fail-fast behavior for entity extraction while allowing citations and topics to be optional.

## Extractor Categories & Rationale

### 🧠 Entity Extraction: **CRITICAL** - LLM Required
- **Decision**: Fail-fast if LLM entity extraction unavailable
- **Rationale**: 
  - IDAES stands for "**Intelligent** Document Analysis and Entity Storage"
  - Entity extraction is the core value proposition
  - Regex entity extraction provides poor-quality results that mislead users
  - Falls back degrades the "intelligent" aspect of IDAES
- **Implementation**: System fails to start if LLM entity extractor cannot be created
- **Available Methods**: `LLM` (requires functional LLM connectivity)

### 📚 Citation Extraction: **OPTIONAL** - Regex Appropriate  
- **Decision**: Allow system to continue without citation extraction
- **Rationale**:
  - Citations follow standardized academic formats (APA, MLA, Chicago, etc.)
  - Regex patterns are actually well-suited for citation detection
  - Citation extraction is supplementary to core entity analysis
  - Many documents don't contain academic citations
- **Implementation**: Warning logged if citation extractor fails, system continues
- **Available Methods**: `Regex` (appropriate for standardized formats)

### 🏷️ Topic Extraction: **OPTIONAL** - Limited Implementation
- **Decision**: Allow system to continue without topic extraction  
- **Rationale**:
  - Current regex implementation uses simple keyword matching
  - Topic analysis is supplementary to core entity extraction
  - Would benefit from LLM implementation in the future
  - Not critical for IDAES core functionality
- **Implementation**: Warning logged if topic extractor fails, system continues
- **Available Methods**: `Regex` (basic keyword matching)

## System Behavior

### ✅ Successful Startup
```
✓ Ollama connectivity confirmed
✓ ChromaDB connectivity confirmed  
✓ LLM entity extractor created
✓ Citation extractor created (regex)
✓ Topic extractor created (regex)
→ System ready for intelligent document analysis
```

### ❌ Failed Startup (LLM Issues)
```
✓ Ollama connectivity confirmed
✓ ChromaDB connectivity confirmed
❌ CRITICAL: LLM entity extractor failed - failing fast
→ Application exits with error
```

### ⚠️ Partial Startup (Optional Extractors Failed)
```
✓ Ollama connectivity confirmed
✓ ChromaDB connectivity confirmed
✓ LLM entity extractor created
⚠️ Citation extractor failed - citation analysis disabled
⚠️ Topic extractor failed - topic analysis disabled  
→ System continues with core entity analysis functionality
```

## Configuration Impact

### Default Configuration
```go
EntityConfig: &interfaces.EntityExtractionConfig{
    Enabled: true,
    Method:  types.ExtractorMethodLLM, // Intelligent LLM-only approach
    // ... other settings
}
```

### Fail-Fast Validation
- System validates LLM connectivity during startup
- Entity extractors require functional LLM connection
- Citation and topic extractors are optional enhancements

## Future Enhancements

### Planned Improvements
1. **LLM Topic Extraction**: Implement intelligent topic analysis using LLM
2. **Citation Enhancement**: Add LLM-based citation validation and metadata extraction
3. **Relationship Extraction**: Add entity relationship detection using LLM
4. **Multi-language Support**: Extend extractors for non-English documents

### Architecture Evolution
- Keep fail-fast behavior for core intelligence features
- Add LLM-based alternatives for current regex-only extractors
- Maintain optional nature of supplementary analysis features

## Benefits of This Architecture

1. **🎯 Clear Value Proposition**: IDAES guarantees intelligent entity analysis
2. **🚫 No Silent Degradation**: Users know when core functionality is compromised  
3. **⚡ Fast Failure**: Problems are detected at startup, not during processing
4. **🔧 Appropriate Tools**: Regex for structured data (citations), LLM for unstructured (entities)
5. **📈 Graceful Enhancement**: Optional features add value without blocking core functionality

## Implementation Notes

### Error Handling
- **Critical errors**: Fail fast with detailed error messages
- **Optional feature errors**: Log warnings, continue operation
- **Runtime errors**: Maintain existing error handling in pipeline

### Monitoring
- Track success rates for each extractor type
- Monitor LLM connectivity and performance
- Alert on degraded functionality

### Testing
- Unit tests for each extractor type
- Integration tests for fail-fast behavior
- End-to-end tests for partial functionality scenarios