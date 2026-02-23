package worker_test

import (
	"bookscout/internal/client"
	"bookscout/internal/config"
	"bookscout/internal/domain"
	"bookscout/internal/tracker"
	"bookscout/internal/worker"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockBookSource implements worker.BookSource
type mockBookSource struct {
	books    []domain.BookMetadata
	content  string
	errFetch error
	errDown  error
}

func (m *mockBookSource) FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]domain.BookMetadata, error) {
	if m.errFetch != nil {
		return nil, m.errFetch
	}
	// Simple filtering logic for test
	var newBooks []domain.BookMetadata
	for _, b := range m.books {
		if b.AddedAt.Unix() > lastCheckTimestamp {
			newBooks = append(newBooks, b)
		}
	}
	return newBooks, nil
}

func (m *mockBookSource) DownloadBookContent(ctx context.Context, book domain.BookMetadata) (io.ReadCloser, error) {
	if m.errDown != nil {
		return nil, m.errDown
	}
	return io.NopCloser(strings.NewReader(m.content)), nil
}

func TestService_Run(t *testing.T) {
	// 1. Mock Destination Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/ingest" {
			// In our simplified test, we don't need to actually hash, just return success
			w.WriteHeader(http.StatusAccepted)
			return
		}

		if r.Method == "GET" && r.URL.Path == "/ingest/status" {
			hash := r.URL.Query().Get("hash")
			if hash == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "completed",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 2. Setup State Store
	tmpDB := filepath.Join(t.TempDir(), "scout_test.db")
	state, err := tracker.NewSQLiteStateStore(tmpDB)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()

	// 3. Setup Config
	cfg := &config.Config{
		WorkerSinceTimestamp: 0,
		WorkerBatchSize:      10,
		WorkerConcurrency:    2,
		StateFilePath:        tmpDB,
		APIBaseURL:           ts.URL,
		WorkerTimeoutStr:     "1h", // Added this
	}

	// 4. Setup Mock Source
	now := time.Now()
	mockSrc := &mockBookSource{
		books: []domain.BookMetadata{
			{ID: "1", Title: "Book One", AddedAt: now.Add(-10 * time.Minute), DownloadURL: "http://example.com/1.epub"},
			{ID: "2", Title: "Book Two", AddedAt: now.Add(-5 * time.Minute), DownloadURL: "http://example.com/2.epub"},
		},
		content: "dummy content",
	}

	// 5. Setup Destination Adapter
	dest := client.NewBookSageAPIAdapter(ts.URL)

	// 6. Run Worker
	svc := worker.NewService(cfg, mockSrc, dest, state)

	// FIRST RUN: Should scrape and record as PROCESSING
	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("First Run failed: %v", err)
	}

	status, exists := state.GetStatus("1")
	if !exists || status != tracker.StatusProcessing {
		t.Errorf("Book 1 should be PROCESSING, got %v", status)
	}

	// SECOND RUN: Should sync status by hash and mark as COMPLETED
	if err := svc.Run(context.Background()); err != nil {
		t.Fatalf("Second Run failed: %v", err)
	}

	if !state.IsProcessed("1") {
		t.Error("Book 1 should be marked COMPLETED after second run")
	}
}
