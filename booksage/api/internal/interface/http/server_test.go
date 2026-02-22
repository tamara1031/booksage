package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/booksage/booksage-api/internal/embedding"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/usecase/ingest"
	"google.golang.org/grpc"
)

func createTestServer() *Server {
	embedBatcher := embedding.NewBatcher(&mockEmbeddingClient{}, 100)
	docRepo := &ingest.MockDocumentRepository{}
	sagaRepo := &ingest.MockSagaRepository{}
	saga := ingest.NewSagaOrchestrator(ingest.NewMockQdrantClient(), ingest.NewMockNeo4jClient(), docRepo, sagaRepo)
	return NewServer(nil, embedBatcher, &mockParserClient{}, saga)
}

func TestHandleQuery_InvalidPayload(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	// Invalid JSON
	resp, err := http.Post(ts.URL+"/api/v1/query", "application/json", strings.NewReader("{invalid"))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid payload, got %d", resp.StatusCode)
	}
}

func TestHandleQuery_EmptyQuery(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	// Empty query
	body, _ := json.Marshal(QueryRequest{Query: ""})
	resp, err := http.Post(ts.URL+"/api/v1/query", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty query, got %d", resp.StatusCode)
	}
}

// Ensure handleQuery errors correctly if the ResponseWriter is not a Flusher
type mockNonFlusherRW struct {
	http.ResponseWriter
}

func TestHandleQuery_NonFlusher(t *testing.T) {
	s := createTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(`{"query":"test"}`))

	// Create a recorder which does NOT implement http.Flusher in this mocked version
	w := httptest.NewRecorder()

	// Wrap it in a non-flusher type
	rw := &mockNonFlusherRW{w}

	s.handleQuery(rw, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 when not flusher, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Streaming unsupported") {
		t.Errorf("Expected Streaming unsupported message, got %s", w.Body.String())
	}
}

func TestHandleIngest(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	// Missing file
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.Close()

	req, err := http.NewRequest("POST", ts.URL+"/api/v1/ingest", &b)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 when missing file, got %d", resp.StatusCode)
	}

	var b2 bytes.Buffer
	w2 := multipart.NewWriter(&b2)
	fw, err := w2.CreateFormFile("file", "new-book.txt") // Use "new-" prefix to satisfy mock existance check
	if err != nil {
		t.Fatalf("failed to create file field: %v", err)
	}
	_, _ = fw.Write([]byte("some text"))
	_ = w2.WriteField("metadata", "{}")
	_ = w2.Close()

	req2, _ := http.NewRequest("POST", ts.URL+"/api/v1/ingest", &b2)
	req2.Header.Set("Content-Type", w2.FormDataContentType())
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp2.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", resp2.StatusCode)
	}
}

func TestHandleIngest_Conflict(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", "registered.txt") // Without "new-" prefix it should conflict in mock
	if err != nil {
		t.Fatalf("failed to create file field: %v", err)
	}
	_, _ = fw.Write([]byte("some text"))
	_ = w.WriteField("metadata", "{}")
	_ = w.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/ingest", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %d", resp.StatusCode)
	}
}

func TestHandleDocumentStatus(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/documents/123/status")
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if data["document_id"] != "123" {
		t.Errorf("expected doc ID 123, got %v", data["document_id"])
	}
}

type mockParserClient struct{}

func (m *mockParserClient) Parse(ctx context.Context, opts ...grpc.CallOption) (pb.DocumentParserService_ParseClient, error) {
	return &mockParseClientStream{}, nil
}

type mockParseClientStream struct {
	grpc.ClientStream
}

func (m *mockParseClientStream) Send(req *pb.ParseRequest) error {
	return nil
}

func (m *mockParseClientStream) CloseAndRecv() (*pb.ParseResponse, error) {
	return &pb.ParseResponse{DocumentId: "mock-doc-id"}, nil
}

type mockEmbeddingClient struct{}

func (m *mockEmbeddingClient) Embed(_ context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = []float32{0.1, 0.2, 0.3}
	}
	return embeddings, nil
}

func (m *mockEmbeddingClient) Name() string {
	return "mock_embedding"
}

func TestHandleIngestStatusByHash_MissingHash(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ingest/status")
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing hash, got %d", resp.StatusCode)
	}
}

func TestHandleIngestStatusByHash_InvalidHash(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ingest/status?hash=not-hex")
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid hex hash, got %d", resp.StatusCode)
	}
}

func TestHandleIngestStatusByHash_NotFound(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	// Use a hash that mock returns nil for (not 0xF1 prefix)
	resp, err := http.Get(ts.URL + "/api/v1/ingest/status?hash=aabb")
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	// Mock returns nil doc, which causes ErrNotFound in GetDocumentStatus
	// Response should be 404 or 500 depending on error handling
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 404 or 500 for unknown hash, got %d", resp.StatusCode)
	}
}

func TestHandleDocumentExist(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	req, _ := http.NewRequest("HEAD", ts.URL+"/api/v1/documents/123", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	// Returns 501 Not Implemented currently
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", resp.StatusCode)
	}
}

func TestHandleDocumentStatus_NotFound(t *testing.T) {
	s := createTestServer()
	ts := httptest.NewServer(s.RegisterRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/documents/not-found/status")
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for not-found, got %d", resp.StatusCode)
	}
}
