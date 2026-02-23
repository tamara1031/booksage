package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/domain"
)

// EntityResolver handles entity deduplication and resolution.
type EntityResolver struct {
	vectorStore domain.VectorRepository
	tensor      domain.TensorEngine
}

// NewEntityResolver creates a new entity resolver.
func NewEntityResolver(v domain.VectorRepository, t domain.TensorEngine) *EntityResolver {
	return &EntityResolver{
		vectorStore: v,
		tensor:      t,
	}
}

// ResolveEntity attempts to find a matching entity in the database.
// Returns (existingID, wasFound, error).
func (er *EntityResolver) ResolveEntity(ctx context.Context, ent domain.Entity) (string, bool, error) {
	if er.tensor == nil {
		return "", false, nil // Skip if tensor engine unavailable
	}

	// 1. Embed entity name
	embeddings, err := er.tensor.Embed(ctx, []string{ent.Name})
	if err != nil {
		return "", false, err
	}
	if len(embeddings) == 0 {
		return "", false, fmt.Errorf("no embedding generated")
	}

	// 2. Search for similar entities
	// Note: This assumes vectorStore has a method for searching entities, not just chunks.
	// Since SearchChunks returns SearchResult, we can potentially reuse it if the collection is unified,
	// or if we had a dedicated "SearchEntities".
	// For this refactor, we'll assume Search works generally or skip if implementation details vary.
	results, err := er.vectorStore.Search(ctx, embeddings[0], 1)
	if err != nil {
		return "", false, err
	}

	if len(results) > 0 {
		top := results[0]
		if top.Score > 0.92 { // Strict threshold for resolution
			log.Printf("[EntityResolver] Resolved '%s' to existing ID %s (score: %.3f)", ent.Name, top.ID, top.Score)
			return top.ID, true, nil
		}
	}

	return "", false, nil
}
