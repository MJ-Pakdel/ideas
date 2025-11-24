package storage

import (
	"context"
	"fmt"

	"github.com/example/idaes/internal/clients"
	"github.com/example/idaes/internal/interfaces"
)

// ChromaDBClientAdapter adapts the concrete ChromaDBClient to the interface
type ChromaDBClientAdapter struct {
	client *clients.ChromaDBClient
}

// NewChromaDBClientAdapter creates a new adapter
func NewChromaDBClientAdapter(client *clients.ChromaDBClient) interfaces.ChromaDBClient {
	return &ChromaDBClientAdapter{client: client}
}

// Health checks if the ChromaDB service is healthy
func (a *ChromaDBClientAdapter) Health(ctx context.Context) error {
	return a.client.Health(ctx)
}

// ListCollections lists all collections
func (a *ChromaDBClientAdapter) ListCollections(ctx context.Context) (*interfaces.ListCollectionsResponse, error) {
	clientResp, err := a.client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	// Convert from []clients.Collection to ListCollectionsResponse
	var collections []*interfaces.Collection
	for _, coll := range clientResp {
		collections = append(collections, &interfaces.Collection{
			ID:       coll.ID,
			Name:     coll.Name,
			Metadata: coll.Metadata,
		})
	}

	return &interfaces.ListCollectionsResponse{
		Collections: collections,
	}, nil
}

// CreateCollection creates a new collection
func (a *ChromaDBClientAdapter) CreateCollection(ctx context.Context, name string, metadata map[string]any) (*interfaces.Collection, error) {
	clientColl, err := a.client.CreateCollection(ctx, name, metadata)
	if err != nil {
		return nil, err
	}

	// Convert from clients.Collection to interfaces.Collection
	return &interfaces.Collection{
		ID:       clientColl.ID,
		Name:     clientColl.Name,
		Metadata: clientColl.Metadata,
	}, nil
}

// AddDocuments adds documents to a collection
func (a *ChromaDBClientAdapter) AddDocuments(ctx context.Context, collectionID string, request interfaces.AddDocumentsRequest) error {
	// Convert from interfaces.AddDocumentsRequest to clients.AddDocumentsRequest
	clientReq := clients.AddDocumentsRequest{
		IDs:        request.IDs,
		Documents:  request.Documents,
		Metadatas:  request.Metadatas,
		Embeddings: request.Embeddings,
	}

	return a.client.AddDocuments(ctx, collectionID, clientReq)
}

// GetDocuments retrieves documents from a collection
func (a *ChromaDBClientAdapter) GetDocuments(ctx context.Context, collectionID string, ids []string, include []string) (*interfaces.GetDocumentsResponse, error) {
	clientResp, err := a.client.GetDocuments(ctx, collectionID, ids, include)
	if err != nil {
		return nil, err
	}

	// Convert from clients.GetDocumentsResponse to interfaces.GetDocumentsResponse
	return &interfaces.GetDocumentsResponse{
		IDs:        clientResp.IDs,
		Documents:  clientResp.Documents,
		Metadatas:  clientResp.Metadatas,
		Embeddings: clientResp.Embeddings,
	}, nil
}

// QueryDocuments queries documents in a collection
func (a *ChromaDBClientAdapter) QueryDocuments(ctx context.Context, collectionID string, request interfaces.QueryRequest) (*interfaces.QueryResponse, error) {
	// Convert from interfaces.QueryRequest to clients.QueryRequest
	clientReq := clients.QueryRequest{
		QueryEmbeddings: request.QueryEmbeddings,
		NResults:        request.NResults,
		Include:         request.Include,
	}

	clientResp, err := a.client.QueryDocuments(ctx, collectionID, clientReq)
	if err != nil {
		return nil, err
	}

	// Convert from clients.QueryResponse to interfaces.QueryResponse
	return &interfaces.QueryResponse{
		IDs:       clientResp.IDs,
		Documents: clientResp.Documents,
		Metadatas: clientResp.Metadatas,
		Distances: clientResp.Distances,
	}, nil
}

// CountDocuments counts documents in a collection
func (a *ChromaDBClientAdapter) CountDocuments(ctx context.Context, collectionID string) (int, error) {
	return a.client.CountDocuments(ctx, collectionID)
}

// Additional interface methods required by ChromaDBClient

// Healthcheck is an alias for Health for interface compatibility
func (a *ChromaDBClientAdapter) Healthcheck(ctx context.Context) error {
	return a.client.Health(ctx)
}

// Version returns the ChromaDB version
func (a *ChromaDBClientAdapter) Version(ctx context.Context) (string, error) {
	// The concrete client may not have this method, so return a default
	return "unknown", nil
}

// GetCollection gets a collection by name
func (a *ChromaDBClientAdapter) GetCollection(ctx context.Context, name string) (*interfaces.Collection, error) {
	// This method may not exist in the concrete client, return an error for now
	return nil, fmt.Errorf("GetCollection not implemented in adapter")
}

// DeleteCollection deletes a collection
func (a *ChromaDBClientAdapter) DeleteCollection(ctx context.Context, name string) error {
	return a.client.DeleteCollection(ctx, name)
}

// DeleteDocuments deletes documents from a collection
func (a *ChromaDBClientAdapter) DeleteDocuments(ctx context.Context, collectionID string, ids []string) error {
	return a.client.DeleteDocuments(ctx, collectionID, ids)
}

// Close closes the client connection
func (a *ChromaDBClientAdapter) Close() error {
	// The HTTP client doesn't need explicit closing
	return nil
}
