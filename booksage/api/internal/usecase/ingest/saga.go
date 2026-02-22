package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/database"
	"github.com/booksage/booksage-api/internal/database/models"
	"github.com/booksage/booksage-api/internal/domain/repository"
)

// SagaOrchestrator orchestrates the ingestion process ensuring consistency via the Saga pattern.
type SagaOrchestrator struct {
	vectorStore repository.VectorRepository
	graphStore  repository.GraphRepository
	docRepo     database.DocumentRepository
	sagaRepo    database.SagaRepository
	raptor      *RaptorBuilder
	extractor   *GraphExtractor
	embedder    repository.EmbeddingClient
}

// NewSagaOrchestrator creates a new ingestion orchestrator.
func NewSagaOrchestrator(v repository.VectorRepository, g repository.GraphRepository, dr database.DocumentRepository, sr database.SagaRepository, router repository.LLMRouter, embedder repository.EmbeddingClient) *SagaOrchestrator {
	return &SagaOrchestrator{
		vectorStore: v,
		graphStore:  g,
		docRepo:     dr,
		sagaRepo:    sr,
		raptor:      NewRaptorBuilder(router),
		extractor:   NewGraphExtractor(router),
		embedder:    embedder,
	}
}

// StartOrResumeIngestion prepares or resumes an ingestion saga.
func (o *SagaOrchestrator) StartOrResumeIngestion(ctx context.Context, doc *models.Document) (*models.IngestSaga, error) {
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
func (o *SagaOrchestrator) RunIngestionSaga(ctx context.Context, saga *models.IngestSaga, chunks []map[string]any, graphNodes []map[string]any) error {
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
	if err := o.vectorStore.InsertChunks(ctx, strID, chunks); err != nil {
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

	// Step: Indexing/Graph Store (GraphRAG + RAPTOR)
	step = &models.SagaStep{
		SagaID: saga.ID,
		Name:   models.StepIndexing,
		Status: models.SagaStatusProcessing,
	}
	stepID, _ = o.sagaRepo.UpsertSagaStep(ctx, step)
	step.ID = stepID

	log.Printf("[Saga - Step Indexing] Building RAPTOR tree and extracting GraphRAG entities")

	// 1. RAPTOR Hierarchical Tree
	treeNodes, _, err := o.raptor.BuildTree(ctx, strID, chunks)
	if err != nil {
		log.Printf("[Saga] RAPTOR tree build failed: %v", err)
	}

	// 2. Entity & Relation Extraction (GraphRAG)
	var allEntities []Entity
	var allRelations []Relation
	for _, chunk := range chunks {
		text, _ := chunk["content"].(string)
		entities, relations, err := o.extractor.ExtractEntitiesAndRelations(ctx, text)
		if err == nil {
			allEntities = append(allEntities, entities...)
			allRelations = append(allRelations, relations...)
		}
	}

	// Combine all nodes for Neo4j
	allGraphNodes := append([]map[string]any{}, graphNodes...)
	allGraphNodes = append(allGraphNodes, treeNodes...)

	// 3. Entity Linking (名寄せ) & GT-Links
	var finalEdges []map[string]any

	// Add GraphRAG relations to finalEdges
	for _, rel := range allRelations {
		finalEdges = append(finalEdges, map[string]any{
			"from": fmt.Sprintf("%s-ent-%s", strID, rel.Source),
			"to":   fmt.Sprintf("%s-ent-%s", strID, rel.Target),
			"type": "RELATED_TO",
			"desc": rel.Description,
		})
	}
	for _, ent := range allEntities {
		entID := fmt.Sprintf("%s-ent-%s", strID, ent.Name)

		// Attempt to find existing similar entity (名寄せ)
		if o.embedder != nil {
			vecs, err := o.embedder.Embed(ctx, []string{ent.Name})
			if err == nil && len(vecs) > 0 {
				matches, _ := o.vectorStore.Search(ctx, vecs[0], 1)
				if len(matches) > 0 && matches[0].Score > 0.9 {
					// Found a similar entity, link to it instead of creating new?
					// In LightRAG/GraphRAG, we usually merge or link.
					// For simplicity, we'll keep the current ID but link it to the chunk.
					log.Printf("[Saga] Entity Linking: Matched '%s' to existing %s", ent.Name, matches[0].ID)
				}
			}
		}

		allGraphNodes = append(allGraphNodes, map[string]any{
			"id":   entID,
			"text": ent.Description,
			"type": "Entity",
			"name": ent.Name,
		})

		// GT-Link: Connect entity to the document root or chunks
		finalEdges = append(finalEdges, map[string]any{
			"from": entID,
			"to":   strID, // Connect to Doc root for now
			"type": "GT_LINK",
		})
	}

	log.Printf("[Saga - Step Indexing] Inserting %d total graph elements into Neo4j", len(allGraphNodes))
	if err := o.graphStore.InsertNodesAndEdges(ctx, strID, allGraphNodes, finalEdges); err != nil {
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
		if compErr := o.vectorStore.DeleteDocument(ctx, strID); compErr != nil {
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
func (o *SagaOrchestrator) GetDocumentStatus(ctx context.Context, hash []byte) (*models.IngestSaga, error) {
	doc, err := o.docRepo.GetDocumentByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, database.ErrNotFound
	}
	return o.sagaRepo.GetLatestSagaByDocumentID(ctx, doc.ID)
}
