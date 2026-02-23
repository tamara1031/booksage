package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pb "github.com/booksage/booksage-api/gen/pb/booksage/v1"
	"github.com/booksage/booksage-api/internal/domain"
	ingest_usecase "github.com/booksage/booksage-api/internal/usecase/ingest"
	query_usecase "github.com/booksage/booksage-api/internal/usecase/query"
	"google.golang.org/grpc"
)

// --- Mocks ---

// MockLLMClient for QueryHandler testing
type MockLLMClient struct {
	GenerateFunc func(ctx context.Context, prompt string) (string, error)
}

// Ensure MockLLMClient implements domain.LLMClient
var _ domain.LLMClient = (*MockLLMClient)(nil)

func (m *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, prompt)
	}
	return "mock answer", nil
}
func (m *MockLLMClient) Name() string { return "mock-llm" }

// MockTensorClient (infrastructure/port)
type mockTensorClient struct{}

// Ensure mockTensorClient implements domain.TensorEngine
var _ domain.TensorEngine = (*mockTensorClient)(nil)

func (m *mockTensorClient) Embed(_ context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = make([]float32, 128)
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

// MockParserClient (GRPC)
type mockParserClient struct{}

func (m *mockParserClient) Parse(ctx context.Context, opts ...grpc.CallOption) (pb.DocumentParserService_ParseClient, error) {
	return &mockParseClientStream{}, nil
}

type mockParseClientStream struct {
	grpc.ClientStream
}

func (m *mockParseClientStream) Send(req *pb.ParseRequest) error { return nil }
func (m *mockParseClientStream) Recv() (*pb.ParseResponse, error) {
	return nil, io.EOF // Simulate empty or finished stream
}
func (m *mockParseClientStream) CloseSend() error { return nil }

// --- Helper to setup router ---
func createTestRouter() http.Handler {
	// 1. Setup Ingest Dependencies
	// Using exported mocks from ingest package
	docRepo := &ingest_usecase.MockDocumentRepository{}
	sagaRepo := &ingest_usecase.MockSagaRepository{}
	qdrant := ingest_usecase.NewMockQdrantClient()
	neo4j := ingest_usecase.NewMockNeo4jClient()
	tensor := &mockTensorClient{}

	saga := ingest_usecase.NewSagaOrchestrator(qdrant, neo4j, docRepo, sagaRepo, nil, tensor)
	ingestService := ingest_usecase.NewIngestionService(saga, tensor)

	// 2. Setup Query Dependencies
	mockLLM := &MockLLMClient{
		GenerateFunc: func(ctx context.Context, prompt string) (string, error) {
			return "This is the answer.", nil
		},
	}
	// We pass nil retriever to simplify test (Generator handles nil retriever)
	generator := query_usecase.NewGenerator(mockLLM, nil)

	// 3. Handlers
	ingestHandler := NewIngestHandler(saga, ingestService, &mockParserClient{})
	queryHandler := NewQueryHandler(generator)

	return RegisterRoutes(ingestHandler, queryHandler)
}

func TestServer_Healthz(t *testing.T) {
	router := createTestRouter()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_Ingest_Success(t *testing.T) {
	router := createTestRouter()

	// Prepare Multipart Request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "dummy content")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// IngestHandler likely returns 202 Accepted or 200 OK with JSON
	if w.Code != http.StatusAccepted && w.Code != http.StatusOK {
		t.Logf("Response Body: %s", w.Body.String())
		t.Errorf("expected 200/202, got %d", w.Code)
	}

	// Verify JSON
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
	if status, ok := resp["status"]; ok {
		if status != "processing" && status != "completed" {
			t.Errorf("unexpected status: %v", status)
		}
	}
}

func TestServer_Query_Success(t *testing.T) {
	router := createTestRouter()

	payload := map[string]string{
		"query": "What is the book about?",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Check SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}

	// Check body for "This is the answer"
	if !strings.Contains(w.Body.String(), "This is the answer") {
		t.Errorf("response does not contain generated answer: %s", w.Body.String())
	}
}
