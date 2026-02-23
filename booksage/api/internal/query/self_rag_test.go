package query

import (
	"context"
	"testing"
)

func TestSelfRAGCritique_EvaluateRetrieval(t *testing.T) {
	tests := []struct {
		name     string
		resp     string
		expected bool
	}{
		{
			name:     "Relevant Context",
			resp:     "The context is relevant to the query.",
			expected: true,
		},
		{
			name:     "Irrelevant Context",
			resp:     "This is irrelevant.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := &mockLLMClient{resp: tt.resp}
			critique := NewSelfRAGCritique(&mockTaskRouter{client: mockClient})

			// Act
			isRelevant := critique.EvaluateRetrieval(context.Background(), "query", "context")

			// Assert
			if isRelevant != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, isRelevant)
			}
		})
	}
}

func TestSelfRAGCritique_EvaluateGeneration(t *testing.T) {
	tests := []struct {
		name     string
		resp     string
		expected SupportLevel
	}{
		{
			name:     "Fully Supported",
			resp:     "fully_supported",
			expected: FullySupported,
		},
		{
			name:     "Partially Supported",
			resp:     "partially",
			expected: Partially,
		},
		{
			name:     "No Support",
			resp:     "no_support",
			expected: NoSupport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := &mockLLMClient{resp: tt.resp}
			critique := NewSelfRAGCritique(&mockTaskRouter{client: mockClient})

			// Act
			level := critique.EvaluateGeneration(context.Background(), "answer", "context")

			// Assert
			if level != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, level)
			}
		})
	}
}
