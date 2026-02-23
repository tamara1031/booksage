package booksage_test

import (
	"bookscout/internal/scout/domain"
	"bookscout/internal/scout/infra/booksage"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIIngestor_Ingest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"saga_id": 1, "status": "processing", "hash": "test-hash-123"}`))
	}))
	defer ts.Close()

	ingestor := booksage.NewAPIIngestor(ts.URL)
	book := domain.Book{ID: "1", DownloadURL: "http://example.com/1.epub"}
	hash, err := ingestor.Ingest(context.Background(), book, strings.NewReader("content"))
	assert.NoError(t, err)
	assert.Equal(t, "test-hash-123", hash)
}
