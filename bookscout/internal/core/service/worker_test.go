package service_test

import (
	"bookscout/internal/adapters/destination"
	"bookscout/internal/adapters/tracker"
	"bookscout/internal/config"
	"bookscout/internal/core/domain/models"
	"bookscout/internal/core/service"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// mockBookSource implements ports.BookSource
type mockBookSource struct {
	books    []models.BookMetadata
	content  string
	errFetch error
	errDown  error
}

func (m *mockBookSource) FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]models.BookMetadata, error) {
	if m.errFetch != nil {
		return nil, m.errFetch
	}
	// Simple filtering logic for test
	var newBooks []models.BookMetadata
	for _, b := range m.books {
		if b.AddedAt.Unix() > lastCheckTimestamp {
			newBooks = append(newBooks, b)
		}
	}
	return newBooks, nil
}

func (m *mockBookSource) DownloadBookContent(ctx context.Context, book models.BookMetadata) (io.ReadCloser, error) {
	if m.errDown != nil {
		return nil, m.errDown
	}
	return io.NopCloser(strings.NewReader(m.content)), nil
}

func TestWorkerService_Run(t *testing.T) {
	// 1. Mock Destination Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ingest" {
			t.Errorf("expected path /ingest, got %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Verify multipart
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatal("failed to parse multipart form")
		}

		// Verify metadata
		metaStr := r.FormValue("metadata")
		var meta models.BookMetadata
		if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
			t.Errorf("invalid metadata json: %v", err)
		}
		if meta.Title == "" {
			t.Error("metadata title is empty")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// 2. Setup State Store
	tmpFile, err := os.CreateTemp("", "scout_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	state, err := tracker.NewFileStateStore(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// 3. Setup Config
	cfg := &config.Config{
		WorkerSinceTimestamp: 0,
		WorkerBatchSize:      10,
		WorkerConcurrency:    2,
		StateFilePath:        tmpFile.Name(),
		APIBaseURL:           ts.URL,
	}

	// 4. Setup Mock Source
	now := time.Now()
	mockSrc := &mockBookSource{
		books: []models.BookMetadata{
			{ID: "1", Title: "Book One", AddedAt: now.Add(-10 * time.Minute), DownloadURL: "http://example.com/1.epub"},
			{ID: "2", Title: "Book Two", AddedAt: now.Add(-5 * time.Minute), DownloadURL: "http://example.com/2.epub"},
		},
		content: "dummy content",
	}

	// 5. Setup Destination Adapter
	dest := destination.NewBookSageAPIAdapter(ts.URL)

	// 6. Run Worker
	svc := service.NewWorkerService(cfg, mockSrc, dest, state)

	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("WorkerService.Run failed: %v", err)
	}

	// 7. Verify State
	if state.GetWatermark() < now.Add(-10*time.Minute).Unix() {
		t.Errorf("Watermark not updated correctly, got %d", state.GetWatermark())
	}
	if !state.IsProcessed("1") {
		t.Error("Book 1 should be marked processed")
	}
	if !state.IsProcessed("2") {
		t.Error("Book 2 should be marked processed")
	}
}
