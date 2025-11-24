# E2E Test API Implementation Plan

**Date**: November 3, 2025  
**Status**: Planning Phase  
**Priority**: High (blocking e2e tests)

## Overview

The end-to-end tests have identified three missing or problematic API endpoints that need implementation:

1. **Search API** (`/api/v1/search/semantic`) - Query parameter handling issues
2. **Citations API** (`/api/v1/documents/{id}/citations`) - Not implemented  
3. **Document Delete API** (`DELETE /api/v1/documents/{id}`) - Not implemented (complex due to intelligent chunking)

## Current State Analysis

### 1. Search API (/api/v1/search/semantic)
**Status**: ✅ IMPLEMENTED with potential edge cases

**Current Implementation**:
- Located in `internal/web/web.go::handleSemanticSearch`
- Query parameters supported:
  - `q` (required): search query text
  - `limit` (optional): results per type (default: 5, max: 50)
  - `types` (optional): comma-separated list (documents,entities,citations,topics)
- Returns aggregated search results across multiple content types

**Potential Issues**:
- URL encoding/decoding edge cases
- Parameter validation edge cases
- Response format inconsistencies
- Error handling gaps

### 2. Citations API (/api/v1/documents/{id}/citations)  
**Status**: ❌ NOT IMPLEMENTED

**Current State**:
- Route exists: `/api/v1/documents/` → `handleDocumentOperations`
- Handler returns: "Document operations not yet implemented"
- Storage layer ready: `ChromaStorageManager` has citation search capabilities
- E2E test expects: `getDocumentCitations(serverURL, documentID)` function

**Required Implementation**:
- URL parsing to extract document ID
- Method routing (GET for citations)
- Citation filtering by document ID
- JSON response formatting

### 3. Document Delete API (DELETE /api/v1/documents/{id})
**Status**: ❌ NOT IMPLEMENTED (COMPLEX - SAVE FOR LAST)

**Current State**:
- Route exists: `/api/v1/documents/` → `handleDocumentOperations`
- Handler returns: "Document operations not yet implemented"
- Storage layer has: `ChromaStorageManager.DeleteDocument()`
- E2E test expects: `deleteDocument(serverURL, documentID)` function

**Complexity Factors**:
- **Intelligent Chunking**: Documents may be split into multiple chunks
- **Cross-Document Relationships**: Citations and entity relationships span documents
- **Dependency Analysis**: Need to check what depends on the document before deletion
- **Cascading Effects**: Deleting one chunk may affect analysis of related chunks

## Implementation Strategy

### Phase 1: Debug Semantic Search Issues (Priority: HIGH)
**Estimated Effort**: 1-2 hours

**Tasks**:
1. Run e2e test to capture specific failure scenarios
2. Review URL parameter parsing edge cases
3. Test response format compatibility
4. Fix any identified issues

**Implementation Approach**:
```bash
# Debug e2e test failures
cd $HOME/workspace/idaes
INTEGRATION_TESTS=1 go test -v ./internal/example -run TestCitationIntelligenceE2E
```

**Potential Fixes**:
- Enhanced parameter validation
- Better error messages
- Response format standardization

### Phase 2: Implement Citations API (Priority: HIGH)
**Estimated Effort**: 4-6 hours

**Implementation Steps**:

1. **Enhance URL Routing** (`internal/web/web.go`)
   ```go
   func (s *Server) handleDocumentOperations(w http.ResponseWriter, r *http.Request) {
       // Parse document ID from URL: /api/v1/documents/{id} or /api/v1/documents/{id}/citations
       // Route based on HTTP method and path suffix
   }
   ```

2. **Add Citations Handler**
   ```go
   func (s *Server) handleDocumentCitations(w http.ResponseWriter, r *http.Request, documentID string) {
       // Get citations for specific document
       // Use storage layer with document filter
       // Return JSON response
   }
   ```

3. **Storage Layer Integration**
   - Use existing `SearchCitations()` with document ID filter
   - Consider adding `GetDocumentCitations(documentID)` method for efficiency

4. **Response Format**
   ```json
   {
     "success": true,
     "data": {
       "document_id": "doc123",
       "citations": [
         {
           "id": "cite1",
           "text": "Smith, J. (2023)...",
           "format": "APA",
           "confidence": 0.95
         }
       ],
       "total_found": 5
     }
   }
   ```

**Testing Requirements**:
- Unit tests for URL parsing
- Unit tests for citation retrieval
- Integration tests with mock storage
- Error case handling (document not found, no citations)

### Phase 3: Design Intelligent Document Deletion Strategy (Priority: MEDIUM)
**Estimated Effort**: 8-12 hours (design + implementation)

**Complexity Analysis**:

#### 3.1 Document Relationship Mapping
Documents in IDAES have complex relationships:

```
Document A (Original)
├── Chunk A1 (entities: Person, Organization)
├── Chunk A2 (citations: [Doc B, Doc C])
└── Chunk A3 (topics: Climate, Agriculture)

Document B (Cited by A)
├── Entities referenced by A
└── Citations that may reference A

Cross-Document Intelligence:
- Citation networks
- Entity co-occurrence
- Topic clustering
- Semantic similarity
```

#### 3.2 Deletion Impact Analysis
Before deleting a document/chunk, we need to:

1. **Identify Dependencies**
   - Which documents cite this document?
   - Which entities are unique to this document?
   - Which topics would lose significant evidence?
   - Which semantic clusters would be affected?

2. **Cascade Analysis**
   - Re-analyze citing documents (may lose citation intelligence)
   - Update entity confidence scores
   - Recalculate topic confidence
   - Update semantic embeddings

3. **Orphan Detection**
   - Citations pointing to deleted document
   - Entities only found in deleted document
   - Topics that become under-supported

#### 3.3 Intelligent Deletion Strategies

**Option A: Simple Deletion**
- Delete document and associated data
- Mark dependent citations as "broken"
- Log warnings about potential impacts

**Option B: Dependency-Aware Deletion**
- Analyze dependencies before deletion
- Warn user about impacts
- Offer cascade vs. orphan options

**Option C: Soft Deletion with Recovery**
- Mark document as deleted but preserve data
- Allow recovery within time window
- Clean up after recovery period

**Recommended Approach**: Start with Option A, evolve to Option B

#### 3.4 Implementation Plan for Document Deletion

**Step 1: Dependency Analysis Service**
```go
type DeletionImpactAnalysis struct {
    DocumentID          string
    CitingDocuments     []string
    UniqueEntities      []string
    AffectedTopics      []string
    OrphanedCitations   []string
    SemanticClusters    []string
    RecommendedAction   DeletionAction
}

type DeletionAction int
const (
    SafeToDelete DeletionAction = iota
    WarnUser
    RequireConfirmation
    BlockDeletion
)
```

**Step 2: Deletion Handler**
```go
func (s *Server) handleDocumentDelete(w http.ResponseWriter, r *http.Request, documentID string) {
    // 1. Analyze deletion impact
    impact := s.analyzeDeletionImpact(documentID)
    
    // 2. Check if force parameter provided
    force := r.URL.Query().Get("force") == "true"
    
    // 3. Apply deletion policy
    if impact.RecommendedAction == BlockDeletion {
        s.sendError(w, r, http.StatusConflict, "Cannot delete: critical dependencies")
        return
    }
    
    if impact.RecommendedAction == RequireConfirmation && !force {
        s.sendJSON(w, r, http.StatusConflict, map[string]any{
            "error": "confirmation_required",
            "impact": impact,
            "message": "Use force=true to confirm deletion"
        })
        return
    }
    
    // 4. Perform deletion
    err := s.performIntelligentDeletion(documentID, impact)
    // 5. Return result
}
```

**Step 3: Cascading Updates**
- Re-analyze affected documents
- Update citation intelligence
- Recalculate entity/topic confidences
- Update semantic indexes

## Implementation Timeline

### Week 1: Foundation
- [ ] Debug semantic search issues
- [ ] Implement basic URL routing
- [ ] Implement citations API
- [ ] Add basic tests

### Week 2: Intelligent Deletion Design
- [ ] Design dependency analysis system
- [ ] Create deletion impact analysis
- [ ] Plan cascading update strategy
- [ ] Design user interface for deletion warnings

### Week 3: Implementation
- [ ] Implement dependency analysis
- [ ] Implement intelligent deletion handler
- [ ] Add cascading update logic
- [ ] Comprehensive testing

### Week 4: Integration & Testing
- [ ] End-to-end testing
- [ ] Performance optimization
- [ ] Documentation updates
- [ ] User interface integration

## Risk Assessment

### High Risk
- **Document Deletion Complexity**: Intelligent chunking creates complex dependency webs
- **Performance Impact**: Dependency analysis could be expensive
- **Data Integrity**: Cascading updates must maintain consistency

### Medium Risk  
- **API Compatibility**: Changes might affect existing clients
- **Storage Layer Changes**: May need new storage methods

### Low Risk
- **Citations API**: Straightforward implementation using existing storage
- **Semantic Search Debug**: Likely minor fixes

## Success Criteria

1. **E2E Tests Pass**: All citation intelligence tests pass
2. **API Completeness**: All three endpoints fully functional
3. **Data Integrity**: No orphaned data after deletions
4. **Performance**: Deletion impact analysis completes in <5 seconds
5. **User Experience**: Clear warnings about deletion impacts

## Future Considerations

### Advanced Features
- **Deletion Undo**: Implement soft deletion with recovery
- **Bulk Operations**: Delete multiple documents intelligently
- **Migration Tools**: Move citations when documents are deleted
- **Analytics**: Track deletion patterns and impacts

### Monitoring
- **Deletion Metrics**: Track what gets deleted and why
- **Impact Metrics**: Measure accuracy changes after deletions
- **Performance Metrics**: Monitor dependency analysis performance

## Conclusion

The implementation should proceed in phases, with intelligent document deletion being the most complex component. The citations API can be implemented quickly, while document deletion requires careful design to preserve the integrity of the intelligent document analysis system.

The key insight is that IDAES is not just a document storage system—it's an intelligent analysis system where documents are interconnected through citations, entities, topics, and semantic relationships. Deletion must respect and maintain these relationships.