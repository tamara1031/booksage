package domain

import (
	"context"
)

// SearchResult represents a single hit from any retrieval engine
type SearchResult struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
	Source  string  `json:"source"` // "vector" or "graph"
}

// VectorRepository handles dense vector storage and retrieval.
type VectorRepository interface {
	InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error
	Search(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
	DeleteDocument(ctx context.Context, docID string) error
	Close() error
}
