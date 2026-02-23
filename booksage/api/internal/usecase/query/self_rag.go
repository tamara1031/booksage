package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/booksage/booksage-api/internal/domain"
)

type SupportLevel string

const (
	FullySupported SupportLevel = "fully_supported"
	Partially      SupportLevel = "partially_supported"
	NoSupport      SupportLevel = "no_support"
)

// SelfRAGCritique evaluates RAG performance using an LLM.
type SelfRAGCritique struct {
	llm domain.LLMClient
}

// NewSelfRAGCritique creates a new critique component.
func NewSelfRAGCritique(llm domain.LLMClient) *SelfRAGCritique {
	return &SelfRAGCritique{llm: llm}
}

// EvaluateRetrieval checks if a retrieved chunk is relevant to the query_usecase.
func (c *SelfRAGCritique) EvaluateRetrieval(ctx context.Context, query, context string) bool {
	if c == nil || c.llm == nil {
		return true
	}

	prompt := fmt.Sprintf(`Evaluate if the following context is relevant to the user query_usecase.
Respond ONLY with "relevant" or "irrelevant".

Query: %s
Context: %s`, query, context)

	resp, err := c.llm.Generate(ctx, prompt)
	if err != nil {
		return true // Fallback to including it
	}

	resp = strings.ToLower(resp)
	if strings.Contains(resp, "irrelevant") {
		return false
	}
	return strings.Contains(resp, "relevant")
}

func (c *SelfRAGCritique) EvaluateGeneration(ctx context.Context, answer, context string) SupportLevel {
	if c == nil || c.llm == nil {
		return FullySupported
	}

	prompt := fmt.Sprintf(`Evaluate if the following answer is strictly supported by the provided context.
Respond ONLY with one of: "fully_supported", "partially_supported", "no_support".

Context: %s
Answer: %s`, context, answer)

	resp, err := c.llm.Generate(ctx, prompt)
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

// EvaluateMissingContext checks if the generated answer is sufficient based on the context.
// If not sufficient, it returns (false, "What info is missing") to guide the next retrieval.
func (c *SelfRAGCritique) EvaluateMissingContext(ctx context.Context, query, answer, context string) (bool, string) {
	if c == nil || c.llm == nil {
		return true, "" // Assume sufficient if critique fails
	}

	prompt := fmt.Sprintf(`Evaluate if the provided Context is SUFFICIENT to fully answer the Query.
If sufficient, respond ONLY with "SUFFICIENT".
If missing information, respond with "MISSING: <brief description of what specific information is missing to answer the query>".

Query: %s
Context: %s
Generated Answer: %s`, query, context, answer)

	resp, err := c.llm.Generate(ctx, prompt)
	if err != nil {
		return true, "" // Fail open
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(strings.ToUpper(resp), "SUFFICIENT") {
		return true, ""
	}

	if strings.HasPrefix(strings.ToUpper(resp), "MISSING:") {
		missingInfo := strings.TrimSpace(strings.TrimPrefix(resp, "MISSING:"))
		return false, missingInfo
	}

	// Fallback check
	if strings.Contains(strings.ToLower(resp), "missing") {
		return false, resp
	}

	return true, ""
}
