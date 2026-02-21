package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/llm"
)

// Generator is responsible for the Agentic RAG Generation loop (CoR, Self-RAG).
type Generator struct {
	router *llm.Router
}

// NewGenerator initializes the Agentic Generator with the necessary routing logic.
func NewGenerator(router *llm.Router) *Generator {
	return &Generator{
		router: router,
	}
}

// GeneratorEvent represents an event in the generation stream
type GeneratorEvent struct {
	Type    string `json:"type"` // "reasoning", "source", "answer", "error"
	Content string `json:"content"`
}

// GenerateAnswer demonstrates the use of the LLM Router deciding which model to use.
// It streams reasoning steps and the final answer via the provided channel.
func (g *Generator) GenerateAnswer(ctx context.Context, query string, stream chan<- GeneratorEvent) {
	defer close(stream)
	log.Printf("[Agent] Starting generation for query: %s", query)

	// Step 1: Lightweight task (e.g. Keyword extraction for fallback)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Extracting simple keywords using Local Model..."}
	localClient := g.router.RouteLLMTask(llm.TaskSimpleKeywordExtraction)

	keywords, err := localClient.Generate(ctx, "Extract keywords from: "+query)
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("keyword extraction failed: %v", err)}
		return
	}
	stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] Local Model Keywords: %s", keywords)}

	// Step 2: Heavy cognitive task (Agentic Reasoning / Critique)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Performing heavy semantic reasoning and critique using Gemini API..."}
	geminiClient := g.router.RouteLLMTask(llm.TaskAgenticReasoning)

	finalAnswer, err := geminiClient.Generate(ctx, "Critique and Answer: "+query+" (Keywords: "+keywords+")")
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("reasoning failed: %v", err)}
		return
	}

	stream <- GeneratorEvent{Type: "answer", Content: finalAnswer}
	log.Printf("[Agent] Generation complete.")
}
