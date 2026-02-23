package query

import (
	"context"
	"strings"
	"testing"

	"github.com/booksage/booksage-api/internal/domain"
)

// MockLLM simulates LLM responses for various components.
type MockLLM struct {
	t                *testing.T
	intent           string
	generateResponse string
	critiqueMissing  bool
	critiqueResponse string
	extractionResp   string
	callCount        int
}

func (m *MockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	m.callCount++
	promptLower := strings.ToLower(prompt)

	// Route Intent
	if strings.Contains(promptLower, "classify the following user query intent") {
		return m.intent, nil
	}
	// LightRAG Extraction
	if strings.Contains(promptLower, "extract keywords") {
		if m.extractionResp != "" {
			return m.extractionResp, nil
		}
		return `{"entities": ["test"], "themes": ["test"]}`, nil
	}
	// Self-RAG Critique (Missing Context)
	if strings.Contains(promptLower, "evaluate if the provided context is sufficient") {
		if m.critiqueMissing {
			m.critiqueMissing = false // Only miss on the first call to test the loop
			return "MISSING: specific date", nil
		}
		return "SUFFICIENT", nil
	}
	// General Generation
	return m.generateResponse, nil
}

func (m *MockLLM) Name() string {
	return "mock-llm"
}

// Ensure MockLLM implements domain.LLMClient
var _ domain.LLMClient = (*MockLLM)(nil)

func TestGenerator_ReflexionLoop(t *testing.T) {
	// Stub test: This would ideally mock the full loop, but without interface abstraction on FusionRetriever
	// we are limited to testing components or integration.
	// We proceed with testing routing logic primarily.
}

func TestGenerator_AdaptiveRouting_Simple(t *testing.T) {
	mockLLM := &MockLLM{
		t:      t,
		intent: "simple",
	}

	gen := NewGenerator(mockLLM, nil) // Nil retriever
	stream := make(chan GeneratorEvent, 10)

	go gen.GenerateAnswer(context.Background(), "Hello", stream)

	var events []GeneratorEvent
	for e := range stream {
		events = append(events, e)
	}

	// Should contain routing reasoning
	foundRouting := false
	for _, e := range events {
		if strings.Contains(e.Content, "Identified intent: SIMPLE") {
			foundRouting = true
			break
		}
	}

	if !foundRouting {
		t.Errorf("Expected to find simple routing event. Got: %v", events)
	}
}

func TestGenerator_AdaptiveRouting_Complex(t *testing.T) {
	mockLLM := &MockLLM{
		t:      t,
		intent: "complex",
	}

	gen := NewGenerator(mockLLM, nil)
	stream := make(chan GeneratorEvent, 20)

	// Since retriever is nil, it won't find context, so it won't critique, so it won't loop.
	// But it SHOULD enter the loop logic initially.
	go gen.GenerateAnswer(context.Background(), "Explain quantum physics", stream)

	var events []GeneratorEvent
	for e := range stream {
		events = append(events, e)
	}

	foundComplex := false
	foundLoop := false
	for _, e := range events {
		if strings.Contains(e.Content, "Identified intent: COMPLEX") {
			foundComplex = true
		}
		if strings.Contains(e.Content, "[Reflexion Loop 1/3]") {
			foundLoop = true
		}
	}

	if !foundComplex {
		t.Errorf("Expected to find complex routing event. Got: %v", events)
	}
	if !foundLoop {
		t.Errorf("Expected to find loop start event. Got: %v", events)
	}
}
