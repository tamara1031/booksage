package query

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// SupportLevel indicates how well an answer is grounded in the context.
type SupportLevel string

const (
	FullySupported     SupportLevel = "fully_supported"
	PartiallySupported SupportLevel = "partially_supported"
	NoSupport          SupportLevel = "no_support"
)

// SelfRAGCritique evaluates retrieval relevance and generation grounding using an LLM.
type SelfRAGCritique struct {
	router repository.LLMRouter
}

// NewSelfRAGCritique creates a new critique evaluator.
func NewSelfRAGCritique(router repository.LLMRouter) *SelfRAGCritique {
	return &SelfRAGCritique{router: router}
}

// EvaluateRetrieval determines if retrieved context is relevant to the query.
// Returns true if the context is relevant, false if it should be discarded.
func (s *SelfRAGCritique) EvaluateRetrieval(ctx context.Context, query, contextText string) bool {
	client := s.router.RouteLLMTask(repository.TaskType("simple_keyword_extraction"))

	prompt := fmt.Sprintf(`Determine if the following context is relevant to answering the question.
Respond with ONLY one word: "Relevant" or "Irrelevant".

Question: %s

Context: %s

Verdict:`, query, truncateForCritique(contextText, 500))

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("[Self-RAG] Retrieval critique failed: %v (defaulting to relevant)", err)
		return true // Fail-open: assume relevant if critique fails
	}

	verdict := strings.TrimSpace(strings.ToLower(resp))
	isRelevant := strings.Contains(verdict, "relevant") && !strings.Contains(verdict, "irrelevant")

	log.Printf("[Self-RAG] Retrieval critique: %s → %v", verdict, isRelevant)
	return isRelevant
}

// EvaluateGeneration determines if an answer is factually supported by the context.
func (s *SelfRAGCritique) EvaluateGeneration(ctx context.Context, answer, contextText string) SupportLevel {
	client := s.router.RouteLLMTask(repository.TaskType("simple_keyword_extraction"))

	prompt := fmt.Sprintf(`Evaluate whether the answer is factually supported by the context.
Respond with ONLY one of: "Fully Supported", "Partially Supported", or "No Support".

Context: %s

Answer: %s

Support Level:`, truncateForCritique(contextText, 500), truncateForCritique(answer, 300))

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("[Self-RAG] Generation critique failed: %v (defaulting to partial)", err)
		return PartiallySupported
	}

	verdict := strings.TrimSpace(strings.ToLower(resp))
	log.Printf("[Self-RAG] Generation critique: %s", verdict)

	switch {
	case strings.Contains(verdict, "fully"):
		return FullySupported
	case strings.Contains(verdict, "no support"):
		return NoSupport
	default:
		return PartiallySupported
	}
}

func truncateForCritique(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
