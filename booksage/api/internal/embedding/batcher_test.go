package embedding

import (
	"context"
	"errors"
	"testing"

	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"google.golang.org/grpc"
)

type mockEmbeddingClient struct {
	err error
}

func (m *mockEmbeddingClient) GenerateEmbeddings(ctx context.Context, in *pb.EmbeddingRequest, opts ...grpc.CallOption) (*pb.EmbeddingResponse, error) {
	if m.err != nil {
		return nil, m.err
	}

	results := make([]*pb.EmbeddingResult, len(in.Texts))
	for i, text := range in.Texts {
		results[i] = &pb.EmbeddingResult{
			Vector: &pb.EmbeddingResult_Dense{
				Dense: &pb.DenseVector{
					Values: []float32{1.0, 2.0, float32(len(text))},
				},
			},
		}
	}

	return &pb.EmbeddingResponse{
		Results:     results,
		TotalTokens: int32(len(in.Texts) * 2), // Mock token count
	}, nil
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
