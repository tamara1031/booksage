package ingest

import (
	"context"
	"log"

	"github.com/booksage/booksage-api/internal/database/models"
)

// MockQdrantClient provides a simple mock of the vector DB for Saga testing
type MockQdrantClient struct{}

func NewMockQdrantClient() *MockQdrantClient {
	return &MockQdrantClient{}
}

func (m *MockQdrantClient) InsertChunks(ctx context.Context, docID string, chunks []any) error {
	log.Printf("[MockQdrant] Inserted %d chunks for doc %s", len(chunks), docID)
	// Return nil to simulate success, or uncomment to simulate failure:
	// return fmt.Errorf("simulated Qdrant failure")
	return nil
}

func (m *MockQdrantClient) DeleteDocument(ctx context.Context, docID string) error {
	log.Printf("[MockQdrant] Deleted vectors for doc %s", docID)
	return nil
}

func (m *MockQdrantClient) DocumentExists(ctx context.Context, docID string) (bool, error) {
	if docID == "registered.txt" {
		return true, nil
	}
	return false, nil
}

// MockNeo4jClient provides a simple mock of the graph DB for Saga testing
type MockNeo4jClient struct{}

func NewMockNeo4jClient() *MockNeo4jClient {
	return &MockNeo4jClient{}
}

func (m *MockNeo4jClient) InsertNodesAndEdges(ctx context.Context, docID string, nodes []any) error {
	log.Printf("[MockNeo4j] Inserted %d nodes for doc %s", len(nodes), docID)
	// Return nil to simulate success, or uncomment to simulate failure:
	// return fmt.Errorf("simulated Neo4j failure")
	return nil
}

func (m *MockNeo4jClient) DeleteDocumentNodes(ctx context.Context, docID string) error {
	log.Printf("[MockNeo4j] Deleted nodes for doc %s", docID)
	return nil
}

func (m *MockNeo4jClient) DocumentExists(ctx context.Context, docID string) (bool, error) {
	if docID == "registered.txt" {
		return true, nil
	}
	return false, nil
}

// MockDocumentRepository
type MockDocumentRepository struct{}

func (m *MockDocumentRepository) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	return 1, nil
}
func (m *MockDocumentRepository) GetDocumentByID(ctx context.Context, id int64) (*models.Document, error) {
	return &models.Document{ID: id}, nil
}
func (m *MockDocumentRepository) GetDocumentByHash(ctx context.Context, hash []byte) (*models.Document, error) {
	// Simulate conflict if hash is specifically marked
	if len(hash) > 0 && hash[0] == 0xF1 { // F1 matches our mock conflict hash
		return &models.Document{ID: 100, FileHash: hash}, nil
	}
	return nil, nil // Simulate not found
}
func (m *MockDocumentRepository) DeleteDocument(ctx context.Context, id int64) error {
	return nil
}

// MockSagaRepository
type MockSagaRepository struct{}

func (m *MockSagaRepository) CreateSaga(ctx context.Context, saga *models.IngestSaga) (int64, error) {
	return 1, nil
}
func (m *MockSagaRepository) GetSagaByID(ctx context.Context, id int64) (*models.IngestSaga, error) {
	return &models.IngestSaga{ID: id, Version: 1}, nil
}
func (m *MockSagaRepository) GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*models.IngestSaga, error) {
	if docID == 100 {
		return &models.IngestSaga{ID: 100, DocumentID: 100, Status: models.SagaStatusCompleted, Version: 1}, nil
	}
	return nil, nil
}
func (m *MockSagaRepository) UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status models.SagaStatus, currentStep models.IngestStep, errorMsg string) error {
	return nil
}
func (m *MockSagaRepository) UpsertSagaStep(ctx context.Context, step *models.SagaStep) (int64, error) {
	return 1, nil
}
func (m *MockSagaRepository) GetSagaSteps(ctx context.Context, sagaID int64) ([]*models.SagaStep, error) {
	return nil, nil
}
