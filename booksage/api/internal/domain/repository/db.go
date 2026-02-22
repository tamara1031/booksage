package repository

import (
	"context"
)

// SearchResult represents a generic search result from any engine.
type SearchResult struct {
	ID      string
	Content string
	Score   float32
	Source  string // "vector", "graph", "tree", etc.
}

// VectorRepository defines the interface for vector database operations.
type VectorRepository interface {
	Search(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
	InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error
	DeleteDocument(ctx context.Context, docID string) error
	Close() error
}

// GraphRepository defines the interface for graph database operations.
type GraphRepository interface {
	SearchChunks(ctx context.Context, query string, limit int) ([]SearchResult, error)
	InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error
	DeleteDocument(ctx context.Context, docID string) error
	Close(ctx context.Context) error
}
