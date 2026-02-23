package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SearchKeys holds extracted keywords for retrieval.
type SearchKeys struct {
	Entities []string `json:"entities"` // Low-level keys (specific names, terms)
	Themes   []string `json:"themes"`   // High-level keys (broad concepts)
}

// DualKeyExtractor extracts multi-level keys from a query using LightRAG strategy.
type DualKeyExtractor struct {
	router LLMRouter
}

// NewDualKeyExtractor creates a new extractor.
func NewDualKeyExtractor(router LLMRouter) *DualKeyExtractor {
	return &DualKeyExtractor{router: router}
}

// ExtractKeys uses an LLM to identify specific entities and broader themes in the query.
func (e *DualKeyExtractor) ExtractKeys(ctx context.Context, query string) (*SearchKeys, error) {
	if e == nil || e.router == nil {
		return &SearchKeys{Entities: []string{query}, Themes: []string{query}}, nil
	}
	client := e.router.RouteLLMTask(TaskType("simple_keyword_extraction"))
	if client == nil {
		return &SearchKeys{Entities: []string{query}, Themes: []string{query}}, nil
	}

	prompt := fmt.Sprintf(`Extract keywords from the user query for a two-stage retrieval system.
1. "entities": Specific names, locations, technical terms (Low-level).
2. "themes": Broader concepts, topics, or intent (High-level).

Respond ONLY with a JSON object containing "entities" and "themes" arrays of strings.

Query: %s`, query)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		// Fallback on error
		return &SearchKeys{Entities: []string{query}}, nil
	}

	// Basic JSON cleanup
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var keys SearchKeys
	if err := json.Unmarshal([]byte(resp), &keys); err != nil {
		// Fallback to simple split if JSON fails
		keys.Entities = strings.Fields(query)
		keys.Themes = []string{} // No themes if parsing fails
	}

	return &keys, nil
}
