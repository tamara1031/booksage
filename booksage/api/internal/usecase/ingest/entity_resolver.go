package ingest

import (
	"context"
	"log"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// EntityResolver handles entity linking and resolution.
type EntityResolver struct {
	vectorStore repository.VectorRepository
	embedder    repository.EmbeddingClient
}

// NewEntityResolver creates a new entity resolver.
func NewEntityResolver(v repository.VectorRepository, e repository.EmbeddingClient) *EntityResolver {
	return &EntityResolver{
		vectorStore: v,
		embedder:    e,
	}
}

// ResolveEntity attempts to find a similar existing entity.
// It returns the ID of the matched entity, a boolean indicating if a match was found, and any error.
func (r *EntityResolver) ResolveEntity(ctx context.Context, ent Entity) (string, bool, error) {
	if r.embedder == nil {
		return "", false, nil
	}

	vecs, err := r.embedder.Embed(ctx, []string{ent.Name})
	if err != nil {
		return "", false, err
	}
	if len(vecs) > 0 {
		matches, err := r.vectorStore.Search(ctx, vecs[0], 1)
		if err != nil {
			return "", false, err
		}
		if len(matches) > 0 && matches[0].Score > 0.9 {
			log.Printf("[EntityResolver] Matched '%s' to existing %s", ent.Name, matches[0].ID)
			return matches[0].ID, true, nil
		}
	}
	return "", false, nil
}
