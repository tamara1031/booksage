package query

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/booksage/booksage-api/internal/domain"
)

// Generator is responsible for the Agentic RAG Generation loop (LightRAG, Self-RAG).
type Generator struct {
	llm       domain.LLMClient
	retriever *FusionRetriever
	critique  *SelfRAGCritique
	router    *AdaptiveRouter
}

// NewGenerator initializes the Agentic Generator with the necessary routing logic.
func NewGenerator(llm domain.LLMClient, retriever *FusionRetriever) *Generator {
	return &Generator{
		llm:       llm,
		retriever: retriever,
		critique:  NewSelfRAGCritique(llm),
		router:    NewAdaptiveRouter(llm),
	}
}

// GeneratorEvent represents an event in the generation stream
type GeneratorEvent struct {
	Type    string `json:"type"` // "reasoning", "source", "answer", "error"
	Content string `json:"content"`
}

// GenerateAnswer orchestrates the full Adaptive RAG pipeline:
// 0. Query Routing (Intent Classification)
// 1. LightRAG Extraction & Parallel Retrieval (handled by FusionRetriever)
// 2. Skyline Fusion (handled by FusionRetriever)
// 3. Context-aware answer generation
// 4. Reflexion Loop (Self-RAG Critique -> Re-Retrieval)
// Results are streamed via SSE events through the provided channel.
func (g *Generator) GenerateAnswer(ctx context.Context, query string, stream chan<- GeneratorEvent) {
	defer close(stream)
	log.Printf("[Agent] Starting generation for query: %s", query)

	// Step 0: Adaptive Routing
	stream <- GeneratorEvent{Type: "reasoning", Content: "[Adaptive RAG] Classifying query intent..."}
	intent, err := g.router.ClassifyIntent(ctx, query)
	if err != nil {
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Adaptive RAG] Warning: Intent classification failed (%v). Defaulting to Complex.", err)}
		intent = IntentComplex
	}
	stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("[Adaptive RAG] Identified intent: %s", strings.ToUpper(string(intent)))}

	if intent == IntentSimple {
		// Fast path: Skip heavy retrieval
		stream <- GeneratorEvent{Type: "reasoning", Content: "[Adaptive RAG] Routing to lightweight generation path."}
		answer, err := g.llm.Generate(ctx, query)
		if err != nil {
			stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("generation failed: %v", err)}
			return
		}
		stream <- GeneratorEvent{Type: "answer", Content: answer}
		return
	}

	// Complex Path: Reflexion Loop
	maxIterations := 3
	currentQuery := query
	var cumulativeContext []string
	seenContent := make(map[string]bool)
	var finalAnswer string

	for i := 0; i < maxIterations; i++ {
		iterLabel := fmt.Sprintf("[Reflexion Loop %d/%d]", i+1, maxIterations)
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Starting retrieval cycle...", iterLabel)}

		// Step 1 & 2: Retrieval (Extraction + Search + Fusion)
		if g.retriever != nil {
			results, err := g.retriever.Retrieve(ctx, currentQuery)
			if err != nil {
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Retrieval warning: %v", iterLabel, err)}
			} else {
				newChunks := 0
				for _, r := range results {
					if !seenContent[r.Content] {
						cumulativeContext = append(cumulativeContext, r.Content)
						seenContent[r.Content] = true
						newChunks++
						stream <- GeneratorEvent{
							Type:    "source",
							Content: fmt.Sprintf("[%s] (score: %.2f) %s", r.Source, r.Score, truncate(r.Content, 200)),
						}
					}
				}
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Found %d new context chunks (Total: %d).", iterLabel, newChunks, len(cumulativeContext))}
			}
		}

		// Step 3: Generation
		stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Generating draft answer...", iterLabel)}

		prompt := buildRAGPrompt(query, cumulativeContext) // Use cumulative context
		draftAnswer, err := g.llm.Generate(ctx, prompt)
		if err != nil {
			stream <- GeneratorEvent{Type: "error", Content: fmt.Sprintf("generation failed: %v", err)}
			return
		}

		// Step 4: Critique & Reflexion
		if g.critique != nil && len(cumulativeContext) > 0 {
			// Check for missing context
			sufficient, missingInfo := g.critique.EvaluateMissingContext(ctx, query, draftAnswer, strings.Join(cumulativeContext, "\n\n"))

			if sufficient {
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Context sufficient. Finalizing answer.", iterLabel)}
				finalAnswer = draftAnswer
				break // Exit loop
			}

			// If missing context and not the last iteration
			if !sufficient && i < maxIterations-1 {
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Critique: Missing information detected: '%s'. Re-retrieving...", iterLabel, missingInfo)}
				// Update query for next iteration to specifically target missing info
				currentQuery = fmt.Sprintf("%s. Specifically find information about: %s", query, missingInfo)
				continue // Next iteration
			} else if !sufficient {
				stream <- GeneratorEvent{Type: "reasoning", Content: fmt.Sprintf("%s Max iterations reached. Proceeding with best effort.", iterLabel)}
			}
		}

		finalAnswer = draftAnswer
		break // Exit loop (default case or max iterations reached)
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
