package query

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/booksage/booksage-api/internal/domain"
	"golang.org/x/sync/errgroup"
)

// FusionRetriever manages concurrent retrieval across multiple data stores.
type FusionRetriever struct {
	vectorStore domain.VectorRepository
	graphStore  domain.GraphRepository
	tensor      domain.TensorEngine // Replaces embedder, handles Embed & Rerank
	extractor   *DualKeyExtractor
	ranker      *SkylineRanker
}

// NewFusionRetriever creates a new FusionRetriever with repository interfaces.
func NewFusionRetriever(vectorStore domain.VectorRepository, graphStore domain.GraphRepository, tensor domain.TensorEngine, llm domain.LLMClient) *FusionRetriever {
	return &FusionRetriever{
		vectorStore: vectorStore,
		graphStore:  graphStore,
		tensor:      tensor,
		extractor:   NewDualKeyExtractor(llm),
		ranker:      &SkylineRanker{},
	}
}

// Retrieve performs asynchronous parallel requests across engines and ensembles them using SOTA techniques.
func (f *FusionRetriever) Retrieve(ctx context.Context, query string) ([]domain.SearchResult, error) {
	log.Printf("[Fusion] Starting SOTA parallel retrieval for: %s", query)

	// 1. Dual-level Key Extraction (LightRAG)
	keys, _ := f.extractor.ExtractKeys(ctx, query)
	log.Printf("[Fusion] Extracted Keys: Entities=%v, Themes=%v", keys.Entities, keys.Themes)

	// Add a global timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	var allResults []domain.SearchResult

	// 2. Parallel Retrieval (1st-Stage)
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
		} else {
			log.Printf("[Fusion] Vector search warning: %v", err)
		}
		return nil
	})

	// Engine B: Graph Search (Structural context/Themes)
	g.Go(func() error {
		log.Println("[Fusion] Dispatching Graph/Theme Search...")
		searchTerm := query
		if len(keys.Themes) > 0 {
			searchTerm = strings.Join(keys.Themes, " ")
		}
		docs, err := f.searchGraphDB(ctx, searchTerm)
		if err == nil {
			mu.Lock()
			allResults = append(allResults, docs...)
			mu.Unlock()
		} else {
			log.Printf("[Fusion] Graph search warning: %v", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if len(allResults) == 0 {
		return nil, nil
	}

	// 3. Tensor Reranking (2nd-Stage)
	log.Printf("[Fusion] Reranking %d candidates via Infinity/ColBERT...", len(allResults))
	if f.tensor != nil {
		candidates := make([]string, len(allResults))
		for i, r := range allResults {
			candidates[i] = r.Content
		}

		scores, err := f.tensor.Rerank(ctx, query, candidates)
		if err != nil {
			log.Printf("[Fusion] Rerank warning: %v (using raw scores)", err)
		} else {
			// Update scores with high-fidelity tensor scores
			for i := range allResults {
				if i < len(scores) {
					allResults[i].Score = scores[i]
				}
			}
		}
	}

	// 4. Skyline Fusion (Pareto Optimal)
	log.Printf("[Fusion] Integrating results via Skyline Ranker...")
	return f.ranker.Rank(allResults), nil
}

// searchVectorDB queries Qdrant using dense vector similarity.
func (f *FusionRetriever) searchVectorDB(ctx context.Context, query string) ([]domain.SearchResult, error) {
	if f.vectorStore == nil || f.tensor == nil {
		return nil, fmt.Errorf("vector store or tensor engine not configured")
	}

	// Generate query embedding
	embeddings, err := f.tensor.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embedding generation failed: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding result returned")
	}

	queryVector := embeddings[0]

	// Search Vector Store
	vectorResults, err := f.vectorStore.Search(ctx, queryVector, 10) // Fetch more for reranking
	if err != nil {
		return nil, err
	}

	var results []domain.SearchResult
	for _, r := range vectorResults {
		results = append(results, domain.SearchResult{
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
func (f *FusionRetriever) searchGraphDB(ctx context.Context, query string) ([]domain.SearchResult, error) {
	if f.graphStore == nil {
		return nil, fmt.Errorf("graph store not configured")
	}

	results, err := f.graphStore.SearchChunks(ctx, query, 10) // Fetch more for reranking
	if err != nil {
		return nil, err
	}

	var out []domain.SearchResult
	for _, r := range results {
		out = append(out, domain.SearchResult{
			ID:      r.ID,
			Content: r.Content,
			Score:   r.Score,
			Source:  "graph",
		})
	}

	log.Printf("[Fusion] Graph search returned %d results", len(out))
	return out, nil
}
