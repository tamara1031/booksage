package query

import (
	"github.com/booksage/booksage-api/internal/ports"
)

// SearchResult represents a generic search result from any engine.
type SearchResult = ports.SearchResult

// VectorRepository defines the interface for vector database operations.
type VectorRepository = ports.VectorRepository

// GraphRepository defines the interface for graph database operations.
type GraphRepository = ports.GraphRepository

// LLMClient defines the interface for generating text from a prompt.
type LLMClient = ports.LLMClient

// TaskType is a string alias for LLM tasks.
type TaskType = ports.TaskType

const (
	TaskEmbedding               = ports.TaskEmbedding
	TaskSimpleKeywordExtraction = ports.TaskSimpleKeywordExtraction
	TaskAgenticReasoning        = ports.TaskAgenticReasoning
	TaskDeepSummarization       = ports.TaskDeepSummarization
	TaskMultimodalParsing       = ports.TaskMultimodalParsing
)
