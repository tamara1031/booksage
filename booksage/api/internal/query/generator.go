package query

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Generator is responsible for the Agentic RAG Generation loop (LightRAG, Self-RAG).
type Generator struct {
	llm       LLMClient
	retriever *FusionRetriever
	critique  *SelfRAGCritique
}

// NewGenerator initializes the Agentic Generator with the necessary routing logic.
func NewGenerator(llm LLMClient, retriever *FusionRetriever) *Generator {
	return &Generator{
		llm:       llm,
		retriever: retriever,
		critique:  NewSelfRAGCritique(llm),
	}
}

// GeneratorEvent represents an event in the generation stream
type GeneratorEvent struct {
	Type    string `json:"type"` // "reasoning", "source", "answer", "error"
	Content string `json:"content"`
}

// GenerateAnswer orchestrates the full RAG pipeline:
// 1. LightRAG Extraction & Parallel Retrieval (handled by FusionRetriever)
// 2. Skyline Fusion (handled by FusionRetriever)
// 3. Context-aware answer generation
// 4. Self-RAG: critique generation grounding
// Results are streamed via SSE events through the provided channel.
func (g *Generator) GenerateAnswer(ctx context.Context, query string, stream chan<- GeneratorEvent) {
	defer close(stream)
	log.Printf("[Agent] Starting generation for query: %s", query)

	// Step 1: Retrieval (includes LightRAG extraction, Parallel Search, Reranking, Skyline Fusion)
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Retrieving context (LightRAG + Parallel Search)..."}

	var allContextChunks []string
	if g.retriever != nil {
		results, err := g.retriever.Retrieve(ctx, query)
		if err != nil {
			stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Fusion] Retrieval warning: %v", err)}
		} else {
			for _, r := range results {
				allContextChunks = append(allContextChunks, r.Content)
				stream <- GeneratorEvent{
					Type:    "source",
					Content: fmt.Sprintf("[%s] (score: %.2f) %s", r.Source, r.Score, truncate(r.Content, 200)),
				}
			}
			stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Agent] %d Pareto-optimal context chunks selected.", len(allContextChunks))}
		}
	} else {
		stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] No retriever configured. Generating without context."}
	}

	// Step 2: Context-aware Generation
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Agent] Generating answer..."}

	prompt := buildRAGPrompt(query, allContextChunks)
	answer, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("generation failed: %v", err)}
		return
	}

	// Step 3: Self-RAG — Generation Critique
	if g.critique != nil && len(allContextChunks) > 0 {
		contextJoined := strings.Join(allContextChunks, "\n\n")
		support := g.critique.EvaluateGeneration(ctx, answer, contextJoined)
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Self-RAG] Support level: %s", support)}

		if support == NoSupport {
			stream <- GeneratorEvent{Type: "reasoning", Content: "[Self-RAG] Answer not supported by context. Regenerating with strict constraints..."}

			answer, err = g.llm.Generate(ctx, prompt+"\n\nIMPORTANT: Base your answer STRICTLY on the provided context.")
			if err != nil {
				stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("regeneration failed: %v", err)}
				return
			}
		}
	}

	stream <- GeneratorEvent{Type: "answer", Content: answer}
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
