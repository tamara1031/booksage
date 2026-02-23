package ports

import (
	"context"
)

// TaskType defines the type of LLM task.
type TaskType string

const (
	TaskEmbedding               TaskType = "embedding"
	TaskSimpleKeywordExtraction TaskType = "simple_keyword_extraction"
	TaskAgenticReasoning        TaskType = "agentic_reasoning"
	TaskDeepSummarization       TaskType = "deep_summarization"
	TaskMultimodalParsing       TaskType = "multimodal_parsing"
)

// LLMClient defines the interface for generating text from a prompt.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// EmbeddingClient defines the interface for generating embeddings.
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
}

// LLMRouter defines the interface for routing tasks.
type LLMRouter interface {
	RouteLLMTask(task TaskType) LLMClient
}

// SearchResult represents a generic search result from any engine.
type SearchResult struct {
	ID      string
	Content string
	Score   float32
	Source  string // "vector", "graph", "tree", etc.
}

// VectorRepository defines the interface for vector database operations.
type VectorRepository interface {
	Search(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
	InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error
	DeleteDocument(ctx context.Context, docID string) error
	Close() error
}

// GraphRepository defines the interface for graph database operations.
type GraphRepository interface {
	SearchChunks(ctx context.Context, query string, limit int) ([]SearchResult, error)
	InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error
	DeleteDocument(ctx context.Context, docID string) error
	Close(ctx context.Context) error
}
