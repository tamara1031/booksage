package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/booksage/booksage-api/internal/usecase/query"
)

type QueryHandler struct {
	generator *query.Generator
}

func NewQueryHandler(gen *query.Generator) *QueryHandler {
	return &QueryHandler{
		generator: gen,
	}
}

type QueryRequest struct {
	Query     string         `json:"query"`
	SessionID string         `json:"session_id,omitempty"`
	Filters   map[string]any `json:"filters,omitempty"`
}

func (h *QueryHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query field is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set headers for Server-Sent Events (SSE)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventStream := make(chan query.GeneratorEvent)

	// Start generation in a goroutine
	go h.generator.GenerateAnswer(r.Context(), req.Query, eventStream)

	// Consume and stream events
	for event := range eventStream {
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("[QueryHandler] Failed to marshal event: %v", err)
			continue
		}

		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}
}
