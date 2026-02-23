package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/booksage/booksage-api/internal/domain"
)

// MockLLMClient satisfies domain.LLMClient for testing
type MockLLMClient struct {
	GenerateFunc func(ctx context.Context, prompt string) (string, error)
}

// Ensure MockLLMClient implements domain.LLMClient
var _ domain.LLMClient = (*MockLLMClient)(nil)

func (m *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt)
	}
	return "mock summary", nil
}

func (m *MockLLMClient) Name() string {
	return "mock-llm"
}

func TestRaptorBuilder_BuildTree(t *testing.T) {
	// Arrange: Define test cases
	tests := []struct {
		name          string
		chunks        []map[string]any
		mockGenerate  func(ctx context.Context, prompt string) (string, error)
		expectedNodes int
		expectSummary string
	}{
		{
			name: "Normal Case: Heading and Text",
			chunks: []map[string]any{
				{"type": "heading", "level": 1, "content": "Introduction"},
				{"type": "text", "content": "This is the intro text."},
			},
			expectedNodes: 1, // Summarized at the end (Last group)
			expectSummary: "mock summary",
		},
		{
			name: "Multiple Headings - Recursive Grouping",
			chunks: []map[string]any{
				{"type": "heading", "level": 1, "content": "Chapter 1"},
				{"type": "text", "content": "Text 1"},
				{"type": "heading", "level": 1, "content": "Chapter 2"},
				{"type": "text", "content": "Text 2"},
			},
			expectedNodes: 2, // Chapter 1 group summarized on Chapter 2 start, Chapter 2 group summarized at end
			expectSummary: "mock summary",
		},
		{
			name: "No Headings (Text Only)",
			chunks: []map[string]any{
				{"type": "text", "content": "Just text."},
			},
			expectedNodes: 1,
			expectSummary: "mock summary",
		},
		{
			name:          "Empty Input",
			chunks:        []map[string]any{},
			expectedNodes: 0,
		},
		{
			name: "LLM Failure Handling",
			chunks: []map[string]any{
				{"type": "text", "content": "Text."},
			},
			mockGenerate: func(ctx context.Context, prompt string) (string, error) {
				return "", errors.New("LLM error")
			},
			expectedNodes: 1,
			expectSummary: "Summary extraction failed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockLLM := &MockLLMClient{GenerateFunc: tt.mockGenerate}
			builder := NewRaptorBuilder(mockLLM)
			ctx := context.Background()

			// Act
			nodes, _, err := builder.BuildTree(ctx, "doc-1", tt.chunks)

			// Assert
			if err != nil {
				t.Fatalf("BuildTree returned unexpected error: %v", err)
			}

			if len(nodes) != tt.expectedNodes {
				t.Errorf("expected %d nodes, got %d", tt.expectedNodes, len(nodes))
			}

			if tt.expectedNodes > 0 && tt.expectSummary != "" {
				// Check the last node's text to verify summary content
				lastNode := nodes[len(nodes)-1]
				if text, ok := lastNode["text"].(string); !ok || text != tt.expectSummary {
					t.Errorf("expected summary %q, got %q", tt.expectSummary, text)
				}
			}
		})
	}
}

func TestRaptorBuilder_NilHandling(t *testing.T) {
	// Arrange
	var nilBuilder *RaptorBuilder
	builderWithNilLLM := NewRaptorBuilder(nil)

	// Act & Assert
	t.Run("Nil Builder Receiver", func(t *testing.T) {
		nodes, _, _ := nilBuilder.BuildTree(context.Background(), "doc-1", nil)
		if nodes != nil {
			t.Error("expected nil return for nil builder")
		}
	})

	t.Run("Nil LLM Client", func(t *testing.T) {
		nodes, _, _ := builderWithNilLLM.BuildTree(context.Background(), "doc-1", nil)
		if nodes != nil {
			t.Error("expected nil return for nil LLM client")
		}
	})
}
