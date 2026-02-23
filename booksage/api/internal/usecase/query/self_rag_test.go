package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/booksage/booksage-api/internal/domain"
)

// MockLLMClient is a manual mock for domain.LLMClient to enable dependency injection in tests.
type MockLLMClient struct {
	GenerateFunc func(ctx context.Context, prompt string) (string, error)
	NameFunc     func() string
}

// Ensure MockLLMClient implements domain.LLMClient
var _ domain.LLMClient = (*MockLLMClient)(nil)

func (m *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt)
	}
	return "", nil
}

func (m *MockLLMClient) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-llm"
}

func TestSelfRAGCritique_EvaluateRetrieval(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *MockLLMClient
		expected  bool
	}{
		{
			name: "Relevant Context",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "The context is relevant to the query.", nil
					},
				}
			},
			expected: true,
		},
		{
			name: "Irrelevant Context",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "This content is irrelevant.", nil
					},
				}
			},
			expected: false,
		},
		{
			name: "Ambiguous Response (Strict checking -> False)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "I am not sure.", nil
					},
				}
			},
			expected: false,
		},
		{
			name: "LLM Error (Fallback to True)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "", errors.New("api error")
					},
				}
			},
			expected: true,
		},
		{
			name: "Context Timeout (Fallback to True)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						select {
						case <-ctx.Done():
							return "", ctx.Err()
						default:
							return "irrelevant", nil
						}
					},
				}
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			if tt.name == "Context Timeout (Fallback to True)" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
				defer cancel()
				time.Sleep(2 * time.Millisecond) // Ensure context expires
			}

			critique := NewSelfRAGCritique(tt.setupMock())

			// Act
			isRelevant := critique.EvaluateRetrieval(ctx, "query", "context")

			// Assert
			if isRelevant != tt.expected {
				t.Errorf("EvaluateRetrieval() = %v, want %v", isRelevant, tt.expected)
			}
		})
	}
}

func TestSelfRAGCritique_EvaluateGeneration(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *MockLLMClient
		expected  SupportLevel
	}{
		{
			name: "Fully Supported",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "fully_supported", nil
					},
				}
			},
			expected: FullySupported,
		},
		{
			name: "Partially Supported",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "partially_supported", nil
					},
				}
			},
			expected: Partially,
		},
		{
			name: "No Support",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "no_support", nil
					},
				}
			},
			expected: NoSupport,
		},
		{
			name: "LLM Error (Fallback to Fully Supported)",
			setupMock: func() *MockLLMClient {
				return &MockLLMClient{
					GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
						return "", errors.New("api error")
					},
				}
			},
			expected: FullySupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			critique := NewSelfRAGCritique(tt.setupMock())
			level := critique.EvaluateGeneration(context.Background(), "answer", "context")
			if level != tt.expected {
				t.Errorf("EvaluateGeneration() = %v, want %v", level, tt.expected)
			}
		})
	}
}
