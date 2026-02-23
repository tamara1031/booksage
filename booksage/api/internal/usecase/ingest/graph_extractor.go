package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/booksage/booksage-api/internal/domain"
)

// GraphExtractor handles entity/relation extraction and entity linking.
type GraphExtractor struct {
	llm domain.LLMClient
}

// NewGraphExtractor creates a new extractor.
func NewGraphExtractor(llm domain.LLMClient) *GraphExtractor {
	return &GraphExtractor{llm: llm}
}

// ExtractEntitiesAndRelations uses an LLM to find entities and their connections in a chunk.
func (e *GraphExtractor) ExtractEntitiesAndRelations(ctx context.Context, text string) ([]domain.Entity, []domain.Relation, error) {
	if e == nil || e.llm == nil {
		return nil, nil, nil
	}

	prompt := fmt.Sprintf(`Extract key entities and their relationships from the following text.
Respond ONLY with a JSON object containing "entities" and "relations" arrays.
Entity: { "name": "...", "type": "...", "description": "..." }
Relation: { "source": "...", "target": "...", "description": "..." }

Text: %s`, text)

	resp, err := e.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Basic JSON cleanup – Ollama might wrap in backticks
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var result struct {
		Entities  []domain.Entity   `json:"entities"`
		Relations []domain.Relation `json:"relations"`
	}

	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		log.Printf("[Extractor] Failed to parse LLM JSON: %v. Raw: %s", err, resp)
		return nil, nil, nil // Return empty rather than failing the whole ingestion
	}

	return result.Entities, result.Relations, nil
}
