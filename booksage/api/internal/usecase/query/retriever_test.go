package query

import (
	"context"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

func TestRetrieve_NilClients(t *testing.T) {
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

func TestPerformWeightedRRF_Empty(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)
	results := retriever.performWeightedRRF(nil, EngineWeights{"vector": 0.5})
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestPerformWeightedRRF_MultiSource(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)

	input := []repository.SearchResult{
		{ID: "v1", Content: "vector result 1", Score: 0.95, Source: "vector"},
		{ID: "v2", Content: "vector result 2", Score: 0.80, Source: "vector"},
		{ID: "g1", Content: "graph result 1", Score: 0.85, Source: "graph"},
	}

	weights := EngineWeights{"vector": 0.70, "graph": 0.30}
	results := retriever.performWeightedRRF(input, weights)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Vector results should be weighted higher
	if results[0].Score <= 0 {
		t.Errorf("Expected positive weighted RRF score, got %f", results[0].Score)
	}
}

func TestPerformWeightedRRF_Dedup(t *testing.T) {
	retriever := NewFusionRetriever(nil, nil, nil)

	input := []repository.SearchResult{
		{ID: "v1", Content: "shared content", Score: 0.95, Source: "vector"},
		{ID: "g1", Content: "shared content", Score: 0.85, Source: "graph"},
	}

	weights := EngineWeights{"vector": 0.5, "graph": 0.5}
	results := retriever.performWeightedRRF(input, weights)
	if len(results) != 1 {
		t.Errorf("Expected 1 deduplicated result, got %d", len(results))
	}
}

func TestIntentClassifier(t *testing.T) {
	c := &IntentClassifier{}

	tests := []struct {
		query  string
		expect QueryIntent
	}{
		{"Give me a summary of chapter 3", IntentSummary},
		{"What is the definition of RAG?", IntentDefinition},
		{"How does Neo4j connect to Qdrant?", IntentRelationship},
		{"Compare RAPTOR vs ColBERT", IntentComparison},
		{"Tell me something interesting", IntentGeneral},
	}

	for _, tc := range tests {
		got := c.Classify(tc.query)
		if got != tc.expect {
			t.Errorf("Classify(%q) = %s, want %s", tc.query, got, tc.expect)
		}
	}
}

func TestRouteOperator(t *testing.T) {
	op := NewRouteOperator()

	// Summary intent should heavily weight tree/RAPTOR
	w := op.GetWeights(IntentSummary)
	if w["tree"] < w["vector"] {
		t.Errorf("Summary should weight tree > vector, got tree=%f vector=%f", w["tree"], w["vector"])
	}

	// Relationship intent should heavily weight graph
	w = op.GetWeights(IntentRelationship)
	if w["graph"] < w["vector"] {
		t.Errorf("Relationship should weight graph > vector, got graph=%f vector=%f", w["graph"], w["vector"])
	}

	// Unknown intent should return general weights
	w = op.GetWeights("unknown_intent")
	if w == nil {
		t.Error("Expected general weights for unknown intent")
	}
}
