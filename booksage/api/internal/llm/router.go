package llm

import (
	"context"
	"log"
)

// TaskType defines the cognitive category of the LLM workload.
type TaskType string

const (
	// High-Volume, Lightweight Tasks (Routed to Local)
	TaskEmbedding               TaskType = "embedding"
	TaskSimpleKeywordExtraction TaskType = "simple_keyword_extraction"

	// Heavy Cognitive, Massive Context Tasks (Routed to Cloud/Gemini)
	TaskAgenticReasoning  TaskType = "agentic_reasoning"
	TaskDeepSummarization TaskType = "deep_summarization"
	TaskMultimodalParsing TaskType = "multimodal_parsing"
)

// LLMClient serves as the abstraction for interacting with underlying AI models.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Name() string
}

// Router determines the appropriate LLMClient based on the task's cognitive requirements.
type Router struct {
	localClient  LLMClient
	geminiClient LLMClient
}

// NewRouter initializes the LLM router with the specified backend clients.
func NewRouter(local LLMClient, gemini LLMClient) *Router {
	return &Router{
		localClient:  local,
		geminiClient: gemini,
	}
}

// RouteLLMTask evaluates the cognitive load required and routes to the optimal backend (ADR-006).
func (r *Router) RouteLLMTask(task TaskType) LLMClient {
	var selected LLMClient
	var icon string

	switch task {
	case TaskEmbedding, TaskSimpleKeywordExtraction:
		// Send high volume or simple tasks to local models (e.g. Ollama/ColBERT) within the cluster.
		selected = r.localClient
		icon = "🏠"
	case TaskAgenticReasoning, TaskDeepSummarization, TaskMultimodalParsing:
		// Send tasks needing complex reasoning, huge 2M context windows, or vision capabilities to Gemini API.
		selected = r.geminiClient
		icon = "☁️"
	default:
		// Fallback to local for safety and cost if unspecified
		selected = r.localClient
		icon = "🏠"
	}

	log.Printf("[Router] 🛤️  Routing task '%s' to %s %s", task, icon, selected.Name())
	return selected
}
