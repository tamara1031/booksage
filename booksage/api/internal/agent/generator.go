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
	critique  *SelfRAGCritique
}

// NewGenerator initializes the Agentic Generator with the necessary routing logic.
func NewGenerator(router *llm.Router, retriever *fusion.FusionRetriever) *Generator {
	return &Generator{
		router:    router,
		retriever: retriever,
		critique:  NewSelfRAGCritique(router),
	}
}

// GeneratorEvent represents an event in the generation stream
type GeneratorEvent struct {
	Type    string `json:"type"` // "reasoning", "source", "answer", "error"
	Content string `json:"content"`
}

// GenerateAnswer orchestrates the full RAG pipeline:
// 1. Chain-of-Retrieval (CoR): decompose complex queries into sub-queries
// 2. Fusion retrieval with intent-driven weights
// 3. Self-RAG: critique retrieval relevance
// 4. Context-aware answer generation
// 5. Self-RAG: critique generation grounding
// Results are streamed via SSE events through the provided channel.
func (g *Generator) GenerateAnswer(ctx context.Context, query string, stream chan<- GeneratorEvent) {
	defer close(stream)
	log.Printf("[Agent] Starting generation for query: %s", query)

	// Step 1: CoR — Sub-query decomposition
	stream <- GeneratorEvent{Type: "reasoning", Content: "[CoR] Analyzing query complexity..."}
	subQueries := g.decomposeQuery(ctx, query)

	if len(subQueries) > 1 {
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[CoR] Decomposed into %d sub-queries", len(subQueries))}
	}

	// Step 2: Fusion Retrieval for each sub-query
	var allContextChunks []string

	if g.retriever != nil {
		for i, sq := range subQueries {
			stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Fusion] Searching for sub-query %d/%d: %s", i+1, len(subQueries), truncate(sq, 80))}

			results, err := g.retriever.Retrieve(ctx, sq)
			if err != nil {
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Fusion] Search warning: %v", err)}
				continue
			}

			// Step 3: Self-RAG — Retrieval Critique
			for _, r := range results {
				if g.critique != nil {
					if !g.critique.EvaluateRetrieval(ctx, sq, r.Content) {
						stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Self-RAG] Filtered irrelevant result from %s", r.Source)}
						continue
					}
				}

				allContextChunks = append(allContextChunks, r.Content)
				stream <- GeneratorEvent{
					Type:    "source",
					Content: fmt.Sprintf("[%s] (score: %.2f) %s", r.Source, r.Score, truncate(r.Content, 200)),
				}
			}
		}

		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] %d relevant context chunks after Self-RAG filtering.", len(allContextChunks))}
	} else {
		stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] No retriever configured. Generating without context."}
	}

	// Step 4: Context-aware Generation
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Generating answer..."}
	geminiClient := g.router.RouteLLMTask(llm.TaskAgenticReasoning)

	prompt := buildRAGPrompt(query, allContextChunks)
	answer, err := geminiClient.Generate(ctx, prompt)
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("generation failed: %v", err)}
		return
	}

	// Step 5: Self-RAG — Generation Critique
	if g.critique != nil && len(allContextChunks) > 0 {
		contextJoined := strings.Join(allContextChunks, "\n\n")
		support := g.critique.EvaluateGeneration(ctx, answer, contextJoined)
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Self-RAG] Support level: %s", support)}

		if support == NoSupport {
			stream <- GeneratorEvent{Type: "reasoning", Content: "[Self-RAG] Answer not supported by context. Regenerating..."}

			answer, err = geminiClient.Generate(ctx, prompt+"\n\nIMPORTANT: Base your answer STRICTLY on the provided context.")
			if err != nil {
				stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("regeneration failed: %v", err)}
				return
			}
		}
	}

	stream <- GeneratorEvent{Type: "answer", Content: answer}
	log.Printf("[Agent] Generation complete.")
}

// decomposeQuery uses an LLM to break complex queries into sub-queries (CoR).
// Falls back to the original query if decomposition fails or isn't needed.
func (g *Generator) decomposeQuery(ctx context.Context, query string) []string {
	client := g.router.RouteLLMTask(llm.TaskSimpleKeywordExtraction)

	prompt := fmt.Sprintf(`Analyze this question. If it contains multiple distinct information needs, decompose it into 2-3 simpler sub-questions. If it's already simple, return it as-is.

Return ONLY the questions, one per line. No numbering, no explanations.

Question: %s`, query)

	resp, err := client.Generate(ctx, prompt)
	if err != nil {
		log.Printf("[CoR] Decomposition failed: %v (using original query)", err)
		return []string{query}
	}

	var subQueries []string
	for _, line := range strings.Split(resp, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(trimmed) > 5 {
			subQueries = append(subQueries, trimmed)
		}
	}

	if len(subQueries) == 0 {
		return []string{query}
	}
	return subQueries
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
