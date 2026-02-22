package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// SearchKeys holds extracted keywords for retrieval.
type SearchKeys struct {
	Entities []string `json:"entities"` // Low-level keys
	Themes   []string `json:"themes"`   // High-level keys
}

// DualKeyExtractor extracts multi-level keys from a query.
type DualKeyExtractor struct {
	router repository.LLMRouter
}

// NewDualKeyExtractor creates a new extractor.
func NewDualKeyExtractor(router repository.LLMRouter) *DualKeyExtractor {
	return &DualKeyExtractor{router: router}
}

// ExtractKeys uses an LLM to identify specific entities and broader themes in the query.
func (e *DualKeyExtractor) ExtractKeys(ctx context.Context, query string) (*SearchKeys, error) {
	if e == nil || e.router == nil {
		return &SearchKeys{Entities: []string{query}}, nil
	}
	client := e.router.RouteLLMTask(repository.TaskType("simple_keyword_extraction"))
	if client == nil {
		return &SearchKeys{Entities: []string{query}}, nil
	}

	prompt := fmt.Sprintf(`Extract specific entities (names, terms) and broader themes or topics from the user query.
Respond ONLY with a JSON object containing "entities" and "themes" arrays.

Query: %s`, query)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return &SearchKeys{}, nil
	}

	// Basic JSON cleanup
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var keys SearchKeys
	if err := json.Unmarshal([]byte(resp), &keys); err != nil {
		// Fallback to simple split if JSON fails
		keys.Entities = strings.Fields(query)
	}

	return &keys, nil
}
