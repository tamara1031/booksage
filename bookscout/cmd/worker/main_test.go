package main

import (
	"bookscout/internal/config"
	"bookscout/internal/core/domain/models"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockBookSource is a simple mock for ports.BookDataSource
type mockBookSource struct {
	books    []models.BookMetadata
	content  []byte
	errFetch error
	errDown  error
}

func (m *mockBookSource) FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]models.BookMetadata, error) {
	if m.errFetch != nil {
		return nil, m.errFetch
	}
	return m.books, nil
}

func (m *mockBookSource) DownloadBookContent(ctx context.Context, book models.BookMetadata) ([]byte, error) {
	if m.errDown != nil {
		return nil, m.errDown
	}
	return m.content, nil
}

func TestRun_Success(t *testing.T) {
	// Start a dummy test server to act as the API for ingestion
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock existence check (HEAD)
		if r.Method == "HEAD" {
			if strings.Contains(r.URL.Path, "already-exists") {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !strings.HasPrefix(r.URL.Path, "/ingest") {
			t.Errorf("expected path /ingest, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify multipart form
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatal("failed to parse multipart form")
		}

		metadataJSON := r.FormValue("metadata")
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			t.Errorf("failed to unmarshal metadata JSON: %v", err)
		}

		title := metadata["title"]
		if title != "Test Book" && title != "Test Book 2" {
			t.Errorf("Expected title 'Test Book' or 'Test Book 2' in metadata, got '%s'", title)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatal("failed to get file from form")
		}
		defer file.Close()

		w.WriteHeader(http.StatusAccepted) // Aligned with API
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
	}))
	defer ts.Close()

	cfg := &config.Config{
		APIBaseURL:           ts.URL,
		WorkerSinceTimestamp: 1700000000,
		WorkerBatchSize:      10,
		WorkerConcurrency:    2,
	}

	mockSource := &mockBookSource{
		books: []models.BookMetadata{
			{Title: "Test Book", ID: "1", Author: "Author A"},
			{Title: "Test Book 2", ID: "2", Author: "Author B"},
		},
		content: []byte("dummy pdf content..."),
	}

	err := Run(context.Background(), cfg, mockSource)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRun_NoBooks(t *testing.T) {
	cfg := &config.Config{
		WorkerSinceTimestamp: 0,
		WorkerBatchSize:      10,
		WorkerConcurrency:    2,
	}

	mockSource := &mockBookSource{
		books: []models.BookMetadata{}, // No books
	}

	err := Run(context.Background(), cfg, mockSource)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestIngestToAPI_ErrorResponses(t *testing.T) {
	// Server returning 500 Internal Server Error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	book := models.BookMetadata{Title: "Test Book"}
	err := ingestToAPI(ts.URL, book, []byte("data"))

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 500 status error, got %v", err)
	}
}

func TestIngestToAPI_InvalidURL(t *testing.T) {
	book := models.BookMetadata{Title: "Test Book"}
	err := ingestToAPI("http://invalid-url-that-does-not-exist", book, []byte("data"))

	if err == nil {
		t.Fatal("expected error due to invalid URL, got nil")
	}
}
