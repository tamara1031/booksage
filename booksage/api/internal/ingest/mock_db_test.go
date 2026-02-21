package ingest

import (
	"context"
	"testing"
)

func TestMockQdrant(t *testing.T) {
	q := NewMockQdrantClient()
	err := q.InsertChunks(context.Background(), "doc", nil)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	err = q.DeleteDocument(context.Background(), "doc")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

func TestMockNeo4j(t *testing.T) {
	n := NewMockNeo4jClient()
	err := n.InsertNodesAndEdges(context.Background(), "doc", nil)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	err = n.DeleteDocumentNodes(context.Background(), "doc")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}
