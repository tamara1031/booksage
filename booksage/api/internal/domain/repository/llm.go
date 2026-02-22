package repository

import (
	"context"
)

// LLMClient defines the interface for generating text from a prompt.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// TaskType defines the type of LLM task.
type TaskType string

// LLMRouter defines the interface for routing tasks to appropriate LLM clients.
type LLMRouter interface {
	RouteLLMTask(task TaskType) LLMClient
}
