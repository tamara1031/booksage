package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/booksage/booksage-api/internal/domain"
)

// Mocks
type mockVectorRepo struct {
	shouldFail bool
}

func (m *mockVectorRepo) Search(ctx context.Context, vector []float32, limit int) ([]domain.SearchResult, error) {
	return []domain.SearchResult{}, nil
}
func (m *mockVectorRepo) InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error {
	if m.shouldFail {
		return errors.New("mock vector error")
	}
	return nil
}
func (m *mockVectorRepo) DeleteDocument(ctx context.Context, docID string) error { return nil }
func (m *mockVectorRepo) Close() error                                           { return nil }

type mockGraphRepo struct {
	shouldFail bool
}

func (m *mockGraphRepo) SearchChunks(ctx context.Context, query string, limit int) ([]domain.SearchResult, error) {
	return []domain.SearchResult{}, nil
}
func (m *mockGraphRepo) InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error {
	if m.shouldFail {
		return errors.New("mock graph error")
	}
	return nil
}
func (m *mockGraphRepo) DeleteDocument(ctx context.Context, docID string) error { return nil }
func (m *mockGraphRepo) Close(ctx context.Context) error                        { return nil }

type mockDocRepo struct{}

func (m *mockDocRepo) CreateDocument(ctx context.Context, doc *domain.Document) (int64, error) {
	return 1, nil
}
func (m *mockDocRepo) GetDocumentByID(ctx context.Context, id int64) (*domain.Document, error) {
	return &domain.Document{ID: id}, nil
}
func (m *mockDocRepo) GetDocumentByHash(ctx context.Context, hash []byte) (*domain.Document, error) {
	return nil, domain.ErrNotFound // Simulate new doc
}
func (m *mockDocRepo) DeleteDocument(ctx context.Context, id int64) error { return nil }

type mockSagaRepo struct {
	status domain.SagaStatus
}

func (m *mockSagaRepo) CreateSaga(ctx context.Context, saga *domain.IngestSaga) (int64, error) {
	return 1, nil
}
func (m *mockSagaRepo) GetSagaByID(ctx context.Context, id int64) (*domain.IngestSaga, error) {
	return &domain.IngestSaga{ID: id, Status: m.status}, nil
}
func (m *mockSagaRepo) GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*domain.IngestSaga, error) {
	return nil, domain.ErrNotFound
}
func (m *mockSagaRepo) UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status domain.SagaStatus, currentStep domain.IngestStep, errorMsg string) error {
	return nil
}
func (m *mockSagaRepo) UpsertSagaStep(ctx context.Context, step *domain.SagaStep) (int64, error) {
	return 1, nil
}
func (m *mockSagaRepo) GetSagaSteps(ctx context.Context, sagaID int64) ([]*domain.SagaStep, error) {
	return nil, nil
}

type mockTensorEngine struct{}

func (m *mockTensorEngine) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2}}, nil
}
func (m *mockTensorEngine) Name() string { return "mock_tensor" }
func (m *mockTensorEngine) Rerank(ctx context.Context, query string, docs []string) ([]float32, error) {
	return make([]float32, len(docs)), nil
}

// LLM is mockLLMClient directly now

type mockLLMClient struct{}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "{}", nil
}
func (m *mockLLMClient) Name() string { return "mock_llm" }

func TestStartOrResumeIngestion_NewDoc(t *testing.T) {
	orch := NewSagaOrchestrator(
		&mockVectorRepo{},
		&mockGraphRepo{},
		&mockDocRepo{},
		&mockSagaRepo{},
		&mockLLMClient{},
		&mockTensorEngine{},
	)

	doc := &domain.Document{FileHash: []byte("hash")}
	saga, err := orch.StartOrResumeIngestion(context.Background(), doc)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if saga.ID != 1 {
		t.Errorf("Expected saga ID 1, got %d", saga.ID)
	}
}

func TestRunIngestionSaga_Success(t *testing.T) {
	orch := NewSagaOrchestrator(
		&mockVectorRepo{},
		&mockGraphRepo{},
		&mockDocRepo{},
		&mockSagaRepo{},
		&mockLLMClient{},
		&mockTensorEngine{},
	)

	saga := &domain.IngestSaga{ID: 1, DocumentID: 1}
	chunks := []map[string]any{{"content": "text"}}
	nodes := []map[string]any{}

	err := orch.RunIngestionSaga(context.Background(), saga, chunks, nodes)
	if err != nil {
		t.Errorf("Expected success, got %v", err)
	}
}

func TestRunIngestionSaga_VectorFail(t *testing.T) {
	orch := NewSagaOrchestrator(
		&mockVectorRepo{shouldFail: true},
		&mockGraphRepo{},
		&mockDocRepo{},
		&mockSagaRepo{},
		&mockLLMClient{},
		&mockTensorEngine{},
	)

	saga := &domain.IngestSaga{ID: 1, DocumentID: 1}
	chunks := []map[string]any{{"content": "text"}}
	nodes := []map[string]any{}

	err := orch.RunIngestionSaga(context.Background(), saga, chunks, nodes)
	if err == nil {
		t.Error("Expected error from vector store")
	}
}

func TestRunIngestionSaga_GraphFail(t *testing.T) {
	orch := NewSagaOrchestrator(
		&mockVectorRepo{},
		&mockGraphRepo{shouldFail: true},
		&mockDocRepo{},
		&mockSagaRepo{},
		&mockLLMClient{},
		&mockTensorEngine{},
	)

	saga := &domain.IngestSaga{ID: 1, DocumentID: 1}
	chunks := []map[string]any{{"content": "text"}}
	nodes := []map[string]any{}

	err := orch.RunIngestionSaga(context.Background(), saga, chunks, nodes)
	if err == nil {
		t.Error("Expected error from graph store")
	}
}
