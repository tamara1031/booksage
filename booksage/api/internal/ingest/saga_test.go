package ingest

import (
	"context"
	"errors"
	"testing"
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

func TestSaga_Success(t *testing.T) {
	q := &mockQdrant{}
	n := &mockNeo4j{}
	orch := NewOrchestrator(q, n)

	err := orch.RunIngestionSaga(context.Background(), "doc1", []any{"chunk1"}, []any{"node1"})
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
	orch := NewOrchestrator(q, n)

	err := orch.RunIngestionSaga(context.Background(), "doc1", []any{"chunk1"}, []any{"node1"})
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
	orch := NewOrchestrator(q, n)

	err := orch.RunIngestionSaga(context.Background(), "doc1", []any{"chunk1"}, []any{"node1"})
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
	orch := NewOrchestrator(q, n)

	err := orch.RunIngestionSaga(context.Background(), "doc1", []any{"chunk1"}, []any{"node1"})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if !q.deleted {
		t.Errorf("Expected Qdrant to be compensated (DeleteDocument called even if failed)")
	}
}
