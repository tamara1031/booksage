package query

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Note: MockLLMClient is defined in self_rag_test.go (shared within package query).
// If running this test in isolation, ensure MockLLMClient is available.

func TestAdaptiveRouter_DetermineStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setupMock func() *MockLLMClient
		expected  Strategy
	}{
		{
			name: "Summary Intent",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "The user wants a summary.", nil
					},
				}
			},
			expected: StrategySummary,
		},
		{
			name: "Factual Intent",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "factual detail", nil
					},
				}
			},
			expected: StrategyFactual,
		},
		{
			name: "Ambiguous Response (Defaults to Factual)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "I am not sure.", nil
					},
				}
			},
			expected: StrategyFactual,
		},
		{
			name: "LLM Error (Defaults to Factual)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "", errors.New("api error")
					},
				}
			},
			expected: StrategyFactual,
		},
		{
			name: "Context Timeout (Defaults to Factual)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						select {
						case <-ctx.Done():
							return "", ctx.Err()
						default:
							return "summary", nil
						}
					},
				}
			},
			expected: StrategyFactual,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Arrange
			ctx := context.Background()
			if tt.name == "Context Timeout (Defaults to Factual)" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
				defer cancel()
				time.Sleep(2 * time.Millisecond) // Ensure timeout triggers
			}

			router := NewAdaptiveRouter(tt.setupMock())

			// Act
			strategy, err := router.DetermineStrategy(ctx, "query")

			// Assert
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if strategy != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, strategy)
			}
		})
	}
}

func TestAdaptiveRouter_NilSafety(t *testing.T) {
	t.Parallel()
	var router *AdaptiveRouter

	strategy, err := router.DetermineStrategy(context.Background(), "query")

	if err != nil {
		t.Errorf("expected no error on nil router, got %v", err)
	}
	if strategy != StrategyFactual {
		t.Errorf("expected default Factual on nil router")
	}
}
