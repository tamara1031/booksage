package agent

import (
	"context"
	"log"

	"github.com/booksage/booksage-api/internal/llm"
)

// TaskType is a re-export or alias if needed, but we'll use the one from llm package
type TaskType = llm.TaskType

// RouteLLMTask determines the appropriate LLM client based on ADR-006.
// This is a high-level wrapper as requested in Step 4.
func RouteLLMTask(router *llm.Router, task llm.TaskType) llm.LLMClient {
	return router.RouteLLMTask(task)
}

// AgentOrchestrator manages the high-level reasoning loop.
type AgentOrchestrator struct {
	router *llm.Router
}

func NewAgentOrchestrator(router *llm.Router) *AgentOrchestrator {
	return &AgentOrchestrator{router: router}
}

func (a *AgentOrchestrator) Run(ctx context.Context, query string) (string, error) {
	log.Printf("[Agent] Orchestrating query: %s", query)
	// Logic for CoR and Self-RAG would go here
	return "Mock Answer", nil
}
