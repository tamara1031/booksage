package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/booksage/booksage-api/internal/ingest"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/query"
	"google.golang.org/grpc"
)

// Mock objects
type mockTensorClient struct{}

func (m *mockTensorClient) Embed(_ context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = make([]float32, 128) // Mock 128d vector
	}
	return embeddings, nil
}
func (m *mockTensorClient) Rerank(_ context.Context, query string, docs []string) ([]float32, error) {
	scores := make([]float32, len(docs))
	for i := range docs {
		scores[i] = 0.9
	}
	return scores, nil
}

type mockParserClient struct{}

func (m *mockParserClient) Parse(ctx context.Context, opts ...grpc.CallOption) (pb.DocumentParserService_ParseClient, error) {
	return &mockParseClientStream{}, nil
}

type mockParseClientStream struct {
	grpc.ClientStream
	sent int
}

func (m *mockParseClientStream) Send(req *pb.ParseRequest) error { return nil }
func (m *mockParseClientStream) Recv() (*pb.ParseResponse, error) {
	if m.sent > 0 {
		return nil, io.EOF
	}
	m.sent++
	return &pb.ParseResponse{
		DocumentId: "mock-doc-id",
		Documents: []*pb.RawDocument{
			{Content: "Mock content", PageNumber: 1, Type: "text"},
		},
	}, nil
}
func (m *mockParseClientStream) CloseSend() error { return nil }

func createTestHandler() http.Handler {
	// Mocks
	tensor := &mockTensorClient{}
	docRepo := &ingest.MockDocumentRepository{}
	sagaRepo := &ingest.MockSagaRepository{}

	// Services
	saga := ingest.NewSagaOrchestrator(ingest.NewMockQdrantClient(), ingest.NewMockNeo4jClient(), docRepo, sagaRepo, nil, tensor)
	ingestService := ingest.NewIngestionService(saga, tensor)

	ingestHandler := ingest.NewHandler(saga, ingestService, &mockParserClient{})

	queryHandler := query.NewHandler(nil) // Generator nil, will panic if used for real logic

	return RegisterRoutes(ingestHandler, queryHandler)
}

func TestRoutes(t *testing.T) {
	handler := createTestHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 1. Healthz
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("Healthz failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Healthz status: %d", resp.StatusCode)
	}

	// 2. Ingest (POST) - Missing file -> 400
	resp, err = http.Post(ts.URL+"/api/v1/ingest", "multipart/form-data", nil)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}
	// Content-Type missing boundary, likely 400.
	if resp.StatusCode != 400 {
		t.Errorf("Ingest expected 400, got %d", resp.StatusCode)
	}
}

// Ensure handleQuery handles invalid json
func TestHandleQuery_InvalidJSON(t *testing.T) {
	handler := createTestHandler()
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/query", "application/json", strings.NewReader("{invalid"))
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Query expected 400, got %d", resp.StatusCode)
	}
}
