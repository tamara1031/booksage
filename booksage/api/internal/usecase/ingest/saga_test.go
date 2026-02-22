package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/booksage/booksage-api/internal/database/models"
	"github.com/booksage/booksage-api/internal/domain/repository"
)

type mockQdrant struct {
	insertErr error
	deleteErr error
	deleted   bool
}

func (m *mockQdrant) InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error {
	return m.insertErr
}
func (m *mockQdrant) DeleteDocument(ctx context.Context, docID string) error {
	m.deleted = true
	return m.deleteErr
}
func (m *mockQdrant) Search(ctx context.Context, vector []float32, limit int) ([]repository.SearchResult, error) {
	return nil, nil
}
func (m *mockQdrant) Close() error { return nil }

type mockNeo4j struct {
	insertErr error
	deleteErr error
}

func (m *mockNeo4j) InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error {
	return m.insertErr
}
func (m *mockNeo4j) DeleteDocument(ctx context.Context, docID string) error {
	m.deleteErr = nil
	return m.deleteErr
}
func (m *mockNeo4j) SearchChunks(ctx context.Context, query string, limit int) ([]repository.SearchResult, error) {
	return nil, nil
}
func (m *mockNeo4j) Close(ctx context.Context) error { return nil }

type mockRouter struct{}

func (m *mockRouter) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	return &mockLLM{}
}

type mockLLM struct{}

func (m *mockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	if strings.Contains(prompt, "entities") {
		return `{"entities":[], "relations":[]}`, nil
	}
	return "summary", nil
}
func (m *mockLLM) Name() string { return "mock" }

type mockEmbeddingClient struct{}

func (m *mockEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}
func (m *mockEmbeddingClient) Name() string { return "mock_emb" }

func TestSaga_Success(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	docRepo := &MockDocumentRepository{}
	sagaRepo := &MockSagaRepository{}
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []map[string]any{{"text": "chunk1"}}, []map[string]any{{"text": "node1"}})
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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []map[string]any{{"text": "chunk1"}}, []map[string]any{{"text": "node1"}})
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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []map[string]any{{"text": "chunk1"}}, []map[string]any{{"text": "node1"}})
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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

	err := orch.RunIngestionSaga(context.Background(), &models.IngestSaga{ID: 1, DocumentID: 1, Version: 1}, []map[string]any{{"text": "chunk1"}}, []map[string]any{{"text": "node1"}})
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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

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
	orch := NewSagaOrchestrator(q, n, docRepo, sagaRepo, &mockRouter{}, &mockEmbeddingClient{})

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

// ... (rest of the file)

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
