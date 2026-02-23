package llm

import (
	"log"

	"github.com/booksage/booksage-api/internal/ports"
)

// Router determines the appropriate LLMClient based on the task's cognitive requirements.
type Router struct {
	localLLMClient   ports.LLMClient
	localEmbedClient ports.LLMClient
	geminiClient     ports.LLMClient
}

// NewRouter initializes the LLM router with the specified backend clients.
func NewRouter(localLLM ports.LLMClient, localEmbed ports.LLMClient, gemini ports.LLMClient) *Router {
	return &Router{
		localLLMClient:   localLLM,
		localEmbedClient: localEmbed,
		geminiClient:     gemini,
	}
}

// GetLocalClient returns the local LLM client for maintenance tasks.
func (r *Router) GetLocalClient() ports.LLMClient {
	return r.localLLMClient
}

// RouteEmbeddingTask returns a client capable of generating embeddings.
func (r *Router) RouteEmbeddingTask(task string) ports.EmbeddingClient {
	if ec, ok := r.localEmbedClient.(ports.EmbeddingClient); ok {
		return ec
	}
	return nil
}

// RouteLLMTask evaluates the cognitive load required and routes to the optimal backend (ADR-006).
func (r *Router) RouteLLMTask(task ports.TaskType) ports.LLMClient {
	var selected ports.LLMClient
	var icon string

	switch task {
	case ports.TaskEmbedding:
		selected = r.localEmbedClient
		icon = "🏎️ [Embed]"
	case ports.TaskSimpleKeywordExtraction:
		selected = r.localLLMClient
		icon = "🏎️ [Light]"
	case ports.TaskAgenticReasoning, ports.TaskDeepSummarization, ports.TaskMultimodalParsing:
		selected = r.geminiClient
		icon = "🧠 [Heavy]"
	default:
		selected = r.localLLMClient
		icon = "🏠 [Default]"
	}

	log.Printf("[Router] 🛤️  Routing task '%s' to %s %s", task, icon, selected.Name())
	return selected
}
