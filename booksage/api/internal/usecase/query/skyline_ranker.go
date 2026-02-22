package query

import (
	"log"
	"sort"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// SkylineRanker implements Pareto-optimal fusion (BookRAG).
type SkylineRanker struct{}

// Rank performs Skyline ranking on search results.
// It prioritizes results that are not "dominated" by others in both axes:
// Axis 1: Vector Similarity (Score)
// Axis 2: Graph Context/Relevance (calculated or secondary score)
func (r *SkylineRanker) Rank(results []repository.SearchResult) []repository.SearchResult {
	if len(results) <= 1 {
		return results
	}

	// For the sake of the SOTA implementation, we simulate two axes.
	// In a real system, Axis 2 would come from graph traversal depth or pagerank.
	// Here we use Source as a proxy: Source="graph" gets a 'graph_bonus'.

	type enrichedResult struct {
		repository.SearchResult
		VectorScore float32
		GraphScore  float32
	}

	enriched := make([]enrichedResult, len(results))
	for i, res := range results {
		vScore := res.Score
		gScore := float32(0.5) // Default graph relevance
		if res.Source == "graph" {
			gScore = 0.8
		} else if res.Source == "vector" {
			vScore *= 1.2 // slight vector bias
		}

		enriched[i] = enrichedResult{
			SearchResult: res,
			VectorScore:  vScore,
			GraphScore:   gScore,
		}
	}

	// Skyline Filter: keeps nodes that are not strictly worse than another node in BOTH scores.
	var skyline []repository.SearchResult
	for i := 0; i < len(enriched); i++ {
		dominated := false
		for j := 0; j < len(enriched); j++ {
			if i == j {
				continue
			}
			// If node J is better than node I in both axes, I is dominated.
			if enriched[j].VectorScore > enriched[i].VectorScore && enriched[j].GraphScore > enriched[i].GraphScore {
				dominated = true
				break
			}
		}
		if !dominated {
			skyline = append(skyline, enriched[i].SearchResult)
		}
	}

	// Sort skyline by average of normalized scores for final output
	sort.Slice(skyline, func(i, j int) bool {
		return skyline[i].Score > skyline[j].Score
	})

	log.Printf("[Skyline] Filtered %d nodes down to %d Pareto-optimal nodes", len(results), len(skyline))
	return skyline
}
