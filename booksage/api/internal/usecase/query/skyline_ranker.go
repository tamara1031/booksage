package query

import (
	"log"
	"sort"

	"github.com/booksage/booksage-api/internal/domain"
)

// SkylineRanker implements Pareto-optimal fusion (BookRAG).
type SkylineRanker struct{}

// Rank performs Skyline ranking on search results.
// It prioritizes results that are not "dominated" by others in both axes:
// Axis 1: Vector Score (from ColBERT/Infinity reranker)
// Axis 2: Graph Relevance (Structure)
func (r *SkylineRanker) Rank(results []domain.SearchResult) []domain.SearchResult {
	if len(results) <= 1 {
		return results
	}

	type enrichedResult struct {
		domain.SearchResult
		VectorScore float32
		GraphScore  float32
	}

	enriched := make([]enrichedResult, len(results))
	for i, res := range results {
		vScore := res.Score    // Reranked score (high fidelity)
		gScore := float32(0.5) // Default graph relevance

		// Heuristic: If source is graph, boost graph score
		if res.Source == "graph" {
			gScore = 0.9
		} else if res.Source == "vector" {
			// Vector results have high semantic score but lower structural score
			gScore = 0.4
		}

		enriched[i] = enrichedResult{
			SearchResult: res,
			VectorScore:  vScore,
			GraphScore:   gScore,
		}
	}

	// Skyline Filter: keeps nodes that are not strictly worse than another node in BOTH scores.
	var skyline []domain.SearchResult
	for i := 0; i < len(enriched); i++ {
		dominated := false
		for j := 0; j < len(enriched); j++ {
			if i == j {
				continue
			}
			// Node J dominates I if J is better in both axes
			if enriched[j].VectorScore > enriched[i].VectorScore && enriched[j].GraphScore > enriched[i].GraphScore {
				dominated = true
				break
			}
		}
		if !dominated {
			skyline = append(skyline, enriched[i].SearchResult)
		}
	}

	// Sort skyline by Vector Score for final generation priority
	sort.Slice(skyline, func(i, j int) bool {
		return skyline[i].Score > skyline[j].Score
	})

	log.Printf("[Skyline] Filtered %d nodes down to %d Pareto-optimal nodes", len(results), len(skyline))
	return skyline
}
