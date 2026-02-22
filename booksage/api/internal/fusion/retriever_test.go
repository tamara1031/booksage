package fusion

import (
	"context"
	"testing"
)

func TestRetrieve_NilClients(t *testing.T) {
	// With nil clients, retrieval should degrade gracefully (no results, no crash)
	retriever := NewFusionRetriever(nil, nil, nil)

	ctx := context.Background()
	results, err := retriever.Retrieve(ctx, "test query")
	if err != nil {
		t.Fatalf("Expected no error (graceful degradation), got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results with nil clients, got %d", len(results))
	}
}

func TestPerformRRF_Empty(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)
	results := retriever.performRRF(nil)
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestPerformRRF_MultiSource(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)

	input := []SearchResult{
		{ID: "v1", Content: "vector result 1", Score: 0.95, Source: "vector"},
		{ID: "v2", Content: "vector result 2", Score: 0.80, Source: "vector"},
		{ID: "g1", Content: "graph result 1", Score: 0.85, Source: "graph"},
	}

	results := retriever.performRRF(input)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// First result should have highest RRF score
	if results[0].Score <= 0 {
		t.Errorf("Expected positive RRF score, got %f", results[0].Score)
	}
}

func TestPerformRRF_Dedup(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)

	// Same content from two sources should be deduplicated
	input := []SearchResult{
		{ID: "v1", Content: "shared content", Score: 0.95, Source: "vector"},
		{ID: "g1", Content: "shared content", Score: 0.85, Source: "graph"},
	}

	results := retriever.performRRF(input)
	if len(results) != 1 {
		t.Errorf("Expected 1 deduplicated result, got %d", len(results))
	}
}
