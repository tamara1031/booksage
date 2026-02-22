package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

type SupportLevel string

const (
	FullySupported SupportLevel = "fully_supported"
	Partially      SupportLevel = "partially_supported"
	NoSupport      SupportLevel = "no_support"
)

// SelfRAGCritique evaluates RAG performance using an LLM.
type SelfRAGCritique struct {
	router repository.LLMRouter
}

// NewSelfRAGCritique creates a new critique component.
func NewSelfRAGCritique(router repository.LLMRouter) *SelfRAGCritique {
	return &SelfRAGCritique{router: router}
}

// EvaluateRetrieval checks if a retrieved chunk is relevant to the query.
func (c *SelfRAGCritique) EvaluateRetrieval(ctx context.Context, query, context string) bool {
	if c == nil || c.router == nil {
		return true
	}
	client := c.router.RouteLLMTask(repository.TaskType("agentic_reasoning"))
	if client == nil {
		return true
	}

	prompt := fmt.Sprintf(`Evaluate if the following context is relevant to the user query.
Respond ONLY with "relevant" or "irrelevant".

Query: %s
Context: %s`, query, context)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return true // Fallback to including it
	}

	return strings.Contains(strings.ToLower(resp), "relevant")
}

// EvaluateGeneration checks if an answer is supported by the context.
func (c *SelfRAGCritique) EvaluateGeneration(ctx context.Context, answer, context string) SupportLevel {
	client := c.router.RouteLLMTask(repository.TaskType("agentic_reasoning"))

	prompt := fmt.Sprintf(`Evaluate if the following answer is strictly supported by the provided context.
Respond ONLY with one of: "fully_supported", "partially_supported", "no_support".

Context: %s
Answer: %s`, context, answer)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return FullySupported // Fallback
	}

	resp = strings.ToLower(strings.TrimSpace(resp))
	switch {
	case strings.Contains(resp, "fully"):
		return FullySupported
	case strings.Contains(resp, "partially"):
		return Partially
	default:
		return NoSupport
	}
}
