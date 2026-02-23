package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	pb "github.com/booksage/booksage-api/gen/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/domain"
	"github.com/booksage/booksage-api/internal/usecase/ingest"
)

// IngestHandler manages HTTP requests for ingestion.
type IngestHandler struct {
	sagaOrchestrator *ingest.SagaOrchestrator
	service          ingest.Service
	parserClient     pb.DocumentParserServiceClient
}

// NewIngestHandler creates a new Ingestion IngestHandler.
func NewIngestHandler(orchestrator *ingest.SagaOrchestrator, service ingest.Service, parser pb.DocumentParserServiceClient) *IngestHandler {
	return &IngestHandler{
		sagaOrchestrator: orchestrator,
		service:          service,
		parserClient:     parser,
	}
}

// HandleIngest accepts a file upload, initiates parsing, and starts the asynchronous ingestion saga.
func (h *IngestHandler) HandleIngest(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("[IngestHandler] Received ingest request for %s (size: %d, metadata: %s)", header.Filename, header.Size, metadataStr)

	// Calculate SHA-256 hash for deduplication
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		http.Error(w, "Failed to calculate hash", http.StatusInternalServerError)
		return
	}
	fileHash := hash.Sum(nil)
	_, _ = file.Seek(0, io.SeekStart) // Reset file pointer

	// Initialize document model
	docModel := &domain.Document{
		FileHash: fileHash,
		Title:    header.Filename,
		FilePath: header.Filename, // In a real app, this would be the actual storage path
		FileSize: header.Size,
		MimeType: header.Header.Get("Content-Type"),
	}

	// Prepare or resume ingestion saga
	saga, err := h.sagaOrchestrator.StartOrResumeIngestion(r.Context(), docModel)
	if err != nil {
		log.Printf("[IngestHandler] Ingestion check failed for %x: %v", fileHash, err)
		// Check if it's "already ingested" error
		if err.Error() == fmt.Sprintf("document already ingested: %x", fileHash) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"hash":   fmt.Sprintf("%x", fileHash),
				"status": "completed",
			})
			return
		}
		http.Error(w, "Failed to initialize ingestion", http.StatusInternalServerError)
		return
	}

	// Open gRPC stream to parser worker
	stream, err := h.parserClient.Parse(r.Context())
	if err != nil {
		log.Printf("[IngestHandler] Failed to open Parse stream: %v", err)
		http.Error(w, "Failed to communicate with parsing worker", http.StatusInternalServerError)
		return
	}

	// 1. Send metadata
	if err := stream.Send(&pb.ParseRequest{
		Payload: &pb.ParseRequest_Metadata{
			Metadata: &pb.DocumentMetadata{
				Filename:   header.Filename,
				FileType:   header.Header.Get("Content-Type"),
				DocumentId: fmt.Sprintf("%d", saga.DocumentID),
			},
		},
	}); err != nil {
		log.Printf("[IngestHandler] Failed to send metadata: %v", err)
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
				log.Printf("[IngestHandler] Failed to send chunk: %v", sendErr)
				http.Error(w, "Internal error sending data", http.StatusInternalServerError)
				return
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[IngestHandler] Error reading file: %v", err)
			http.Error(w, "Error reading uploaded file", http.StatusInternalServerError)
			return
		}
	}

	// 3. Receive response from worker (Stream)
	if err := stream.CloseSend(); err != nil {
		log.Printf("[IngestHandler] Failed to close send stream: %v", err)
		http.Error(w, "Worker processing failed", http.StatusInternalServerError)
		return
	}

	var allDocs []*pb.RawDocument
	var docID string
	var extractedMetadata map[string]string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[IngestHandler] Worker returned error: %v", err)
			http.Error(w, "Worker processing failed", http.StatusInternalServerError)
			return
		}

		if chunk.DocumentId != "" {
			docID = chunk.DocumentId
		}
		if len(chunk.ExtractedMetadata) > 0 {
			if extractedMetadata == nil {
				extractedMetadata = make(map[string]string)
			}
			for k, v := range chunk.ExtractedMetadata {
				extractedMetadata[k] = v
			}
		}
		allDocs = append(allDocs, chunk.Documents...)
	}

	resp := &pb.ParseResponse{
		DocumentId:        docID,
		ExtractedMetadata: extractedMetadata,
		Documents:         allDocs,
	}

	log.Printf("[IngestHandler] Successfully parsed document %s. Received %d elements.", resp.DocumentId, len(resp.Documents))

	// 4. Delegate heavy lifting to Service (Async)
	go func(sagaID int64, dID string, pResp *pb.ParseResponse) {
		ctx := context.Background() // Independent context
		if err := h.service.ProcessDocument(ctx, sagaID, dID, pResp); err != nil {
			log.Printf("[IngestHandler - Async] Processing failed: %v", err)
		}
	}(saga.ID, resp.DocumentId, resp)

	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saga_id": saga.ID,
		"status":  "processing",
	})
}

func (h *IngestHandler) HandleDocumentStatus(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if docID == "" {
		http.Error(w, "Document ID required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Mocking registration check
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

func (h *IngestHandler) HandleIngestStatusByHash(w http.ResponseWriter, r *http.Request) {
	hashStr := r.URL.Query().Get("hash")
	if hashStr == "" {
		http.Error(w, "Query parameter 'hash' (hex) is required", http.StatusBadRequest)
		return
	}

	hashBytes, err := hex.DecodeString(hashStr)
	if err != nil {
		http.Error(w, "Invalid hex hash", http.StatusBadRequest)
		return
	}

	saga, err := h.sagaOrchestrator.GetDocumentStatus(r.Context(), hashBytes)
	if err != nil {
		if err.Error() == "record not found" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_started"})
			return
		}
		log.Printf("[IngestHandler] Failed checking status for hash %s: %v", hashStr, err)
		http.Error(w, "Failed to check status", http.StatusInternalServerError)
		return
	}

	statusStr := "pending"
	switch saga.Status {
	case domain.SagaStatusProcessing:
		statusStr = "processing"
	case domain.SagaStatusCompleted:
		statusStr = "completed"
	case domain.SagaStatusFailed:
		statusStr = "failed"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saga_id":      saga.ID,
		"document_id":  saga.DocumentID,
		"status":       statusStr,
		"current_step": saga.CurrentStep,
		"updated_at":   saga.UpdatedAt.Unix(),
	})
}

// HandleDocumentExist is used for the HEAD request to check if a document is already indexed.
func (h *IngestHandler) HandleDocumentExist(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("document_id")
	if docID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte("HEAD by ID not implemented, use status check by hash"))
}
