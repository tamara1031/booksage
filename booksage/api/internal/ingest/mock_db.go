package ingest

import (
	"context"
	"log"
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
