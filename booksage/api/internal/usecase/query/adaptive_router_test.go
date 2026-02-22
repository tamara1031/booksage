package query

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

type mockTaskRouter struct {
	client repository.LLMClient
}

func (m *mockTaskRouter) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	return m.client
}

func TestAdaptiveRouter_DetermineStrategy(t *testing.T) {
	tests := []struct {
		name     string
		resp     string
		expected Strategy
	}{
		{
			name:     "Summary Intent",
			resp:     "This is a summary of the book.",
			expected: StrategySummary,
		},
		{
			name:     "Factual Intent",
			resp:     "The specific detail is 42.",
			expected: StrategyFactual,
		},
		{
			name:     "Empty Response defaults to Factual",
			resp:     "",
			expected: StrategyFactual,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := &mockLLMClient{resp: tt.resp}
			router := NewAdaptiveRouter(&mockTaskRouter{client: mockClient})

			// Act
			strategy, err := router.DetermineStrategy(context.Background(), "dummy query")

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strategy != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, strategy)
			}
		})
	}
}

func TestAdaptiveRouter_NilSafety(t *testing.T) {
	// Arrange
	var router *AdaptiveRouter = nil

	// Act
	strategy, err := router.DetermineStrategy(context.Background(), "query")

	// Assert
	if err != nil {
		t.Errorf("expected no error on nil router, got %v", err)
	}
	if strategy != StrategyFactual {
		t.Errorf("expected default Factual on nil router")
	}
}
