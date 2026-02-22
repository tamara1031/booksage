package llm

import (
	"log"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// TaskType is a re-export of domain.TaskType for convenience, or use domain directly.
type TaskType = repository.TaskType

const (
	TaskEmbedding               TaskType = repository.TaskType("embedding")
	TaskSimpleKeywordExtraction TaskType = repository.TaskType("simple_keyword_extraction")
	TaskAgenticReasoning        TaskType = repository.TaskType("agentic_reasoning")
	TaskDeepSummarization       TaskType = repository.TaskType("deep_summarization")
	TaskMultimodalParsing       TaskType = repository.TaskType("multimodal_parsing")
)

// Router determines the appropriate LLMClient based on the task's cognitive requirements.
type Router struct {
	localClient  repository.LLMClient
	geminiClient repository.LLMClient
}

// NewRouter initializes the LLM router with the specified backend clients.
func NewRouter(local repository.LLMClient, gemini repository.LLMClient) *Router {
	return &Router{
		localClient:  local,
		geminiClient: gemini,
	}
}

// GetLocalClient returns the local LLM client for maintenance tasks.
func (r *Router) GetLocalClient() repository.LLMClient {
	return r.localClient
}

// RouteEmbeddingTask returns a client capable of generating embeddings.
func (r *Router) RouteEmbeddingTask(task repository.TaskType) repository.EmbeddingClient {
	client := r.RouteLLMTask(task)
	if ec, ok := client.(repository.EmbeddingClient); ok {
		return ec
	}
	return nil
}

// RouteLLMTask evaluates the cognitive load required and routes to the optimal backend (ADR-006).
func (r *Router) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	var selected repository.LLMClient
	var icon string

	switch task {
	case TaskEmbedding, TaskSimpleKeywordExtraction:
		selected = r.localClient
		icon = "🏠"
	case TaskAgenticReasoning, TaskDeepSummarization, TaskMultimodalParsing:
		selected = r.geminiClient
		icon = "☁️"
	default:
		selected = r.localClient
		icon = "🏠"
	}

	log.Printf("[Router] 🛤️  Routing task '%s' to %s %s", task, icon, selected.Name())
	return selected
}
