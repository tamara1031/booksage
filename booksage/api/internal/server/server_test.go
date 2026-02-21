package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleQuery_InvalidPayload(t *testing.T) {
	s := NewServer(nil, nil, nil, nil)
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
	s := NewServer(nil, nil, nil, nil)
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
	s := NewServer(nil, nil, nil, nil)
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
	s := NewServer(nil, nil, nil, nil)
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
	s := NewServer(nil, nil, nil, nil)
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
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}
}

func TestHandleDocumentStatus(t *testing.T) {
	s := NewServer(nil, nil, nil, nil)
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
