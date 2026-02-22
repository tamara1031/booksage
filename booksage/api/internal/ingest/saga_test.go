package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/booksage/booksage-api/internal/database/models"
)

type mockQdrant struct {
	insertErr error
	deleteErr error
	deleted   bool
}

func (m *mockQdrant) InsertChunks(ctx context.Context, docID string, chunks []any) error {
	return m.insertErr
}
func (m *mockQdrant) DeleteDocument(ctx context.Context, docID string) error {
	m.deleted = true
	return m.deleteErr
}
func (m *mockQdrant) DocumentExists(ctx context.Context, docID string) (bool, error) {
	return false, nil
}

type mockNeo4j struct {
	insertErr error
	deleteErr error
}

func (m *mockNeo4j) InsertNodesAndEdges(ctx context.Context, docID string, nodes []any) error {
	return m.insertErr
}
func (m *mockNeo4j) DeleteDocumentNodes(ctx context.Context, docID string) error {
	return m.deleteErr
}
func (m *mockNeo4j) DocumentExists(ctx context.Context, docID string) (bool, error) {
	return false, nil
}

func TestSaga_Success(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []any{"chunk1"}, []any{"node1"})
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}

	if q.deleted {
		t.Errorf("Expected no compensation on success")
	}
}

func TestSaga_QdrantFails(t *testing.T) {
	q := &mockQdrant{insertErr: errors.New("qdrant error")}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []any{"chunk1"}, []any{"node1"})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != "qdrant insertion failed: qdrant error" {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should not compensate because insert didn't succeed
	if q.deleted {
		t.Errorf("Expected no compensation if qdrant insert fails")
	}
}

func TestSaga_Neo4jFails_CompensatesQdrant(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{insertErr: errors.New("neo4j error")}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []any{"chunk1"}, []any{"node1"})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !q.deleted {
		t.Errorf("Expected Qdrant to be compensated (DeleteDocument called)")
	}
}

func TestSaga_Neo4jFails_CompensationFails(t *testing.T) {
	q := &mockQdrant{deleteErr: errors.New("delete error")}
	n := &mockNeo4j{insertErr: errors.New("neo4j error")}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []any{"chunk1"}, []any{"node1"})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !q.deleted {
		t.Errorf("Expected Qdrant to be compensated (DeleteDocument called even if failed)")
	}
}

func TestStartOrResumeIngestion_NewDocument(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	doc := &models.Document{
		FileHash: []byte{0xAA, 0xBB}, // Not 0xF1, so mock returns nil (new doc)
		Title:    "New Book",
	}

	saga, err := orch.StartOrResumeIngestion(context.Background(), doc)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if saga == nil {
		t.Fatal("Expected saga, got nil")
	}
	if saga.Status != models.SagaStatusPending {
		t.Errorf("Expected pending status, got %v", saga.Status)
	}
	if saga.CurrentStep != models.StepParsing {
		t.Errorf("Expected parsing step, got %v", saga.CurrentStep)
	}
}

func TestStartOrResumeIngestion_AlreadyIngested(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	// 0xF1 triggers mock to return existing doc with ID=100
	// ID=100 triggers mock saga repo to return completed saga
	doc := &models.Document{
		FileHash: []byte{0xF1, 0x00},
		Title:    "Already Ingested Book",
	}

	_, err := orch.StartOrResumeIngestion(context.Background(), doc)
	if err == nil {
		t.Fatal("Expected error for already ingested document")
	}
	if !errors.Is(err, nil) {
		// Check error message contains "already ingested"
		if err.Error() != "document already ingested: f100" {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestStartOrResumeIngestion_ExistingDocNoSaga(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	// Create a custom docRepo that returns an existing doc but with a non-100 ID
	// so the saga repo returns nil (no existing saga)
	docRepo := &mockDocRepoWithID{id: 50}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	doc := &models.Document{
		FileHash: []byte{0xF1, 0x01},
		Title:    "Existing Doc No Saga",
	}

	saga, err := orch.StartOrResumeIngestion(context.Background(), doc)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if saga == nil {
		t.Fatal("Expected saga, got nil")
	}
}

func TestGetDocumentStatus(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewOrchestrator(q, n, docRepo, sagaRepo)

	// 0xF1 hash → doc ID 100 → completed saga
	saga, err := orch.GetDocumentStatus(context.Background(), []byte{0xF1, 0x00})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if saga == nil {
		t.Fatal("Expected saga, got nil")
	}
	if saga.Status != models.SagaStatusCompleted {
		t.Errorf("Expected completed status, got %v", saga.Status)
	}
}

// mockDocRepoWithID returns a fixed doc for any hash query
type mockDocRepoWithID struct {
	id int64
}

func (m *mockDocRepoWithID) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	return m.id, nil
}
func (m *mockDocRepoWithID) GetDocumentByID(ctx context.Context, id int64) (*models.Document, error) {
	return &models.Document{ID: id}, nil
}
func (m *mockDocRepoWithID) GetDocumentByHash(ctx context.Context, hash []byte) (*models.Document, error) {
	if len(hash) > 0 && hash[0] == 0xF1 {
		return &models.Document{ID: m.id, FileHash: hash}, nil
	}
	return nil, nil
}
func (m *mockDocRepoWithID) DeleteDocument(ctx context.Context, id int64) error {
	return nil
}
