package fusion

import (
	"context"
	"testing"
	"time"
)

func TestRetrieval_Success(t *testing.T) {
	retriever := NewFusionRetriever()

	// Use a short timeout so tests are fast.
	// We might need to adjust the mock wait times in retriever if we really wanted true parallel testing,
	// but the wait times in searchGraphDB etc are hardcoded to 800ms, 500ms, 200ms.
	// It will take ~800ms to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results, err := retriever.Retrieve(ctx, "test query")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results (1 from each db), got %d", len(results))
	}

	sources := make(map[string]bool)
	for _, res := range results {
		sources[res.Source] = true
	}

	for _, expectedSource := range []string{"graph", "tree", "vector"} {
		if !sources[expectedSource] {
			t.Errorf("Expected result from %s", expectedSource)
		}
	}
}

func TestRetrieval_Timeout(t *testing.T) {
	retriever := NewFusionRetriever()

	// Timeout faster than the fastest DB (200ms)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results, err := retriever.Retrieve(ctx, "timeout query")
	if err != nil {
		t.Fatalf("Expected no error due to graceful degradation, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results on timeout, got %d", len(results))
	}
}
