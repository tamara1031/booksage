package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/booksage/booksage-api/internal/domain"
)

// SagaOrchestrator orchestrates the ingestion process ensuring consistency via the Saga pattern.
type SagaOrchestrator struct {
	vectorStore    domain.VectorRepository
	graphStore     domain.GraphRepository
	docRepo        domain.DocumentRepository
	sagaRepo       domain.SagaRepository
	raptor         *RaptorBuilder
	extractor      *GraphExtractor
	entityResolver *EntityResolver
	graphBuilder   *GraphBuilder
}

// NewSagaOrchestrator creates a new ingestion orchestrator.
func NewSagaOrchestrator(v domain.VectorRepository, g domain.GraphRepository, dr domain.DocumentRepository, sr domain.SagaRepository, llm domain.LLMClient, tensor domain.TensorEngine) *SagaOrchestrator {
	return &SagaOrchestrator{
		vectorStore:    v,
		graphStore:     g,
		docRepo:        dr,
		sagaRepo:       sr,
		raptor:         NewRaptorBuilder(llm),
		extractor:      NewGraphExtractor(llm),
		entityResolver: NewEntityResolver(v, tensor),
		graphBuilder:   NewGraphBuilder(),
	}
}

// StartOrResumeIngestion prepares or resumes an ingestion saga.
func (o *SagaOrchestrator) StartOrResumeIngestion(ctx context.Context, doc *domain.Document) (*domain.IngestSaga, error) {
	// 1. Check if document exists by hash
	existingDoc, err := o.docRepo.GetDocumentByHash(ctx, doc.FileHash)
	if err != nil && err.Error() != "record not found" {
		return nil, err
	}

	if existingDoc != nil {
		// Document exists, check for existing saga
		saga, err := o.sagaRepo.GetLatestSagaByDocumentID(ctx, existingDoc.ID)
		if err != nil && err.Error() != "record not found" {
			return nil, err
		}

		if saga != nil {
			if saga.Status == domain.SagaStatusCompleted {
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
	saga := &domain.IngestSaga{
		DocumentID:  doc.ID,
		Status:      domain.SagaStatusPending,
		CurrentStep: domain.StepParsing,
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
func (o *SagaOrchestrator) RunIngestionSaga(ctx context.Context, saga *domain.IngestSaga, chunks []map[string]any, graphNodes []map[string]any) error {
	log.Printf("[Saga Orchestrator] Starting ingestion saga ID: %d", saga.ID)
	strID := fmt.Sprintf("%d", saga.DocumentID)

	// Update to PROCESSING
	if err := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, domain.SagaStatusProcessing, domain.StepEmbedding, ""); err != nil {
		return err
	}
	saga.Version++

	// Step 1: Embedding/Vector Store
	if err := o.executeEmbeddingStep(ctx, saga, strID, chunks); err != nil {
		return err
	}

	// Step 2: Indexing/Graph Store (GraphRAG + RAPTOR)
	if err := o.executeIndexingStep(ctx, saga, strID, chunks, graphNodes); err != nil {
		// Compensate Qdrant on failure
		log.Printf("[Saga - Rollback] Neo4j insertion failed for saga %d. Compensating Qdrant...", saga.ID)
		if compErr := o.vectorStore.DeleteDocument(ctx, strID); compErr != nil {
			log.Printf("[Saga - CRITICAL ALERT] Compensation failed! docID: %s. Error: %v", strID, compErr)
		}
		return err
	}

	// Final status
	if err := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, domain.SagaStatusCompleted, domain.StepIndexing, ""); err != nil {
		return err
	}

	log.Printf("[Saga Orchestrator] Ingestion completed successfully for saga: %d", saga.ID)
	return nil
}

func (o *SagaOrchestrator) executeEmbeddingStep(ctx context.Context, saga *domain.IngestSaga, strID string, chunks []map[string]any) error {
	step := &domain.SagaStep{
		SagaID: saga.ID,
		Name:   domain.StepEmbedding,
		Status: domain.SagaStatusProcessing,
	}
	stepID, _ := o.sagaRepo.UpsertSagaStep(ctx, step)
	step.ID = stepID

	log.Printf("[Saga - Step Embedding] Inserting %d chunks into Qdrant", len(chunks))
	if err := o.vectorStore.InsertChunks(ctx, strID, chunks); err != nil {
		if statusErr := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, domain.SagaStatusFailed, domain.StepEmbedding, err.Error()); statusErr != nil {
			log.Printf("[Saga] Failed to update saga status: %v", statusErr)
		}
		step.Status = domain.SagaStatusFailed
		step.ErrorLog = err.Error()
		if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
			log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
		}
		return fmt.Errorf("qdrant insertion failed: %w", err)
	}

	step.Status = domain.SagaStatusCompleted
	if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
		log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
	}
	return nil
}

func (o *SagaOrchestrator) executeIndexingStep(ctx context.Context, saga *domain.IngestSaga, strID string, chunks []map[string]any, graphNodes []map[string]any) error {
	step := &domain.SagaStep{
		SagaID: saga.ID,
		Name:   domain.StepIndexing,
		Status: domain.SagaStatusProcessing,
	}
	stepID, _ := o.sagaRepo.UpsertSagaStep(ctx, step)
	step.ID = stepID

	log.Printf("[Saga - Step Indexing] Building RAPTOR tree and extracting GraphRAG entities")

	// 1. RAPTOR Hierarchical Tree
	treeNodes, _, err := o.raptor.BuildTree(ctx, strID, chunks)
	if err != nil {
		log.Printf("[Saga] RAPTOR tree build failed: %v", err)
	}

	// 2. Entity & Relation Extraction (GraphRAG)
	var allEntities []domain.Entity
	var allRelations []domain.Relation
	for _, chunk := range chunks {
		text, _ := chunk["content"].(string)
		entities, relations, err := o.extractor.ExtractEntitiesAndRelations(ctx, text)
		if err == nil {
			allEntities = append(allEntities, entities...)
			allRelations = append(allRelations, relations...)
		}
	}

	// 3. Resolve Entities (Delegated to EntityResolver)
	for _, ent := range allEntities {
		_, _, err := o.entityResolver.ResolveEntity(ctx, ent)
		if err != nil {
			log.Printf("[Saga] Entity resolution error for '%s': %v", ent.Name, err)
		}
	}

	// 4. Build Graph Elements (Delegated to GraphBuilder)
	nodesFromBuilder, finalEdges := o.graphBuilder.BuildGraphElements(strID, allEntities, allRelations, treeNodes)

	// Combine all nodes for Neo4j (Input nodes + Builder nodes)
	allGraphNodes := append([]map[string]any{}, graphNodes...)
	allGraphNodes = append(allGraphNodes, nodesFromBuilder...)

	log.Printf("[Saga - Step Indexing] Inserting %d total graph elements into Neo4j", len(allGraphNodes))
	if err := o.graphStore.InsertNodesAndEdges(ctx, strID, allGraphNodes, finalEdges); err != nil {
		if statusErr := o.sagaRepo.UpdateSagaStatus(ctx, saga.ID, saga.Version, domain.SagaStatusFailed, domain.StepIndexing, err.Error()); statusErr != nil {
			log.Printf("[Saga] Failed to update saga status: %v", statusErr)
		}
		step.Status = domain.SagaStatusFailed
		step.ErrorLog = err.Error()
		if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
			log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
		}
		return fmt.Errorf("neo4j insertion failed: %w", err)
	}

	step.Status = domain.SagaStatusCompleted
	if _, stepErr := o.sagaRepo.UpsertSagaStep(ctx, step); stepErr != nil {
		log.Printf("[Saga] Failed to upsert saga step: %v", stepErr)
	}
	return nil
}

// GetDocumentStatus retrieves the status of a document by its hash
func (o *SagaOrchestrator) GetDocumentStatus(ctx context.Context, hash []byte) (*domain.IngestSaga, error) {
	doc, err := o.docRepo.GetDocumentByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("record not found")
	}
	return o.sagaRepo.GetLatestSagaByDocumentID(ctx, doc.ID)
}
