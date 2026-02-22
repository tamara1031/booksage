package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

// Entity represents a named entity extracted from text.
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Relation represents a relationship between two entities.
type Relation struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Description string `json:"description"`
}

// GraphExtractor handles entity/relation extraction and entity linking.
type GraphExtractor struct {
	router repository.LLMRouter
}

// NewGraphExtractor creates a new extractor.
func NewGraphExtractor(router repository.LLMRouter) *GraphExtractor {
	return &GraphExtractor{router: router}
}

// ExtractEntitiesAndRelations uses an LLM to find entities and their connections in a chunk.
func (e *GraphExtractor) ExtractEntitiesAndRelations(ctx context.Context, text string) ([]Entity, []Relation, error) {
	if e == nil || e.router == nil {
		return nil, nil, nil
	}
	client := e.router.RouteLLMTask(repository.TaskType("simple_keyword_extraction"))
	if client == nil {
		return nil, nil, nil
	}

	prompt := fmt.Sprintf(`Extract key entities and their relationships from the following text.
Respond ONLY with a JSON object containing "entities" and "relations" arrays.
Entity: { "name": "...", "type": "...", "description": "..." }
Relation: { "source": "...", "target": "...", "description": "..." }

Text: %s`, text)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Basic JSON cleanup – Ollama might wrap in backticks
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var result struct {
		Entities  []Entity   `json:"entities"`
		Relations []Relation `json:"relations"`
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		log.Printf("[Extractor] Failed to parse LLM JSON: %v. Raw: %s", err, resp)
		return nil, nil, nil // Return empty rather than failing the whole ingestion
	}

	return result.Entities, result.Relations, nil
}
