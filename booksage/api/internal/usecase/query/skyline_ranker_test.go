package query

import (
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

func TestSkylineRanker_Pareto(t *testing.T) {
	// Arrange
	ranker := &SkylineRanker{}
	results := []repository.SearchResult{
		{ID: "best_vector", Score: 0.99, Source: "vector"}, // High Vector, Low Graph (default 0.5)
		{ID: "best_graph", Score: 0.7, Source: "graph"},    // Lower Vector, High Graph (0.8)
		{ID: "dominated", Score: 0.1, Source: "vector"},    // Low Vector, Low Graph -> Should be removed
		{ID: "also_good", Score: 0.85, Source: "vector"},   // Mid Vector, Low Graph
	}

	// Act
	ranked := ranker.Rank(results)

	// Assert
	// "dominated" is clearly worse than "best_vector" in both simulated axes.
	// axis 1 (vector): best_vector (0.99*1.2=1.18) vs dominated (0.1*1.2=0.12)
	// axis 2 (graph): best_vector (0.5) vs dominated (0.5) [strictly better or equal]

	foundDominated := false
	for _, r := range ranked {
		if r.ID == "dominated" {
			foundDominated = true
		}
	}

	if foundDominated {
		t.Errorf("Skyline should have filtered out the dominated node")
	}

	if len(ranked) == 0 {
		t.Error("Should have returned non-dominated results")
	}
}

func TestSkylineRanker_Empty(t *testing.T) {
	ranker := &SkylineRanker{}

	// Act & Assert
	if len(ranker.Rank(nil)) != 0 {
		t.Error("Expected empty slice for nil input")
	}
}
