package query

import (
	"context"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

type mockQdrant struct{}

func (m *mockQdrant) InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error {
	return nil
}
func (m *mockQdrant) DeleteDocument(ctx context.Context, docID string) error { return nil }
func (m *mockQdrant) Search(ctx context.Context, vector []float32, limit int) ([]repository.SearchResult, error) {
	return []repository.SearchResult{{ID: "1", Content: "Vector result", Score: 0.9, Source: "vector"}}, nil
}
func (m *mockQdrant) Close() error { return nil }

type mockNeo4j struct{}

func (m *mockNeo4j) InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error {
	return nil
}
func (m *mockNeo4j) DeleteDocument(ctx context.Context, docID string) error { return nil }
func (m *mockNeo4j) SearchChunks(ctx context.Context, query string, limit int) ([]repository.SearchResult, error) {
	return []repository.SearchResult{{ID: "2", Content: "Graph result", Score: 0.8, Source: "graph"}}, nil
}
func (m *mockNeo4j) Close(ctx context.Context) error { return nil }

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

func TestRetrieve_ParallelFlow(t *testing.T) {
	// Arrange
	mockRouter := &mockTaskRouter{client: &mockLLMClient{resp: "summary"}}
	retriever := NewFusionRetriever(&mockQdrant{}, &mockNeo4j{}, nil, mockRouter)

	// Act
	results, err := retriever.Retrieve(context.Background(), "test query")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even if search results are empty in mocks, the flow should complete
	if results == nil {
		t.Error("expected non-nil result slice")
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
