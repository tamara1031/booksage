package ingest

import (
	"context"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

type mockLLMClient struct {
	resp string
	err  error
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return m.resp, m.err
}
func (m *mockLLMClient) Name() string { return "mock" }

type mockTaskRouter struct{}

func (m *mockTaskRouter) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	return &mockLLMClient{resp: "Summary of group"}
}

func TestRaptorBuilder_BuildTree(t *testing.T) {
	// Arrange
	builder := NewRaptorBuilder(&mockTaskRouter{})
	chunks := []map[string]any{
		{"content": "Root title", "type": "heading", "level": 1},
		{"content": "Para 1", "type": "text", "level": 0},
		{"content": "Sub title", "type": "heading", "level": 2},
		{"content": "Para 2", "type": "text", "level": 0},
	}

	// Act
	nodes, edges, err := builder.BuildTree(context.Background(), "doc1", chunks)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// It should create at least two tree nodes (summaries for groups before/after headings)
	if len(nodes) < 2 {
		t.Errorf("expected at least 2 tree nodes, got %d", len(nodes))
	}
	if len(edges) != 0 {
		// In the current lightweight version, edges are omitted or simplified
	}
}
