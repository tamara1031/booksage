package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

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
	defer func() { _ = file.Close() }()

	// Retrieve metadata
	metadataStr := r.FormValue("metadata")
	log.Printf("[Server] Received ingest request for %s (size: %d, metadata: %s)", header.Filename, header.Size, metadataStr)

	// Determine document_id (mock logic: same as returned in JSON)
	docID := "doc-" + header.Filename

	// Check if document already exists to prevent duplicate ingestion
	exists, err := s.ingestSaga.DocumentExists(r.Context(), docID)
	if err != nil {
		log.Printf("[Server] Error checking document existence for %s: %v", docID, err)
		http.Error(w, "Failed to verify document status", http.StatusInternalServerError)
		return
	}
	if exists {
		log.Printf("[Server] Document %s already exists, skipping ingestion", docID)
		w.WriteHeader(http.StatusConflict) // 409 Conflict means it's already there
		_ = json.NewEncoder(w).Encode(map[string]string{
			"document_id": docID,
			"status":      "completed",
		})
		return
	}

	// Open gRPC stream to parser worker
	stream, err := s.parserClient.Parse(r.Context())
	if err != nil {
		log.Printf("[Server] Failed to open Parse stream: %v", err)
		http.Error(w, "Failed to communicate with parsing worker", http.StatusInternalServerError)
		return
	}

	// 1. Send metadata
	if err := stream.Send(&pb.ParseRequest{
		Payload: &pb.ParseRequest_Metadata{
			Metadata: &pb.DocumentMetadata{
				Filename:   header.Filename,
				FileType:   header.Header.Get("Content-Type"),
				DocumentId: docID,
			},
		},
	}); err != nil {
		log.Printf("[Server] Failed to send metadata: %v", err)
		http.Error(w, "Internal error sending data", http.StatusInternalServerError)
		return
	}

	// 2. Stream chunks (1MB chunks)
	buffer := make([]byte, 1024*1024)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			if sendErr := stream.Send(&pb.ParseRequest{
				Payload: &pb.ParseRequest_ChunkData{
					ChunkData: buffer[:n],
				},
			}); sendErr != nil {
				log.Printf("[Server] Failed to send chunk: %v", sendErr)
				http.Error(w, "Internal error sending data", http.StatusInternalServerError)
				return
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[Server] Error reading file: %v", err)
			http.Error(w, "Error reading uploaded file", http.StatusInternalServerError)
			return
		}
	}

	// 3. Receive response from worker
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("[Server] Worker returned error: %v", err)
		http.Error(w, "Worker processing failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[Server] Successfully parsed document %s. Received %d elements.", resp.DocumentId, len(resp.Documents))

	// 4. Generate embeddings and run ingestion saga asynchronously
	go func(parsedResp *pb.ParseResponse) {
		ctx := context.Background() // Use an independent background context for the async job

		// Extract texts for embedding
		var texts []string
		for _, doc := range parsedResp.Documents {
			texts = append(texts, doc.Content)
		}

		// Generate Embeddings
		embResults, _, err := s.embedBatcher.GenerateEmbeddingsBatched(ctx, texts, "dense", "retrieval")
		if err != nil {
			log.Printf("[Server - Async] Failed to generate embeddings for %s: %v", parsedResp.DocumentId, err)
			return
		}

		// Prepare Qdrant Chunks
		var chunks []any
		for i, res := range embResults {
			chunks = append(chunks, map[string]any{
				"id":     fmt.Sprintf("%s-chunk-%d", parsedResp.DocumentId, i),
				"text":   res.Text,
				"vector": res.GetDense().GetValues(),
			})
		}

		// Prepare Neo4j Nodes
		var graphNodes []any
		for i, doc := range parsedResp.Documents {
			graphNodes = append(graphNodes, map[string]any{
				"id":   fmt.Sprintf("%s-node-%d", parsedResp.DocumentId, i),
				"text": doc.Content,
				"type": "Chunk",
			})
		}

		// Run the Saga Orchestrator
		if err := s.ingestSaga.RunIngestionSaga(ctx, parsedResp.DocumentId, chunks, graphNodes); err != nil {
			log.Printf("[Server - Async] Ingestion saga failed for %s: %v", parsedResp.DocumentId, err)
			return
		}

		log.Printf("[Server - Async] Successfully completed asynchronous ingestion for %s", parsedResp.DocumentId)
	}(resp)

	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"document_id": resp.DocumentId,
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

// handleDocumentExist is used for the HEAD request to check if a document is already indexed.
func (s *Server) handleDocumentExist(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if docID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	exists, err := s.ingestSaga.DocumentExists(r.Context(), docID)
	if err != nil {
		log.Printf("[Server] Failed checking DB for document %s: %v", docID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
