package query

import (
	"context"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

func TestRetrieve_NilClients(t *testing.T) {
	// Passing nils to verify graceful degradation
	retriever := NewFusionRetriever(nil, nil, nil, nil)

	ctx := context.Background()
	results, err := retriever.Retrieve(ctx, "test query")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	// Skyline filter might return empty if no axes are available
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestSkylineRanker(t *testing.T) {
	ranker := &SkylineRanker{}

	results := []repository.SearchResult{
		{ID: "1", Content: "A", Score: 0.9, Source: "vector"},
		{ID: "2", Content: "B", Score: 0.8, Source: "graph"},
		{ID: "3", Content: "C", Score: 0.5, Source: "vector"},
	}

	fused := ranker.Rank(results)
	if len(fused) == 0 {
		t.Fatal("Expected skyline nodes, got 0")
	}
}
