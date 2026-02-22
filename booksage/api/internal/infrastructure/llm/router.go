package llm

import (
	"log"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// TaskType is a re-export of domain.TaskType for convenience, or use domain directly.
type TaskType = repository.TaskType

const (
	TaskEmbedding               TaskType = repository.TaskType("embedding")
	TaskSimpleKeywordExtraction TaskType = repository.TaskType("simple_keyword_extraction") // Light
	TaskAgenticReasoning        TaskType = repository.TaskType("agentic_reasoning")         // Heavy
	TaskDeepSummarization       TaskType = repository.TaskType("deep_summarization")        // Heavy
	TaskMultimodalParsing       TaskType = repository.TaskType("multimodal_parsing")        // Heavy
)

// Router determines the appropriate LLMClient based on the task's cognitive requirements.
type Router struct {
	localLLMClient   repository.LLMClient
	localEmbedClient repository.LLMClient
	geminiClient     repository.LLMClient
}

// NewRouter initializes the LLM router with the specified backend clients.
func NewRouter(localLLM repository.LLMClient, localEmbed repository.LLMClient, gemini repository.LLMClient) *Router {
	return &Router{
		localLLMClient:   localLLM,
		localEmbedClient: localEmbed,
		geminiClient:     gemini,
	}
}

// GetLocalClient returns the local LLM client for maintenance tasks.
func (r *Router) GetLocalClient() repository.LLMClient {
	return r.localLLMClient
}

// RouteEmbeddingTask returns a client capable of generating embeddings.
func (r *Router) RouteEmbeddingTask(task repository.TaskType) repository.EmbeddingClient {
	if ec, ok := r.localEmbedClient.(repository.EmbeddingClient); ok {
		return ec
	}
	return nil
}

// RouteLLMTask evaluates the cognitive load required and routes to the optimal backend (ADR-006).
func (r *Router) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	var selected repository.LLMClient
	var icon string

	switch task {
	case TaskEmbedding:
		selected = r.localEmbedClient
		icon = "🏎️ [Embed]"
	case TaskSimpleKeywordExtraction:
		selected = r.localLLMClient
		icon = "🏎️ [Light]"
	case TaskAgenticReasoning, TaskDeepSummarization, TaskMultimodalParsing:
		selected = r.geminiClient
		icon = "🧠 [Heavy]"
	default:
		selected = r.localLLMClient
		icon = "🏠 [Default]"
	}

	log.Printf("[Router] 🛤️  Routing task '%s' to %s %s", task, icon, selected.Name())
	return selected
}
