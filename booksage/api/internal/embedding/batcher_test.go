package embedding

import (
	"context"
	"errors"
	"testing"
)

type mockEmbeddingClient struct {
	err error
}

func (m *mockEmbeddingClient) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = []float32{1.0, 2.0, float32(len(text))}
	}
	return embeddings, nil
}

func (m *mockEmbeddingClient) Name() string {
	return "mock_embedding"
}

func TestBatcher_Empty(t *testing.T) {
	client := &mockEmbeddingClient{}
	batcher := NewBatcher(client, 2)

	res, tokens, err := batcher.GenerateEmbeddingsBatched(context.Background(), []string{}, "text", "search")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(res) != 0 {
		t.Errorf("Expected 0 results, got %d", len(res))
	}
	if tokens != 0 {
		t.Errorf("Expected 0 tokens, got %d", tokens)
	}
}

func TestBatcher_GenerateEmbeddingsBatched(t *testing.T) {
	client := &mockEmbeddingClient{}
	batcher := NewBatcher(client, 2)

	texts := []string{"one", "two", "three", "four", "five"}
	res, tokens, err := batcher.GenerateEmbeddingsBatched(context.Background(), texts, "text", "search")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(res) != 5 {
		t.Fatalf("Expected 5 results, got %d", len(res))
	}
	if tokens != 10 { // 5 texts * 2 tokens/text
		t.Errorf("Expected 10 total tokens, got %d", tokens)
	}

	// Verify order is preserved
	if res[0].GetDense().Values[2] != float32(len("one")) {
		t.Errorf("Expected length of 'one', got %v", res[0].GetDense().Values[2])
	}
	if res[4].GetDense().Values[2] != float32(len("five")) {
		t.Errorf("Expected length of 'five', got %v", res[4].GetDense().Values[2])
	}
}

func TestBatcher_Error(t *testing.T) {
	client := &mockEmbeddingClient{err: errors.New("mock error")}
	batcher := NewBatcher(client, 2)

	texts := []string{"one", "two"}
	_, _, err := batcher.GenerateEmbeddingsBatched(context.Background(), texts, "text", "search")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != "batch 0 failed: mock error" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
