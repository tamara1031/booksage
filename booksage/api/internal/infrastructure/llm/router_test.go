package llm_test

import (
	"context"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
	"github.com/booksage/booksage-api/internal/infrastructure/llm"
)

// mockClient implements the repository.LLMClient interface for testing.
type mockClient struct {
	name string
}

func (m *mockClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "Mock response from: " + m.name, nil
}

func (m *mockClient) Name() string {
	return m.name
}

func TestLLMRouter(t *testing.T) {
	localMock := &mockClient{name: "local_ollama"}
	geminiMock := &mockClient{name: "gemini_api"}

	router := llm.NewRouter(localMock, geminiMock)

	tests := []struct {
		name         string
		taskType     repository.TaskType
		expectedName string
	}{
		{
			name:         "Embedding should route to Local",
			taskType:     repository.TaskType("embedding"),
			expectedName: "local_ollama",
		},
		{
			name:         "Keyword Extraction should route to Local",
			taskType:     repository.TaskType("simple_keyword_extraction"),
			expectedName: "local_ollama",
		},
		{
			name:         "Agentic Reasoning should route to Gemini",
			taskType:     repository.TaskType("agentic_reasoning"),
			expectedName: "gemini_api",
		},
		{
			name:         "Deep Summarization should route to Gemini",
			taskType:     repository.TaskType("deep_summarization"),
			expectedName: "gemini_api",
		},
		{
			name:         "Multimodal Parsing should route to Gemini",
			taskType:     repository.TaskType("multimodal_parsing"),
			expectedName: "gemini_api",
		},
		{
			name:         "Unknown tasks should default to Local",
			taskType:     repository.TaskType("unknown_task_123"),
			expectedName: "local_ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := router.RouteLLMTask(tt.taskType)

			mock, ok := client.(*mockClient)
			if !ok {
				t.Fatalf("Expected client to be of type *mockClient")
			}

			if mock.name != tt.expectedName {
				t.Errorf("For Task %s, expected router to select %s but got %s", tt.taskType, tt.expectedName, mock.name)
			}
		})
	}
}
