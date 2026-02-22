package query

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/booksage/booksage-api/internal/domain/repository"
	"github.com/booksage/booksage-api/internal/embedding"
	"golang.org/x/sync/errgroup"
)

// FusionRetriever manages concurrent retrieval across multiple data stores.
type FusionRetriever struct {
	vectorStore repository.VectorRepository
	graphStore  repository.GraphRepository
	embedder    *embedding.Batcher
	router      *AdaptiveRouter
	extractor   *DualKeyExtractor
	ranker      *SkylineRanker
}

// NewFusionRetriever creates a new FusionRetriever with repository interfaces.
func NewFusionRetriever(vectorStore repository.VectorRepository, graphStore repository.GraphRepository, embedder *embedding.Batcher, llmRouter repository.LLMRouter) *FusionRetriever {
	return &FusionRetriever{
		vectorStore: vectorStore,
		graphStore:  graphStore,
		embedder:    embedder,
		router:      NewAdaptiveRouter(llmRouter),
		extractor:   NewDualKeyExtractor(llmRouter),
		ranker:      &SkylineRanker{},
	}
}

// Retrieve performs asynchronous parallel requests across engines and ensembles them using SOTA techniques.
func (f *FusionRetriever) Retrieve(ctx context.Context, query string) ([]repository.SearchResult, error) {
	log.Printf("[Fusion] Starting SOTA parallel retrieval for: %s", query)

	// 1. Adaptive Routing
	strategy, _ := f.router.DetermineStrategy(ctx, query)
	log.Printf("[Fusion] Strategy selected: %s", strategy)

	// 2. Dual-level Key Extraction
	keys, _ := f.extractor.ExtractKeys(ctx, query)
	log.Printf("[Fusion] Extracted Keys: Entities=%v, Themes=%v", keys.Entities, keys.Themes)

	// Add a global timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	var allResults []repository.SearchResult

	// 3. Parallel Retrieval
	// Engine A: Vector Search (Entities/Low-level)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Entity-based Vector Search...")
		searchTerm := query
		if len(keys.Entities) > 0 {
			searchTerm = strings.Join(keys.Entities, " ")
		}
		docs, err := f.searchVectorDB(ctx, searchTerm)
		if err == nil {
			mu.Lock()
			allResults = append(allResults, docs...)
			mu.Unlock()
		}
		return nil
	})

	// Engine B: Graph Search (Structural context)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Graph/Theme Search...")
		// Use themes for graph search if strategy is summary
		searchTerm := query
		if strategy == StrategySummary && len(keys.Themes) > 0 {
			searchTerm = strings.Join(keys.Themes, " ")
		}
		docs, err := f.searchGraphDB(ctx, searchTerm)
		if err == nil {
			mu.Lock()
			allResults = append(allResults, docs...)
			mu.Unlock()
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	log.Printf("[Fusion] Retrieval complete. Integrating %d results via Skyline Ranker...", len(allResults))
	return f.ranker.Rank(allResults), nil
}

// LastIntent returns the intent from the most recent classification (for SSE reporting).
func (f *FusionRetriever) ClassifyIntent(ctx context.Context, query string) Strategy {
	s, _ := f.router.DetermineStrategy(ctx, query)
	return s
}

// searchVectorDB queries Qdrant using dense vector similarity.
func (f *FusionRetriever) searchVectorDB(ctx context.Context, query string) ([]repository.SearchResult, error) {
	if f.vectorStore == nil || f.embedder == nil {
		return nil, fmt.Errorf("vector store or embedder not configured")
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

	// Search Vector Store
	vectorResults, err := f.vectorStore.Search(ctx, queryVector, 5)
	if err != nil {
		return nil, err
	}

	var results []repository.SearchResult
	for _, r := range vectorResults {
		results = append(results, repository.SearchResult{
			ID:      r.ID,
			Content: r.Content,
			Score:   r.Score,
			Source:  "vector",
		})
	}

	log.Printf("[Fusion] Vector search returned %d results", len(results))
	return results, nil
}

// searchGraphDB queries Graph Store for text-matching Chunk nodes.
func (f *FusionRetriever) searchGraphDB(ctx context.Context, query string) ([]repository.SearchResult, error) {
	if f.graphStore == nil {
		return nil, fmt.Errorf("graph store not configured")
	}

	results, err := f.graphStore.SearchChunks(ctx, query, 5)
	if err != nil {
		return nil, err
	}

	var out []repository.SearchResult
	for _, r := range results {
		out = append(out, repository.SearchResult{
			ID:      r.ID,
			Content: r.Content,
			Score:   r.Score,
			Source:  "graph",
		})
	}

	log.Printf("[Fusion] Graph search returned %d results", len(out))
	return out, nil
}
