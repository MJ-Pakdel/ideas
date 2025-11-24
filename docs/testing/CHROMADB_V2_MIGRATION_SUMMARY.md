# ChromaDB v2 API Migration Summary

## Overview
Successfully migrated the IDAES ChromaDB client from deprecated v1 API to the current v2 API specification based on the OpenAPI schema at `chromadb_api/openapi.json`.

## Key Changes Made

### 1. Client Constructor Updates
**Before (v1):**
```go
NewChromaDBClient(baseURL string, timeout time.Duration, maxRetries int) *ChromaDBClient
```

**After (v2):**
```go
NewChromaDBClient(baseURL string, timeout time.Duration, maxRetries int, tenant, database string) *ChromaDBClient
```

**Changes:**
- Added tenant and database parameters to support v2 hierarchy
- Default values: `tenant="default_tenant"`, `database="default_database"`

### 2. API Endpoint Updates

#### Health Check Endpoints
- **v1**: `/api/v1/heartbeat`
- **v2**: `/api/v2/heartbeat` (returns `HeartbeatResponse` with nanosecond timestamp)
- **New**: `/api/v2/healthcheck` (server readiness check)
- **New**: `/api/v2/version` (server version info)

#### Collection Endpoints
**Before (v1):**
- `/api/v1/collections` (list/create)
- `/api/v1/collections/{name}` (get/delete by name)

**After (v2):**
- `/api/v2/tenants/{tenant}/databases/{database}/collections` (list/create)
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}` (get/delete by UUID)

#### Document Operations
**Before (v1):**
- `/api/v1/collections/{name}/add`
- `/api/v1/collections/{name}/query`
- `/api/v1/collections/{name}/get`
- `/api/v1/collections/{name}/update`
- `/api/v1/collections/{name}/delete`
- `/api/v1/collections/{name}/count`

**After (v2):**
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/add`
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/query`
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/get`
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/update`
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/delete`
- `/api/v2/tenants/{tenant}/databases/{database}/collections/{collection_id}/count`

### 3. Data Structure Updates

#### Collection Structure
**Enhanced with v2 fields:**
```go
type Collection struct {
    ID                string                 `json:"id"`
    Name              string                 `json:"name"`
    ConfigurationJSON CollectionConfig       `json:"configuration_json"`
    Tenant            string                 `json:"tenant"`
    Database          string                 `json:"database"`
    LogPosition       int64                  `json:"log_position"`
    Version           int32                  `json:"version"`
    Metadata          map[string]interface{} `json:"metadata,omitempty"`
    Dimension         *int32                 `json:"dimension,omitempty"`
}
```

#### New Response Types
- `HeartbeatResponse` - nanosecond timestamp for heartbeat
- `ErrorResponse` - standardized error format
- Enhanced configuration types: `CollectionConfig`, `EmbeddingFunctionConfig`, `HNSWConfig`, `SPANNConfig`

### 4. Storage Manager Updates

#### Collection ID Management
- Added `collectionIDs map[string]string` to cache name-to-ID mappings
- Created `getCollectionID()` helper method to resolve collection names to UUIDs
- Updated all storage methods to use collection IDs instead of names

#### Method Updates
All storage methods now:
1. Resolve collection name to ID using cache or API lookup
2. Use collection ID for ChromaDB operations
3. Cache collection IDs for performance

### 5. Factory Updates
Updated `analyzers/factory.go`:
```go
// Before
chromaClient := clients.NewChromaDBClient(chromaDBURL, 30*time.Second, 3)

// After  
chromaClient := clients.NewChromaDBClient(chromaDBURL, 30*time.Second, 3, "default_tenant", database)
```

### 6. Request/Response Format Changes

#### AddDocumentsRequest
- **New**: `URIs []string` field for document URIs
- **Updated**: `Embeddings` now required (not optional)

#### QueryResponse / GetResponse
- **New**: `URIs` field in responses
- **New**: `Include []string` field to specify returned data

#### Count Endpoint
- **v1**: Returns `{"count": 42}`
- **v2**: Returns `42` directly (integer)

## Backward Compatibility Considerations

### Collection Name Resolution
The v2 API requires collection UUIDs, but the application logic uses collection names. The migration implements:
1. **Collection ID Caching**: Maps collection names to UUIDs in memory
2. **Lazy Resolution**: Looks up collection IDs when needed via `ListCollections()`
3. **Race Condition Handling**: Gracefully handles concurrent collection creation

### Default Configuration
- Uses `"default_tenant"` and provided database name for v2 hierarchy
- Maintains existing collection naming conventions (`"documents"`, `"entities"`, etc.)

## Testing Impact

### Unit Test Plan Updates
Updated `UNIT_TEST_GENERATION_PLAN.md` to reflect:
- New client constructor signature
- Collection ID-based operations
- New v2 endpoints (healthcheck, version)
- Enhanced error handling patterns

### Current Test Status
- **Build**: ✅ Successful compilation
- **Unit Tests**: ✅ All Phase 1 tests (types, config) passing at 100% coverage
- **Integration Tests**: ⚠️ Some orchestrator timeout issues (unrelated to ChromaDB changes)

## Migration Benefits

1. **Future-Proof**: Uses current, supported ChromaDB API
2. **Enhanced Features**: Access to new v2 capabilities (healthcheck, version info)
3. **Better Error Handling**: Standardized error response format
4. **Tenant/Database Support**: Prepared for multi-tenant deployments
5. **Performance**: Collection ID caching reduces API calls

## Next Steps

1. **Phase 2 Testing**: Implement comprehensive ChromaDB client tests using v2 API
2. **Integration Testing**: Test with actual ChromaDB v2 instance
3. **Performance Validation**: Verify collection ID caching effectiveness
4. **Documentation**: Update API documentation to reflect v2 usage

## Files Modified

1. `internal/clients/chromadb.go` - Complete v2 API migration
2. `internal/storage/chromadb.go` - Collection ID management
3. `internal/analyzers/factory.go` - Client creation update
4. `UNIT_TEST_GENERATION_PLAN.md` - Test plan updates

The migration maintains full backward compatibility at the application level while modernizing the underlying ChromaDB integration to use the current v2 API specification.