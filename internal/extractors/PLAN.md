# PLAN.md
A significant part of the IDAES architecture is a dual model approach.  One model is used to create embeddings.  One model is used to extract entities, topics, and citations as well as identify the classification of the document.  We need to create an extractor that will use a prompt (or multiple prompts) to achieve the extraction and classification.  It will use the Ollama client at its foundation.  The response should be JSON so it can be parsed by the rest of the intelligence layer.  In testing, these interfaces should be mocked but you should actually implement calls to Ollama in production code.

There should be no regex extractors.

## ARCHITECTURAL DECISION: Hybrid Extractor Approach (Option A)

**Decision Made**: November 2, 2025

**Primary Method**: Unified LLM Extractor
- Single comprehensive LLM call that extracts entities, citations, topics, and classification in one pass
- More efficient for edge hardware (lower latency, fewer API calls)
- Better context sharing between extraction types
- Primary implementation in `LLMUnifiedExtractor`

**Secondary Method**: Individual Extractors as Lightweight Wrappers
- `LLMEntityExtractor`, `LLMCitationExtractor`, `LLMTopicExtractor` are implemented as lightweight wrappers around the unified extractor
- They call the unified extractor and extract only their specific results
- Maintains interface compatibility with existing factory patterns
- Provides fallback capability if needed

**Benefits**:
- Best of both worlds: efficiency + interface compatibility
- Single LLM call for most use cases (cost/latency optimization)
- Maintains existing analyzer factory expectations
- Future flexibility to add focused prompts if needed

**Implementation Notes**:
- Individual extractors use `UnifiedExtractor.ExtractAll()` internally
- Each individual extractor filters results to return only their specific type
- All extractors share the same underlying LLM analysis for consistency