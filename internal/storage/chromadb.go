// Package storage provides storage management implementations for different backends.package storage

// This package contains storage managers for ChromaDB vector storage with collection
// management, document operations, and search capabilities.
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/example/idaes/internal/clients"
	"github.com/example/idaes/internal/interfaces"
	"github.com/example/idaes/internal/types"
)

// ChromaStorageManager implements the StorageManager interface using ChromaDB
type ChromaStorageManager struct {
	client             interfaces.ChromaDBClient
	llmClient          interfaces.LLMClient
	defaultCollection  string
	entityCollection   string
	citationCollection string
	topicCollection    string
	embeddingDimension int
	maxRetries         int
	retryDelay         time.Duration
	// Map collection names to their IDs for v2 API compatibility
	collectionIDs map[string]string
}

// NewChromaStorageManager creates a new ChromaDB storage manager
func NewChromaStorageManager(
	client *clients.ChromaDBClient,
	llmClient interfaces.LLMClient,
	defaultCollection, entityCollection, citationCollection, topicCollection string,
	embeddingDimension int,
	maxRetries int,
	retryDelay time.Duration,
) *ChromaStorageManager {

	return &ChromaStorageManager{
		client:             NewChromaDBClientAdapter(client),
		llmClient:          llmClient,
		defaultCollection:  defaultCollection,
		entityCollection:   entityCollection,
		citationCollection: citationCollection,
		topicCollection:    topicCollection,
		embeddingDimension: embeddingDimension,
		maxRetries:         maxRetries,
		retryDelay:         retryDelay,
		collectionIDs:      make(map[string]string),
	}
}

// NewChromaStorageManagerWithClient creates a new ChromaDB storage manager with injectable client
// This constructor allows for dependency injection and is preferred for testing
func NewChromaStorageManagerWithClient(
	client interfaces.ChromaDBClient,
	llmClient interfaces.LLMClient,
	defaultCollection, entityCollection, citationCollection, topicCollection string,
	embeddingDimension int,
	maxRetries int,
	retryDelay time.Duration,
) *ChromaStorageManager {

	return &ChromaStorageManager{
		client:             client,
		llmClient:          llmClient,
		defaultCollection:  defaultCollection,
		entityCollection:   entityCollection,
		citationCollection: citationCollection,
		topicCollection:    topicCollection,
		embeddingDimension: embeddingDimension,
		maxRetries:         maxRetries,
		retryDelay:         retryDelay,
		collectionIDs:      make(map[string]string),
	}
}

// Initialize sets up the storage manager and creates necessary collections
func (s *ChromaStorageManager) Initialize(ctx context.Context) error {
	slog.InfoContext(ctx, "initializing ChromaDB storage manager")

	// Check ChromaDB health
	if err := s.client.Health(ctx); err != nil {
		return fmt.Errorf("ChromaDB health check failed: %w", err)
	}

	// Create default collections if they don't exist
	collections := []struct {
		name        string
		description string
	}{
		{s.defaultCollection, "Default collection for documents"},
		{s.entityCollection, "Collection for extracted entities"},
		{s.citationCollection, "Collection for extracted citations"},
		{s.topicCollection, "Collection for extracted topics"},
	}

	for _, coll := range collections {
		if err := s.ensureCollection(ctx, coll.name, coll.description); err != nil {
			return fmt.Errorf("failed to ensure collection %s: %w", coll.name, err)
		}
	}

	slog.InfoContext(ctx, "chromaDB storage manager initialized successfully")
	return nil
}

// StoreDocument stores a document in the default collection
func (s *ChromaStorageManager) StoreDocument(ctx context.Context, doc *types.Document) error {
	return s.StoreDocumentInCollection(ctx, s.defaultCollection, doc)
}

// StoreDocumentInCollection stores a document in a specific collection
func (s *ChromaStorageManager) StoreDocumentInCollection(ctx context.Context, collection string, doc *types.Document) error {
	slog.DebugContext(ctx, "Storing document",
		"document_id", doc.ID,
		"collection", collection,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to get collection ID for %s: %w", collection, err)
	}

	// Generate embedding for the document content
	embedding, err := s.llmClient.Embed(ctx, doc.Content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Prepare metadata
	metadata := map[string]any{
		"path":         doc.Path,
		"name":         doc.Name,
		"content_type": doc.ContentType,
		"size":         doc.Size,
		"hash":         doc.Hash,
		"created_at":   doc.CreatedAt.Format(time.RFC3339),
		"updated_at":   doc.UpdatedAt.Format(time.RFC3339),
	}

	// Add custom metadata
	for k, v := range doc.Metadata {
		metadata[k] = v
	}

	// Store in ChromaDB
	request := interfaces.AddDocumentsRequest{
		IDs:        []string{doc.ID},
		Embeddings: [][]float64{embedding},
		Documents:  []string{doc.Content},
		Metadatas:  []map[string]any{metadata},
	}

	if err := s.client.AddDocuments(ctx, collectionID, request); err != nil {
		return fmt.Errorf("failed to store document in ChromaDB: %w", err)
	}

	slog.InfoContext(ctx, "Document stored successfully", "document_id", doc.ID)
	return nil
}

// GetDocument retrieves a document by ID from the default collection
func (s *ChromaStorageManager) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	return s.GetDocumentFromCollection(ctx, s.defaultCollection, id)
}

// GetDocumentFromCollection retrieves a document by ID from a specific collection
func (s *ChromaStorageManager) GetDocumentFromCollection(ctx context.Context, collection, id string) (*types.Document, error) {
	slog.DebugContext(ctx, "Retrieving document",
		"document_id", id,
		"collection", collection,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection ID for %s: %w", collection, err)
	}

	response, err := s.client.GetDocuments(ctx, collectionID, []string{id}, []string{"documents", "metadatas"})
	if err != nil {
		return nil, fmt.Errorf("failed to get document from ChromaDB: %w", err)
	}

	if len(response.IDs) == 0 {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	// Convert ChromaDB response to Document
	doc, err := s.convertToDocument(response.IDs[0], response.Documents[0], response.Metadatas[0])
	if err != nil {
		return nil, fmt.Errorf("failed to convert ChromaDB response: %w", err)
	}

	return doc, nil
}

// SearchDocuments performs semantic search in the default collection
func (s *ChromaStorageManager) SearchDocuments(ctx context.Context, query string, limit int) ([]*types.Document, error) {
	return s.SearchDocumentsInCollection(ctx, s.defaultCollection, query, limit)
}

// SearchDocumentsInCollection performs semantic search in a specific collection
func (s *ChromaStorageManager) SearchDocumentsInCollection(ctx context.Context, collection, query string, limit int) ([]*types.Document, error) {
	slog.DebugContext(ctx, "Searching documents",
		"query", query,
		"collection", collection,
		"limit", limit,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection ID for %s: %w", collection, err)
	}

	// Generate embedding for the query
	embedding, err := s.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in ChromaDB
	request := interfaces.QueryRequest{
		QueryEmbeddings: [][]float64{embedding},
		NResults:        limit,
		Include:         []string{"documents", "metadatas", "distances"},
	}

	response, err := s.client.QueryDocuments(ctx, collectionID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents in ChromaDB: %w", err)
	}

	if len(response.IDs) == 0 {
		return []*types.Document{}, nil
	}

	// Convert results to documents
	documents := make([]*types.Document, 0, len(response.IDs[0]))
	for i, id := range response.IDs[0] {
		doc, err := s.convertToDocument(id, response.Documents[0][i], response.Metadatas[0][i])
		if err != nil {
			slog.WarnContext(ctx, "Failed to convert search result", "document_id", id, "error", err)
			continue
		}

		// Add search score
		if len(response.Distances[0]) > i {
			doc.Metadata["search_score"] = response.Distances[0][i]
		}

		documents = append(documents, doc)
	}

	slog.InfoContext(ctx, "Document search completed", "results_count", len(documents))
	return documents, nil
}

// SearchEntities performs semantic search for entities
func (s *ChromaStorageManager) SearchEntities(ctx context.Context, query string, limit int) ([]*types.Entity, error) {
	slog.DebugContext(ctx, "Searching entities", "query", query, "limit", limit)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.entityCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection ID for %s: %w", s.entityCollection, err)
	}

	// Generate embedding for the query
	embedding, err := s.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in ChromaDB
	request := interfaces.QueryRequest{
		QueryEmbeddings: [][]float64{embedding},
		NResults:        limit,
		Include:         []string{"documents", "metadatas", "distances"},
	}

	response, err := s.client.QueryDocuments(ctx, collectionID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities in ChromaDB: %w", err)
	}

	if len(response.IDs) == 0 {
		return []*types.Entity{}, nil
	}

	// Convert results to entities
	entities := make([]*types.Entity, 0, len(response.IDs[0]))
	for i, id := range response.IDs[0] {
		entity, err := s.convertToEntity(id, response.Documents[0][i], response.Metadatas[0][i])
		if err != nil {
			slog.WarnContext(ctx, "Failed to convert search result", "entity_id", id, "error", err)
			continue
		}

		// Add search score
		if len(response.Distances[0]) > i {
			if entity.Metadata == nil {
				entity.Metadata = make(map[string]any)
			}
			entity.Metadata["search_score"] = response.Distances[0][i]
		}

		entities = append(entities, entity)
	}

	slog.InfoContext(ctx, "Entity search completed", "results_count", len(entities))
	return entities, nil
}

// SearchCitations performs semantic search for citations
func (s *ChromaStorageManager) SearchCitations(ctx context.Context, query string, limit int) ([]*types.Citation, error) {
	slog.DebugContext(ctx, "Searching citations", "query", query, "limit", limit)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.citationCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection ID for %s: %w", s.citationCollection, err)
	}

	// Generate embedding for the query
	embedding, err := s.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in ChromaDB
	request := interfaces.QueryRequest{
		QueryEmbeddings: [][]float64{embedding},
		NResults:        limit,
		Include:         []string{"documents", "metadatas", "distances"},
	}

	response, err := s.client.QueryDocuments(ctx, collectionID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to search citations in ChromaDB: %w", err)
	}

	if len(response.IDs) == 0 {
		return []*types.Citation{}, nil
	}

	// Convert results to citations
	citations := make([]*types.Citation, 0, len(response.IDs[0]))
	for i, id := range response.IDs[0] {
		citation, err := s.convertToCitation(id, response.Documents[0][i], response.Metadatas[0][i])
		if err != nil {
			slog.WarnContext(ctx, "Failed to convert search result", "citation_id", id, "error", err)
			continue
		}

		// Add search score
		if len(response.Distances[0]) > i {
			if citation.Metadata == nil {
				citation.Metadata = make(map[string]any)
			}
			citation.Metadata["search_score"] = response.Distances[0][i]
		}

		citations = append(citations, citation)
	}

	slog.InfoContext(ctx, "Citation search completed", "results_count", len(citations))
	return citations, nil
}

// SearchTopics performs semantic search for topics
func (s *ChromaStorageManager) SearchTopics(ctx context.Context, query string, limit int) ([]*types.Topic, error) {
	slog.DebugContext(ctx, "Searching topics", "query", query, "limit", limit)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.topicCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection ID for %s: %w", s.topicCollection, err)
	}

	// Generate embedding for the query
	embedding, err := s.llmClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search in ChromaDB
	request := interfaces.QueryRequest{
		QueryEmbeddings: [][]float64{embedding},
		NResults:        limit,
		Include:         []string{"documents", "metadatas", "distances"},
	}

	response, err := s.client.QueryDocuments(ctx, collectionID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to search topics in ChromaDB: %w", err)
	}

	if len(response.IDs) == 0 {
		return []*types.Topic{}, nil
	}

	// Convert results to topics
	topics := make([]*types.Topic, 0, len(response.IDs[0]))
	for i, id := range response.IDs[0] {
		topic, err := s.convertToTopic(id, response.Documents[0][i], response.Metadatas[0][i])
		if err != nil {
			slog.WarnContext(ctx, "Failed to convert search result", "topic_id", id, "error", err)
			continue
		}

		// Add search score
		if len(response.Distances[0]) > i {
			if topic.Metadata == nil {
				topic.Metadata = make(map[string]any)
			}
			topic.Metadata["search_score"] = response.Distances[0][i]
		}

		topics = append(topics, topic)
	}

	slog.InfoContext(ctx, "Topic search completed", "results_count", len(topics))
	return topics, nil
}

// StoreEntity stores an entity in the entity collection
func (s *ChromaStorageManager) StoreEntity(ctx context.Context, entity *types.Entity) error {
	slog.DebugContext(ctx, "Storing entity",
		"entity_id", entity.ID,
		"entity_type", entity.Type,
		"entity_text", entity.Text,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.entityCollection)
	if err != nil {
		return fmt.Errorf("failed to get collection ID for %s: %w", s.entityCollection, err)
	}

	// Generate embedding for the entity text
	embedding, err := s.llmClient.Embed(ctx, entity.Text)
	if err != nil {
		return fmt.Errorf("failed to generate entity embedding: %w", err)
	}

	// Prepare metadata
	metadata := map[string]any{
		"type":         string(entity.Type),
		"document_id":  entity.DocumentID,
		"start_offset": entity.StartOffset,
		"end_offset":   entity.EndOffset,
		"confidence":   entity.Confidence,
		"extracted_at": entity.ExtractedAt.Format(time.RFC3339),
		"method":       string(entity.Method),
	}

	// Add custom metadata
	for k, v := range entity.Metadata {
		metadata[k] = v
	}

	// Store in ChromaDB
	request := interfaces.AddDocumentsRequest{
		IDs:        []string{entity.ID},
		Embeddings: [][]float64{embedding},
		Documents:  []string{entity.Text},
		Metadatas:  []map[string]any{metadata},
	}

	if err := s.client.AddDocuments(ctx, collectionID, request); err != nil {
		return fmt.Errorf("failed to store entity in ChromaDB: %w", err)
	}

	slog.InfoContext(ctx, "Entity stored successfully", "entity_id", entity.ID)
	return nil
}

// StoreCitation stores a citation in the citation collection
func (s *ChromaStorageManager) StoreCitation(ctx context.Context, citation *types.Citation) error {
	slog.DebugContext(ctx, "Storing citation",
		"citation_id", citation.ID,
		"citation_format", citation.Format,
		"citation_text", citation.Text,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.citationCollection)
	if err != nil {
		return fmt.Errorf("failed to get collection ID for %s: %w", s.citationCollection, err)
	}

	// Generate embedding for the citation text
	embedding, err := s.llmClient.Embed(ctx, citation.Text)
	if err != nil {
		return fmt.Errorf("failed to generate citation embedding: %w", err)
	}

	// Prepare metadata
	metadata := map[string]any{
		"format":       string(citation.Format),
		"document_id":  citation.DocumentID,
		"start_offset": citation.StartOffset,
		"end_offset":   citation.EndOffset,
		"confidence":   citation.Confidence,
		"extracted_at": citation.ExtractedAt.Format(time.RFC3339),
		"method":       string(citation.Method),
	}

	// Add structured data (convert arrays to strings for ChromaDB v2 compatibility)
	if citation.Authors != nil {
		metadata["authors"] = strings.Join(citation.Authors, "; ")
	}
	if citation.Title != nil && *citation.Title != "" {
		metadata["title"] = *citation.Title
	}
	if citation.Year != nil && *citation.Year != 0 {
		metadata["year"] = *citation.Year
	}
	if citation.DOI != nil && *citation.DOI != "" {
		metadata["doi"] = *citation.DOI
	}
	if citation.URL != nil && *citation.URL != "" {
		metadata["url"] = *citation.URL
	}

	// Add custom metadata
	for k, v := range citation.Metadata {
		metadata[k] = v
	}

	// Store in ChromaDB
	request := interfaces.AddDocumentsRequest{
		IDs:        []string{citation.ID},
		Embeddings: [][]float64{embedding},
		Documents:  []string{citation.Text},
		Metadatas:  []map[string]any{metadata},
	}

	if err := s.client.AddDocuments(ctx, collectionID, request); err != nil {
		return fmt.Errorf("failed to store citation in ChromaDB: %w", err)
	}

	slog.InfoContext(ctx, "Citation stored successfully", "citation_id", citation.ID)
	return nil
}

// StoreTopic stores a topic in the topic collection
func (s *ChromaStorageManager) StoreTopic(ctx context.Context, topic *types.Topic) error {
	slog.DebugContext(ctx, "Storing topic",
		"topic_id", topic.ID,
		"topic_name", topic.Name,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, s.topicCollection)
	if err != nil {
		return fmt.Errorf("failed to get collection ID for %s: %w", s.topicCollection, err)
	}

	// Generate embedding for the topic name and description
	content := topic.Name
	if topic.Description != "" {
		content = fmt.Sprintf("%s: %s", topic.Name, topic.Description)
	}

	embedding, err := s.llmClient.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate topic embedding: %w", err)
	}

	// Prepare metadata
	metadata := map[string]any{
		"name":         topic.Name,
		"description":  topic.Description,
		"document_id":  topic.DocumentID,
		"confidence":   topic.Confidence,
		"weight":       topic.Weight,
		"extracted_at": topic.ExtractedAt.Format(time.RFC3339),
		"method":       topic.Method,
	}

	// Add keywords as comma-separated string (ChromaDB v2 doesn't support array metadata)
	if len(topic.Keywords) > 0 {
		metadata["keywords"] = strings.Join(topic.Keywords, ", ")
	}

	// Add custom metadata
	for k, v := range topic.Metadata {
		metadata[k] = v
	}

	// Store in ChromaDB
	request := interfaces.AddDocumentsRequest{
		IDs:        []string{topic.ID},
		Embeddings: [][]float64{embedding},
		Documents:  []string{content},
		Metadatas:  []map[string]any{metadata},
	}

	if err := s.client.AddDocuments(ctx, collectionID, request); err != nil {
		return fmt.Errorf("failed to store topic in ChromaDB: %w", err)
	}

	slog.InfoContext(ctx, "Topic stored successfully", "topic_id", topic.ID)
	return nil
}

// DeleteDocument removes a document from the default collection
func (s *ChromaStorageManager) DeleteDocument(ctx context.Context, id string) error {
	return s.DeleteDocumentFromCollection(ctx, s.defaultCollection, id)
}

// DeleteDocumentFromCollection removes a document from a specific collection
func (s *ChromaStorageManager) DeleteDocumentFromCollection(ctx context.Context, collection, id string) error {
	slog.DebugContext(ctx, "Deleting document",
		"document_id", id,
		"collection", collection,
	)

	// Get collection ID from name
	collectionID, err := s.getCollectionID(ctx, collection)
	if err != nil {
		return fmt.Errorf("failed to get collection ID for %s: %w", collection, err)
	}

	if err := s.client.DeleteDocuments(ctx, collectionID, []string{id}); err != nil {
		return fmt.Errorf("failed to delete document from ChromaDB: %w", err)
	}

	slog.InfoContext(ctx, "Document deleted successfully", "document_id", id)
	return nil
}

// ListCollections returns all available collections
func (s *ChromaStorageManager) ListCollections(ctx context.Context) ([]interfaces.CollectionInfo, error) {
	response, err := s.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	result := make([]interfaces.CollectionInfo, len(response.Collections))
	for i, coll := range response.Collections {
		count, err := s.client.CountDocuments(ctx, coll.ID)
		if err != nil {
			slog.WarnContext(ctx, "Failed to get document count", "collection", coll.Name, "error", err)
			count = 0
		}

		result[i] = interfaces.CollectionInfo{
			Name:      coll.Name,
			Metadata:  coll.Metadata,
			Documents: count,
		}
	}

	return result, nil
}

// GetStats returns storage statistics
func (s *ChromaStorageManager) GetStats(ctx context.Context) (*interfaces.StorageStats, error) {
	collections, err := s.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collections for stats: %w", err)
	}

	stats := &interfaces.StorageStats{
		Collections: collections,
		Metrics: map[string]any{
			"total_collections": len(collections),
			"backend":           "chromadb",
			"embedding_dim":     s.embeddingDimension,
		},
	}

	totalDocs := 0
	for _, coll := range collections {
		totalDocs += coll.Documents
	}
	stats.Metrics["total_documents"] = totalDocs

	return stats, nil
}

// Close closes the storage manager
func (s *ChromaStorageManager) Close() error {
	slog.Info("Closing ChromaDB storage manager")
	return nil
}

// getCollectionID retrieves the collection ID for a given collection name
func (s *ChromaStorageManager) getCollectionID(ctx context.Context, name string) (string, error) {
	// Check cache first
	if id, exists := s.collectionIDs[name]; exists {
		return id, nil
	}

	// Try to find the collection by iterating through all collections
	// Note: v2 API doesn't have a direct name->ID lookup, so we need to list and match
	response, err := s.client.ListCollections(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list collections: %w", err)
	}

	for _, coll := range response.Collections {
		if coll.Name == name {
			s.collectionIDs[name] = coll.ID
			return coll.ID, nil
		}
	}

	return "", fmt.Errorf("collection %s not found", name)
}

// ensureCollection creates a collection if it doesn't exist and caches its ID
func (s *ChromaStorageManager) ensureCollection(ctx context.Context, name, description string) error {
	// Try to get the collection ID from cache first
	if _, exists := s.collectionIDs[name]; exists {
		slog.DebugContext(ctx, "Collection ID already cached", "collection", name)
		return nil
	}

	// Try to find existing collection
	response, err := s.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	// Check if collection already exists
	for _, coll := range response.Collections {
		if coll.Name == name {
			s.collectionIDs[name] = coll.ID
			slog.DebugContext(ctx, "Collection already exists", "collection", name, "id", coll.ID)
			return nil
		}
	}

	// If collection doesn't exist, create it
	metadata := map[string]any{
		"description": description,
		"created_at":  time.Now().Format(time.RFC3339),
	}

	collection, err := s.client.CreateCollection(ctx, name, metadata)
	if err != nil {
		// Check if it was created by another process (race condition)
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "unique constraint") {
			// Try to get the ID again
			if id, getErr := s.getCollectionID(ctx, name); getErr == nil {
				s.collectionIDs[name] = id
				slog.DebugContext(ctx, "Collection was created by another process", "collection", name, "id", id)
				return nil
			}
		}
		return fmt.Errorf("failed to create collection %s: %w", name, err)
	}

	// Cache the new collection ID
	s.collectionIDs[name] = collection.ID
	slog.InfoContext(ctx, "Collection created successfully", "collection", name, "id", collection.ID)
	return nil
}

// convertToDocument converts ChromaDB response data to a Document
func (s *ChromaStorageManager) convertToDocument(id, content string, metadata map[string]any) (*types.Document, error) {
	doc := &types.Document{
		ID:      id,
		Content: content,
	}

	// Extract known metadata fields
	if path, ok := metadata["path"].(string); ok {
		doc.Path = path
	}
	if name, ok := metadata["name"].(string); ok {
		doc.Name = name
	}
	if contentType, ok := metadata["content_type"].(string); ok {
		doc.ContentType = contentType
	}
	if createdAt, ok := metadata["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			doc.CreatedAt = t
		}
	}
	if updatedAt, ok := metadata["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}
	}

	// Store all metadata
	doc.Metadata = make(map[string]any)
	for k, v := range metadata {
		doc.Metadata[k] = v
	}

	return doc, nil
}

// convertToEntity converts ChromaDB response data to an Entity
func (s *ChromaStorageManager) convertToEntity(id, content string, metadata map[string]any) (*types.Entity, error) {
	entity := &types.Entity{
		ID:   id,
		Text: content,
	}

	// Extract known metadata fields
	if entityType, ok := metadata["type"].(string); ok {
		entity.Type = types.EntityType(entityType)
	}
	if confidence, ok := metadata["confidence"].(float64); ok {
		entity.Confidence = confidence
	}
	if startOffset, ok := metadata["start_offset"].(float64); ok {
		entity.StartOffset = int(startOffset)
	}
	if endOffset, ok := metadata["end_offset"].(float64); ok {
		entity.EndOffset = int(endOffset)
	}
	if documentID, ok := metadata["document_id"].(string); ok {
		entity.DocumentID = documentID
	}
	if method, ok := metadata["method"].(string); ok {
		entity.Method = types.ExtractorMethod(method)
	}
	if extractedAt, ok := metadata["extracted_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, extractedAt); err == nil {
			entity.ExtractedAt = t
		}
	}

	// Store all metadata
	entity.Metadata = make(map[string]any)
	for k, v := range metadata {
		entity.Metadata[k] = v
	}

	return entity, nil
}

// convertToCitation converts ChromaDB response data to a Citation
func (s *ChromaStorageManager) convertToCitation(id, content string, metadata map[string]any) (*types.Citation, error) {
	citation := &types.Citation{
		ID:   id,
		Text: content,
	}

	// Extract known metadata fields
	if authors, ok := metadata["authors"].(string); ok {
		// Authors stored as comma-separated string
		if authors != "" {
			citation.Authors = strings.Split(authors, ", ")
		}
	}
	if title, ok := metadata["title"].(string); ok && title != "" {
		citation.Title = &title
	}
	if year, ok := metadata["year"].(float64); ok && year != 0 {
		yearInt := int(year)
		citation.Year = &yearInt
	}
	if journal, ok := metadata["journal"].(string); ok && journal != "" {
		citation.Journal = &journal
	}
	if volume, ok := metadata["volume"].(string); ok && volume != "" {
		citation.Volume = &volume
	}
	if issue, ok := metadata["issue"].(string); ok && issue != "" {
		citation.Issue = &issue
	}
	if pages, ok := metadata["pages"].(string); ok && pages != "" {
		citation.Pages = &pages
	}
	if publisher, ok := metadata["publisher"].(string); ok && publisher != "" {
		citation.Publisher = &publisher
	}
	if url, ok := metadata["url"].(string); ok && url != "" {
		citation.URL = &url
	}
	if doi, ok := metadata["doi"].(string); ok && doi != "" {
		citation.DOI = &doi
	}
	if format, ok := metadata["format"].(string); ok {
		citation.Format = types.CitationFormat(format)
	}
	if confidence, ok := metadata["confidence"].(float64); ok {
		citation.Confidence = confidence
	}
	if startOffset, ok := metadata["start_offset"].(float64); ok {
		citation.StartOffset = int(startOffset)
	}
	if endOffset, ok := metadata["end_offset"].(float64); ok {
		citation.EndOffset = int(endOffset)
	}
	if documentID, ok := metadata["document_id"].(string); ok {
		citation.DocumentID = documentID
	}
	if method, ok := metadata["method"].(string); ok {
		citation.Method = types.ExtractorMethod(method)
	}
	if extractedAt, ok := metadata["extracted_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, extractedAt); err == nil {
			citation.ExtractedAt = t
		}
	}

	// Store all metadata
	citation.Metadata = make(map[string]any)
	for k, v := range metadata {
		citation.Metadata[k] = v
	}

	return citation, nil
}

// convertToTopic converts ChromaDB response data to a Topic
func (s *ChromaStorageManager) convertToTopic(id, content string, metadata map[string]any) (*types.Topic, error) {
	topic := &types.Topic{
		ID:   id,
		Name: content, // The content is the topic name
	}

	// Extract known metadata fields
	if keywords, ok := metadata["keywords"].(string); ok {
		// Keywords stored as comma-separated string
		if keywords != "" {
			topic.Keywords = strings.Split(keywords, ", ")
		}
	}
	if confidence, ok := metadata["confidence"].(float64); ok {
		topic.Confidence = confidence
	}
	if weight, ok := metadata["weight"].(float64); ok {
		topic.Weight = weight
	}
	if description, ok := metadata["description"].(string); ok {
		topic.Description = description
	}
	if documentID, ok := metadata["document_id"].(string); ok {
		topic.DocumentID = documentID
	}
	if method, ok := metadata["method"].(string); ok {
		topic.Method = method
	}
	if extractedAt, ok := metadata["extracted_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, extractedAt); err == nil {
			topic.ExtractedAt = t
		}
	}

	// Store all metadata
	topic.Metadata = make(map[string]any)
	for k, v := range metadata {
		topic.Metadata[k] = v
	}

	return topic, nil
}

// Health checks if the storage manager is healthy
func (s *ChromaStorageManager) Health(ctx context.Context) error {
	// Check ChromaDB health
	if err := s.client.Health(ctx); err != nil {
		return fmt.Errorf("ChromaDB health check failed: %w", err)
	}
	return nil
}
