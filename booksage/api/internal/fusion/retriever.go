package fusion

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/booksage/booksage-api/internal/embedding"
	neo4jpkg "github.com/booksage/booksage-api/internal/neo4j"
	qdrantpkg "github.com/booksage/booksage-api/internal/qdrant"
	"golang.org/x/sync/errgroup"
)

// SearchResult represents a common structure for results from different engines.
type SearchResult struct {
	ID      string
	Content string
	Score   float32
	Source  string // "graph", "tree", "vector"
}

// FusionRetriever manages concurrent retrieval across multiple data stores.
type FusionRetriever struct {
	qdrant   *qdrantpkg.Client
	neo4j    *neo4jpkg.Client
	embedder *embedding.Batcher
}

// NewFusionRetriever creates a new FusionRetriever with real DB clients.
func NewFusionRetriever(qdrant *qdrantpkg.Client, neo4j *neo4jpkg.Client, embedder *embedding.Batcher) *FusionRetriever {
	return &FusionRetriever{
		qdrant:   qdrant,
		neo4j:    neo4j,
		embedder: embedder,
	}
}

// Retrieve performs asynchronous parallel requests across engines and ensembles them.
func (f *FusionRetriever) Retrieve(ctx context.Context, query string) ([]SearchResult, error) {
	log.Printf("[Fusion] Starting parallel retrieval for: %s", query)

	// Add a global timeout for the entire fusion process.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var allResults []SearchResult

	// 1. Vector Engine (Qdrant Dense Search)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Vector Engine request...")
		docs, err := f.searchVectorDB(ctx, query)
		if err != nil {
			log.Printf("Warning: Vector DB search failed, degrading gracefully: %v", err)
			return nil
		}
		mu.Lock()
		allResults = append(allResults, docs...)
		mu.Unlock()
		return nil
	})

	// 2. Graph Engine (Neo4j)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Graph Engine request...")
		docs, err := f.searchGraphDB(ctx, query)
		if err != nil {
			log.Printf("Warning: Graph DB search failed, degrading gracefully: %v", err)
			return nil
		}
		mu.Lock()
		allResults = append(allResults, docs...)
		mu.Unlock()
		return nil
	})

	// 3. RAPTOR/Tree Engine (placeholder for Phase 3)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Tree/RAPTOR Engine request (stub)...")
		// RAPTOR tree search will be implemented in Phase 3.
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	log.Printf("[Fusion] Retrieval complete. Integrating %d total results via RRF...", len(allResults))
	return f.performRRF(allResults), nil
}

// searchVectorDB queries Qdrant using dense vector similarity.
func (f *FusionRetriever) searchVectorDB(ctx context.Context, query string) ([]SearchResult, error) {
	if f.qdrant == nil || f.embedder == nil {
		return nil, fmt.Errorf("qdrant or embedder not configured")
	}

	// Generate query embedding
	embResults, _, err := f.embedder.GenerateEmbeddingsBatched(ctx, []string{query}, "dense", "query")
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}
	if len(embResults) == 0 || embResults[0].GetDense() == nil {
		return nil, fmt.Errorf("no embedding result returned")
	}

	queryVector := embResults[0].GetDense().GetValues()

	// Search Qdrant
	qdrantResults, err := f.qdrant.Search(ctx, queryVector, 5)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, r := range qdrantResults {
		results = append(results, SearchResult{
			ID:      r.ID,
			Content: r.Text,
			Score:   r.Score,
			Source:  "vector",
		})
	}

	log.Printf("[Fusion] Vector search returned %d results", len(results))
	return results, nil
}

// searchGraphDB queries Neo4j for text-matching Chunk nodes.
func (f *FusionRetriever) searchGraphDB(ctx context.Context, query string) ([]SearchResult, error) {
	if f.neo4j == nil {
		return nil, fmt.Errorf("neo4j not configured")
	}

	results, err := f.neo4j.SearchChunks(ctx, query, 5)
	if err != nil {
		return nil, err
	}

	var out []SearchResult
	for _, r := range results {
		out = append(out, SearchResult{
			ID:      r.NodeID,
			Content: r.Text,
			Score:   r.Score,
			Source:  "graph",
		})
	}

	log.Printf("[Fusion] Graph search returned %d results", len(out))
	return out, nil
}

// performRRF applies Reciprocal Rank Fusion to ensemble the results.
func (f *FusionRetriever) performRRF(results []SearchResult) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// RRF: score = sum(1 / (k + rank)) where k=60
	const k = 60.0

	// Group by source to establish per-engine rankings
	sourceGroups := map[string][]SearchResult{}
	for _, r := range results {
		sourceGroups[r.Source] = append(sourceGroups[r.Source], r)
	}

	// Calculate RRF scores
	rrfScores := map[string]float32{}
	rrfContent := map[string]SearchResult{}
	for _, group := range sourceGroups {
		for rank, r := range group {
			score := float32(1.0 / (k + float64(rank+1)))
			key := r.Content // Use content as dedup key
			rrfScores[key] += score
			if _, exists := rrfContent[key]; !exists {
				rrfContent[key] = r
			}
		}
	}

	// Build final sorted results
	var fused []SearchResult
	for key, r := range rrfContent {
		r.Score = rrfScores[key]
		fused = append(fused, r)
	}

	// Sort by RRF score descending
	for i := 0; i < len(fused); i++ {
		for j := i + 1; j < len(fused); j++ {
			if fused[j].Score > fused[i].Score {
				fused[i], fused[j] = fused[j], fused[i]
			}
		}
	}

	return fused
}
