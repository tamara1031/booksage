package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/booksage/booksage-api/internal/agent"
	"github.com/booksage/booksage-api/internal/embedding"
	"github.com/booksage/booksage-api/internal/ingest"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
)

// Server holds the dependencies for the HTTP API server
type Server struct {
	generator    *agent.Generator
	embedBatcher *embedding.Batcher
	parserClient pb.DocumentParserServiceClient
	ingestSaga   *ingest.Orchestrator // Handled locally for now, typically injected via DI module
}

// NewServer initializes a new API server with the required dependencies
func NewServer(gen *agent.Generator, embed *embedding.Batcher, parser pb.DocumentParserServiceClient, saga *ingest.Orchestrator) *Server {
	return &Server{
		generator:    gen,
		embedBatcher: embed,
		parserClient: parser,
		ingestSaga:   saga,
	}
}

// RegisterRoutes registers all API endpoints with a new ServeMux
func (s *Server) RegisterRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// REST API Endpoints defined in API.md
	// Go 1.22+ supports HTTP method routing directly in ServeMux
	mux.HandleFunc("POST /api/v1/query", s.handleQuery)
	mux.HandleFunc("POST /api/v1/ingest", s.handleIngest)
	mux.HandleFunc("GET /api/v1/documents/{document_id}/status", s.handleDocumentStatus)
	mux.HandleFunc("HEAD /api/v1/documents/{document_id}", s.handleDocumentExist)

	return mux
}

type QueryRequest struct {
	Query     string         `json:"query"`
	SessionID string         `json:"session_id,omitempty"`
	Filters   map[string]any `json:"filters,omitempty"`
}

// Removed static QueryResponse as we use SSE now

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
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

	eventStream := make(chan agent.GeneratorEvent)

	// Start generation in a goroutine
	go s.generator.GenerateAnswer(r.Context(), req.Query, eventStream)

	// Consume and stream events
	for event := range eventStream {
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("[Server] Failed to marshal event: %v", err)
			continue
		}

		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form data
	if err := r.ParseMultipartForm(10 << 20); err != nil { // limit 10MB in memory
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Retrieve file from form data
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing 'file' parameter", http.StatusBadRequest)
		return
	}
	_ = file.Close()

	// Retrieve metadata
	metadataStr := r.FormValue("metadata")
	log.Printf("[Server] Received ingest request for %s (size: %d, metadata: %s)", header.Filename, header.Size, metadataStr)

	// Determine document_id (mock logic: same as returned in JSON)
	docID := "doc-" + header.Filename

	// Check if already exists (using same mock logic as handleDocumentExist)
	// For mock purposes, assume IDs starting with "doc-new-" don't exist yet.
	if !strings.HasPrefix(docID, "doc-new-") && docID != "doc-not-found" {
		log.Printf("[Server] Conflict: Document %s already exists", docID)
		http.Error(w, "Document already exists", http.StatusConflict)
		return
	}

	// In a complete implementation, we would stream this `file` to `s.parserClient.Parse`
	// via gRPC and return a generated document_id in the 202 Accepted response.
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"document_id": docID,
		"status":      "processing",
	})
}

func (s *Server) handleDocumentStatus(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if docID == "" {
		http.Error(w, "Document ID required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Mocking registration check: if ID starts with "registered-", return 200, else 404 for this logic
	// In a real implementation, we check the DB.
	if docID == "not-found" {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"document_id": docID,
		"status":      "completed",
		"extracted_metadata": map[string]any{
			"title": "Mock Retrieved Title",
			"pages": 1,
		},
	})
}

func (s *Server) handleDocumentExist(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if docID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Mocking registration check:
	// For demonstration, assume any ID that starts with "new-" doesn't exist.
	// In real life, we check Neo4j or Qdrant.
	exists := !strings.HasPrefix(docID, "new-") && !strings.HasPrefix(docID, "doc-new-") && docID != "not-found"

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
