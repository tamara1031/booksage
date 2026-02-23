package domain

import (
	"context"
)

// TaskType clarifies the LLM objective
type TaskType string

const (
	TaskExtraction TaskType = "extraction"
	TaskSummary    TaskType = "summary"
	TaskRAG        TaskType = "rag"
)

// LLMClient defines the behavior for interacting with a Large Language Model.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// TensorEngine defines high-performance tensor operations (Embeddings, Reranking).
type TensorEngine interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Rerank(ctx context.Context, query string, documents []string) ([]float32, error)
}
