package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/database"
	"github.com/booksage/booksage-api/internal/database/models"
)

// QdrantClient defines the interface for Vector DB operations
type QdrantClient interface {
	InsertChunks(ctx context.Context, docID string, chunks []any) error
	DeleteDocument(ctx context.Context, docID string) error
	DocumentExists(ctx context.Context, docID string) (bool, error)
}

// Neo4jClient defines the interface for Graph DB operations
type Neo4jClient interface {
	InsertNodesAndEdges(ctx context.Context, docID string, nodes []any) error
	DeleteDocumentNodes(ctx context.Context, docID string) error
	DocumentExists(ctx context.Context, docID string) (bool, error)
}

// Orchestrator orchestrates the ingestion process ensuring consistency via the Saga pattern.
type Orchestrator struct {
	qdrant   QdrantClient
	neo4j    Neo4jClient
	docRepo  database.DocumentRepository
	sagaRepo database.SagaRepository
}

// NewOrchestrator creates a new ingestion orchestrator.
func NewOrchestrator(q QdrantClient, n Neo4jClient, dr database.DocumentRepository, sr database.SagaRepository) *Orchestrator {
	return &Orchestrator{
		qdrant:   q,
		neo4j:    n,
		docRepo:  dr,
		sagaRepo: sr,
	}
}

// StartOrResumeIngestion prepares or resumes an ingestion saga.
func (o *Orchestrator) StartOrResumeIngestion(ctx context.Context, doc *models.Document) (*models.IngestSaga, error) {
	// 1. Check if document exists by hash
	existingDoc, err := o.docRepo.GetDocumentByHash(ctx, doc.FileHash)
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}

	if existingDoc != nil {
		// Document exists, check for existing saga
		saga, err := o.sagaRepo.GetLatestSagaByDocumentID(ctx, existingDoc.ID)
		if err != nil && err != database.ErrNotFound {
			return nil, err
		}

		if saga != nil {
			if saga.Status == models.SagaStatusCompleted {
				return nil, fmt.Errorf("document already ingested: %x", doc.FileHash)
			}
			// Return existing saga to resume
			return saga, nil
		}
		// Doc exists but no saga? Create one.
		doc.ID = existingDoc.ID
	} else {
		// New document
		id, err := o.docRepo.CreateDocument(ctx, doc)
		if err != nil {
			return nil, err
		}
		doc.ID = id
	}

	// Create new saga
	saga := &models.IngestSaga{
		DocumentID:  doc.ID,
		Status:      models.SagaStatusPending,
		CurrentStep: models.StepParsing,
	}
	sagaID, err := o.sagaRepo.CreateSaga(ctx, saga)
	if err != nil {
		return nil, err
	}
	saga.ID = sagaID
	saga.Version = 1

	return saga, nil
}

// RunIngestionSaga executes the dual-database ingestion with compensating transactions.
// It tracks progress in the SagaRepository.
func (o *Orchestrator) RunIngestionSaga(ctx context.Context, saga *models.IngestSaga, chunks []any, graphNodes []any) error {
	log.Printf("[Saga Orchestrator] Starting ingestion saga ID: %d", saga.ID)
	strID := fmt.Sprintf("%d", saga.DocumentID)

	// Update to PROCESSING
	if err := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, models.SagaStatusProcessing, models.StepEmbedding, ""); err != nil {
		return err
	}
	saga.Version++

	// Step: Embedding/Vector Store (Simplified for now as existing code does chunks)
	step := &models.SagaStep{
		SagaID: saga.ID,
		Name:   models.StepEmbedding,
		Status: models.SagaStatusProcessing,
	}
	stepID, _ := o.sagaRepo.UpsertSagaStep(ctx, step)
	step.ID = stepID

	log.Printf("[Saga - Step Embedding] Inserting %d chunks into Qdrant", len(chunks))
	if err := o.qdrant.InsertChunks(ctx, strID, chunks); err != nil {
		if statusErr := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, models.SagaStatusFailed, models.StepEmbedding, err.Error()); statusErr != nil {
			log.Printf("[Saga] Failed to update saga status: %v", statusErr)
		}
		step.Status = models.SagaStatusFailed
		step.ErrorLog = err.Error()
		if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
			log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
		}
		return fmt.Errorf("qdrant insertion failed: %w", err)
	}

	step.Status = models.SagaStatusCompleted
	if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
		log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
	}

	// Step: Indexing/Graph Store
	step = &models.SagaStep{
		SagaID: saga.ID,
		Name:   models.StepIndexing,
		Status: models.SagaStatusProcessing,
	}
	stepID, _ = o.sagaRepo.UpsertSagaStep(ctx, step)
	step.ID = stepID

	log.Printf("[Saga - Step Indexing] Inserting %d nodes into Neo4j", len(graphNodes))
	if err := o.neo4j.InsertNodesAndEdges(ctx, strID, graphNodes); err != nil {
		log.Printf("[Saga - Rollback] Neo4j insertion failed for saga %d. Compensating Qdrant...", saga.ID)

		// Update state
		if statusErr := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, models.SagaStatusFailed, models.StepIndexing, err.Error()); statusErr != nil {
			log.Printf("[Saga] Failed to update saga status: %v", statusErr)
		}
		step.Status = models.SagaStatusFailed
		step.ErrorLog = err.Error()
		if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
			log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
		}

		// Compensation: Rollback the Qdrant insertion
		if compErr := o.qdrant.DeleteDocument(ctx, strID); compErr != nil {
			log.Printf("[Saga - CRITICAL ALERT] Compensation failed! docID: %s. Error: %v", strID, compErr)
		}

		return fmt.Errorf("neo4j insertion failed, transaction rolled back: %w", err)
	}

	step.Status = models.SagaStatusCompleted
	if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
		log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
	}

	// Final status
	if err := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, models.SagaStatusCompleted, models.StepIndexing, ""); err != nil {
		return err
	}

	log.Printf("[Saga Orchestrator] Ingestion completed successfully for saga: %d", saga.ID)
	return nil
}

// GetDocumentStatus retrieves the status of a document by its hash
func (o *Orchestrator) GetDocumentStatus(ctx context.Context, hash []byte) (*models.IngestSaga, error) {
	doc, err := o.docRepo.GetDocumentByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, database.ErrNotFound
	}
	return o.sagaRepo.GetLatestSagaByDocumentID(ctx, doc.ID)
}
