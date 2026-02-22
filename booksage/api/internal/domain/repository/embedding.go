package repository

import (
	"context"
)

// EmbeddingClient defines the interface for generating embeddings from text.
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
}
