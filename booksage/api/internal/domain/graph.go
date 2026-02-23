package domain

import (
	"context"
)

// Entity represents a named entity extracted from text (LightRAG)
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Relation represents a relationship between two entities (LightRAG)
type Relation struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Description string `json:"description"`
}

// GraphRepository handles structured knowledge graph storage and retrieval.
type GraphRepository interface {
	InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error
	SearchChunks(ctx context.Context, query string, limit int) ([]SearchResult, error)
	DeleteDocument(ctx context.Context, docID string) error
	Close(ctx context.Context) error
}
