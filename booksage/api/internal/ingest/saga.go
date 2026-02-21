package ingest

import (
	"context"
	"fmt"
	"log"
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
	qdrant QdrantClient
	neo4j  Neo4jClient
}

// NewOrchestrator creates a new ingestion orchestrator.
func NewOrchestrator(q QdrantClient, n Neo4jClient) *Orchestrator {
	return &Orchestrator{
		qdrant: q,
		neo4j:  n,
	}
}

// RunIngestionSaga executes the dual-database ingestion with compensating transactions.
func (o *Orchestrator) RunIngestionSaga(ctx context.Context, docID string, chunks []any, graphNodes []any) error {
	log.Printf("[Saga Orchestrator] Starting ingestion for document: %s", docID)

	// Step 1: Write to Qdrant
	log.Printf("[Saga - Step 1] Inserting %d chunks into Qdrant", len(chunks))
	if err := o.qdrant.InsertChunks(ctx, docID, chunks); err != nil {
		// If step 1 fails, we just abort. No compensation needed as nothing was fully written.
		return fmt.Errorf("qdrant insertion failed: %w", err)
	}

	// Step 2: Write to Neo4j
	log.Printf("[Saga - Step 2] Inserting %d nodes into Neo4j", len(graphNodes))
	if err := o.neo4j.InsertNodesAndEdges(ctx, docID, graphNodes); err != nil {
		log.Printf("[Saga - Rollback] Neo4j insertion failed for %s. Compensating Qdrant...", docID)

		// Compensation: Rollback the Qdrant insertion
		if compErr := o.qdrant.DeleteDocument(ctx, docID); compErr != nil {
			// CRITICAL: A compensation failure requires manual intervention or a dead-letter queue
			log.Printf("[Saga - CRITICAL ALERT] Compensation failed! Ghost data left in Qdrant for docID: %s. Error: %v", docID, compErr)
		} else {
			log.Printf("[Saga - Rollback] Successfully compensated Qdrant for docID: %s", docID)
		}

		return fmt.Errorf("neo4j insertion failed, transaction rolled back: %w", err)
	}

	log.Printf("[Saga Orchestrator] Ingestion completed successfully for document: %s", docID)
	return nil
}

// DocumentExists checks if the document has already been ingested in both databases.
func (o *Orchestrator) DocumentExists(ctx context.Context, docID string) (bool, error) {
	qExists, err := o.qdrant.DocumentExists(ctx, docID)
	if err != nil {
		return false, fmt.Errorf("failed to check Qdrant for existence: %w", err)
	}
	nExists, err := o.neo4j.DocumentExists(ctx, docID)
	if err != nil {
		return false, fmt.Errorf("failed to check Neo4j for existence: %w", err)
	}

	// Consider it exists if it's in either, to prevent partial or duplicate inserts
	return qExists || nExists, nil
}
