package query

import (
	"testing"

	"github.com/booksage/booksage-api/internal/domain"
)

func TestSkylineRanker_Rank(t *testing.T) {
	ranker := &SkylineRanker{}

	tests := []struct {
		name     string
		input    []domain.SearchResult
		expected []string // IDs of expected results in order
	}{
		{
			name:     "Empty input",
			input:    []domain.SearchResult{},
			expected: []string{},
		},
		{
			name: "Single result",
			input: []domain.SearchResult{
				{ID: "1", Content: "A", Score: 0.9, Source: "vector"},
			},
			expected: []string{"1"},
		},
		{
			name: "No Dominance (Equal Graph Score)",
			// Node 1: Vector=0.9, Source="vector" => V=0.9, G=0.4
			// Node 2: Vector=0.95, Source="vector" => V=0.95, G=0.4
			// Node 2 better V, equal G. Not strictly dominating.
			input: []domain.SearchResult{
				{ID: "1", Score: 0.9, Source: "vector"},
				{ID: "2", Score: 0.95, Source: "vector"},
			},
			expected: []string{"2", "1"}, // Sorted by score descending
		},
		{
			name: "Strictly Dominated (Vector < Graph)",
			// Node 1: Vector=0.8, Source="vector" => V=0.8, G=0.4
			// Node 2: Vector=0.9, Source="graph" => V=0.9, G=0.9
			// Node 2 has V > Node 1.V (0.9 > 0.8) AND G > Node 1.G (0.9 > 0.4).
			// So Node 1 is dominated by Node 2.
			input: []domain.SearchResult{
				{ID: "1", Score: 0.8, Source: "vector"},
				{ID: "2", Score: 0.9, Source: "graph"},
			},
			expected: []string{"2"},
		},
		{
			name: "Pareto Front (Trade-off)",
			// Node 1: Vector=0.9, Source="vector" => V=0.9, G=0.4
			// Node 2: Vector=0.5, Source="graph" => V=0.5, G=0.9
			// Node 1 better V, Node 2 better G. Neither dominates.
			input: []domain.SearchResult{
				{ID: "1", Score: 0.9, Source: "vector"},
				{ID: "2", Score: 0.5, Source: "graph"},
			},
			expected: []string{"1", "2"}, // Sorted by score (0.9 > 0.5)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ranker.Rank(tt.input)

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d results, got %d", len(tt.expected), len(got))
			}

			for i, id := range tt.expected {
				if got[i].ID != id {
					t.Errorf("expected result %d to be %s, got %s", i, id, got[i].ID)
				}
			}
		})
	}
}
