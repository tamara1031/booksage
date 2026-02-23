package ingest

import (
	"context"
	"log"

	"github.com/booksage/booksage-api/internal/domain"
)

// MockQdrantClient provides a simple mock of the vector DB for Saga testing
type MockQdrantClient struct{}

func NewMockQdrantClient() *MockQdrantClient {
	return &MockQdrantClient{}
}

func (m *MockQdrantClient) InsertChunks(ctx context.Context, docID string, chunks []map[string]any) error {
	log.Printf("[MockQdrant] Inserted %d chunks for doc %s", len(chunks), docID)
	return nil
}

func (m *MockQdrantClient) DeleteDocument(ctx context.Context, docID string) error {
	log.Printf("[MockQdrant] Deleted vectors for doc %s", docID)
	return nil
}

func (m *MockQdrantClient) Search(ctx context.Context, vector []float32, limit int) ([]domain.SearchResult, error) {
	return nil, nil
}

func (m *MockQdrantClient) Close() error {
	return nil
}

// MockNeo4jClient provides a simple mock of the graph DB for Saga testing
type MockNeo4jClient struct{}

func NewMockNeo4jClient() *MockNeo4jClient {
	return &MockNeo4jClient{}
}

func (m *MockNeo4jClient) InsertNodesAndEdges(ctx context.Context, docID string, nodes []map[string]any, edges []map[string]any) error {
	log.Printf("[MockNeo4j] Inserted %d nodes for doc %s", len(nodes), docID)
	return nil
}

func (m *MockNeo4jClient) DeleteDocument(ctx context.Context, docID string) error {
	log.Printf("[MockNeo4j] Deleted nodes for doc %s", docID)
	return nil
}

func (m *MockNeo4jClient) SearchChunks(ctx context.Context, query string, limit int) ([]domain.SearchResult, error) {
	return nil, nil
}

func (m *MockNeo4jClient) Close(ctx context.Context) error {
	return nil
}

// MockDocumentRepository
type MockDocumentRepository struct{}

func (m *MockDocumentRepository) CreateDocument(ctx context.Context, doc *domain.Document) (int64, error) {
	return 1, nil
}
func (m *MockDocumentRepository) GetDocumentByID(ctx context.Context, id int64) (*domain.Document, error) {
	return &domain.Document{ID: id}, nil
}
func (m *MockDocumentRepository) GetDocumentByHash(ctx context.Context, hash []byte) (*domain.Document, error) {
	// Simulate conflict if hash is specifically marked
	if len(hash) > 0 && hash[0] == 0xF1 { // F1 matches our mock conflict hash
		return &domain.Document{ID: 100, FileHash: hash}, nil
	}
	return nil, nil // Simulate not found
}
func (m *MockDocumentRepository) DeleteDocument(ctx context.Context, id int64) error {
	return nil
}

// MockSagaRepository
type MockSagaRepository struct{}

func (m *MockSagaRepository) CreateSaga(ctx context.Context, saga *domain.IngestSaga) (int64, error) {
	return 1, nil
}
func (m *MockSagaRepository) GetSagaByID(ctx context.Context, id int64) (*domain.IngestSaga, error) {
	return &domain.IngestSaga{ID: id, Version: 1}, nil
}
func (m *MockSagaRepository) GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*domain.IngestSaga, error) {
	if docID == 100 {
		return &domain.IngestSaga{ID: 100, DocumentID: 100, Status: domain.SagaStatusCompleted, Version: 1}, nil
	}
	return nil, nil
}
func (m *MockSagaRepository) UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status domain.SagaStatus, currentStep domain.IngestStep, errorMsg string) error {
	return nil
}
func (m *MockSagaRepository) UpsertSagaStep(ctx context.Context, step *domain.SagaStep) (int64, error) {
	return 1, nil
}
func (m *MockSagaRepository) GetSagaSteps(ctx context.Context, sagaID int64) ([]*domain.SagaStep, error) {
	return nil, nil
}
