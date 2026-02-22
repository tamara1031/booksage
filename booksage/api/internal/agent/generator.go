package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/booksage/booksage-api/internal/fusion"
	"github.com/booksage/booksage-api/internal/llm"
)

// Generator is responsible for the Agentic RAG Generation loop (CoR, Self-RAG).
type Generator struct {
	router    *llm.Router
	retriever *fusion.FusionRetriever
}

// NewGenerator initializes the Agentic Generator with the necessary routing logic.
func NewGenerator(router *llm.Router, retriever *fusion.FusionRetriever) *Generator {
	return &Generator{
		router:    router,
		retriever: retriever,
	}
}

// GeneratorEvent represents an event in the generation stream
type GeneratorEvent struct {
	Type    string `json:"type"` // "reasoning", "source", "answer", "error"
	Content string `json:"content"`
}

// GenerateAnswer orchestrates the RAG pipeline:
// 1. Keyword extraction (local LLM)
// 2. Fusion retrieval (Qdrant + Neo4j)
// 3. Context-aware answer generation (Cloud/Local LLM)
// Results are streamed via SSE events through the provided channel.
func (g *Generator) GenerateAnswer(ctx context.Context, query string, stream chan<- GeneratorEvent) {
	defer close(stream)
	log.Printf("[Agent] Starting generation for query: %s", query)

	// Step 1: Keyword extraction (lightweight, local model)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Extracting keywords using Local Model..."}
	localClient := g.router.RouteLLMTask(llm.TaskSimpleKeywordExtraction)

	keywords, err := localClient.Generate(ctx, "Extract the most important keywords from this question. Return only the keywords separated by commas: "+query)
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("keyword extraction failed: %v", err)}
		// Fall back to using the raw query
		keywords = query
	}
	stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] Keywords: %s", keywords)}

	// Step 2: Fusion Retrieval (parallel Qdrant + Neo4j search)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Searching knowledge base (Vector + Graph)..."}

	var contextChunks []string
	if g.retriever != nil {
		results, err := g.retriever.Retrieve(ctx, query)
		if err != nil {
			stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] Retrieval warning: %v. Proceeding without context.", err)}
		} else {
			for _, r := range results {
				contextChunks = append(contextChunks, r.Content)
				stream <- GeneratorEvent{
					Type:    "source",
					Content: fmt.Sprintf("[%s] (score: %.2f) %s", r.Source, r.Score, truncate(r.Content, 200)),
				}
			}
			stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] Retrieved %d context chunks from knowledge base.", len(results))}
		}
	} else {
		stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] No retriever configured. Generating without context."}
	}

	// Step 3: Context-aware Generation (heavy reasoning, Cloud LLM)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Generating answer with context..."}
	geminiClient := g.router.RouteLLMTask(llm.TaskAgenticReasoning)

	prompt := buildRAGPrompt(query, contextChunks)

	finalAnswer, err := geminiClient.Generate(ctx, prompt)
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("generation failed: %v", err)}
		return
	}

	stream <- GeneratorEvent{Type: "answer", Content: finalAnswer}
	log.Printf("[Agent] Generation complete.")
}

// buildRAGPrompt constructs a prompt with retrieved context for the LLM.
func buildRAGPrompt(query string, contextChunks []string) string {
	if len(contextChunks) == 0 {
		return "Answer the following question to the best of your ability:\n\n" + query
	}

	var sb strings.Builder
	sb.WriteString("You are a helpful assistant that answers questions based on the provided context.\n")
	sb.WriteString("Use ONLY the information in the context to answer. If the context doesn't contain the answer, say so.\n\n")
	sb.WriteString("=== CONTEXT ===\n")
	for i, chunk := range contextChunks {
		sb.WriteString(fmt.Sprintf("[Source %d]\n%s\n\n", i+1, chunk))
	}
	sb.WriteString("=== QUESTION ===\n")
	sb.WriteString(query)
	sb.WriteString("\n\n=== ANSWER ===\n")
	return sb.String()
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
