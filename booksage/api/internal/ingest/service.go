package ingest

import (
	"context"
	"fmt"
	"log"

	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
)

// Service defines the interface for ingestion operations.
type Service interface {
	ProcessDocument(ctx context.Context, sagaID int64, docID string, parsedResp *pb.ParseResponse) error
}

// BatchEmbedder defines the interface for batched embedding generation.
type BatchEmbedder interface {
	GenerateEmbeddingsBatched(ctx context.Context, texts []string, embType, taskType string) ([]*pb.EmbeddingResult, int32, error)
}

// IngestionService implements the Service interface.
type IngestionService struct {
	sagaOrchestrator *SagaOrchestrator
	embedBatcher     BatchEmbedder
}

// NewIngestionService creates a new IngestionService.
func NewIngestionService(orchestrator *SagaOrchestrator, embedder BatchEmbedder) *IngestionService {
	return &IngestionService{
		sagaOrchestrator: orchestrator,
		embedBatcher:     embedder,
	}
}

// ProcessDocument handles the asynchronous processing of a parsed document (Embedding -> Graph -> Saga).
func (s *IngestionService) ProcessDocument(ctx context.Context, sagaID int64, docID string, parsedResp *pb.ParseResponse) error {
	// Extract texts for embedding
	var texts []string
	for _, doc := range parsedResp.Documents {
		texts = append(texts, doc.Content)
	}

	// Generate Embeddings
	embResults, _, err := s.embedBatcher.GenerateEmbeddingsBatched(ctx, texts, "dense", "retrieval")
	if err != nil {
		log.Printf("[IngestionService] Failed to generate embeddings for %s: %v", docID, err)
		return fmt.Errorf("embedding generation failed: %w", err)
	}

	// Prepare Qdrant Chunks (with metadata from parse response)
	var chunks []map[string]any
	for i, res := range embResults {
		chunk := map[string]any{
			"id":     fmt.Sprintf("%s-chunk-%d", docID, i),
			"text":   res.Text,
			"vector": res.GetDense().GetValues(),
		}
		// Propagate structural metadata from ParseResponse
		if i < len(parsedResp.Documents) {
			doc := parsedResp.Documents[i]
			chunk["page_number"] = int(doc.PageNumber)
			chunk["type"] = doc.Type
			if levelStr, ok := doc.Metadata["level"]; ok {
				var level int
				if _, err := fmt.Sscanf(levelStr, "%d", &level); err == nil {
					chunk["level"] = level
				}
			}
		}
		chunks = append(chunks, chunk)
	}

	// Prepare Neo4j Nodes (with enriched metadata)
	var graphNodes []map[string]any
	for i, doc := range parsedResp.Documents {
		node := map[string]any{
			"id":          fmt.Sprintf("%s-node-%d", docID, i),
			"text":        doc.Content,
			"type":        doc.Type,
			"page_number": int(doc.PageNumber),
		}
		if levelStr, ok := doc.Metadata["level"]; ok {
			var level int
			if _, err := fmt.Sscanf(levelStr, "%d", &level); err == nil {
				node["level"] = level
			}
		}
		graphNodes = append(graphNodes, node)
	}

	// Run the Saga Orchestrator
	// Fetch Saga state for safety
	saga, err := s.sagaOrchestrator.sagaRepo.GetSagaByID(ctx, sagaID)
	if err != nil {
		log.Printf("[IngestionService] Failed to retrieve saga %d: %v", sagaID, err)
		return fmt.Errorf("saga retrieval failed: %w", err)
	}

	if err := s.sagaOrchestrator.RunIngestionSaga(ctx, saga, chunks, graphNodes); err != nil {
		log.Printf("[IngestionService] Ingestion saga failed for ID %d: %v", saga.ID, err)
		return fmt.Errorf("saga execution failed: %w", err)
	}

	log.Printf("[IngestionService] Successfully completed ingestion for saga ID %d", saga.ID)
	return nil
}
