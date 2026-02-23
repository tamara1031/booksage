package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/booksage/booksage-api/internal/domain"
)

// Strategy defines the retrieval approach.
type Strategy string

const (
	StrategyFactual Strategy = "factual" // Entity-based lookup
	StrategySummary Strategy = "summary" // Tree-based overview
)

// AdaptiveRouter analyzes query intent to select the best retrieval strategy.
type AdaptiveRouter struct {
	llm domain.LLMClient
}

// NewAdaptiveRouter creates a new router.
func NewAdaptiveRouter(llm domain.LLMClient) *AdaptiveRouter {
	return &AdaptiveRouter{llm: llm}
}

// DetermineStrategy uses an LLM to decide if the query requires a factual or summary-based approach.
func (r *AdaptiveRouter) DetermineStrategy(ctx context.Context, query string) (Strategy, error) {
	if r == nil || r.llm == nil {
		return StrategyFactual, nil
	}

	prompt := fmt.Sprintf(`Analyze the following user query and decide if it requires a factual (specific details) or summary (overview/theme) strategy.
Respond ONLY with one word: "factual" or "summary".

Query: %s`, query)

	resp, err := r.llm.Generate(ctx, prompt)
	if err != nil {
		// Fallback to factual if LLM fails
		return StrategyFactual, nil
	}

	resp = strings.ToLower(strings.TrimSpace(resp))
	if strings.Contains(resp, "summary") {
		return StrategySummary, nil
	}

	return StrategyFactual, nil
}
