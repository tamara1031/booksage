package client_test

import (
	"bookscout/internal/client"
	"bookscout/internal/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBookSageAPIAdapter_Send(t *testing.T) {
	content := "fake book content"
	// Pre-calculate hash manually
	h := sha256.New()
	h.Write([]byte(content))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ingest", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		err := r.ParseMultipartForm(10 << 20)
		assert.NoError(t, err)

		metadata := r.FormValue("metadata")
		assert.NotEmpty(t, metadata)

		file, header, err := r.FormFile("file")
		assert.NoError(t, err)
		defer file.Close()
		assert.Equal(t, "1.epub", header.Filename)

		fileContent, _ := io.ReadAll(file)
		assert.Equal(t, content, string(fileContent))

		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	adapter := client.NewBookSageAPIAdapter(ts.URL)
	book := domain.BookMetadata{ID: "1", DownloadURL: "http://example.com/item/1.epub"}

	hash, err := adapter.Send(context.Background(), book, strings.NewReader(content))
	assert.NoError(t, err)
	assert.Equal(t, expectedHash, hash)
}

func TestBookSageAPIAdapter_GetStatusByHash(t *testing.T) {
	fileHash := "abcdef123456"

	t.Run("Completed", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "hash="+fileHash, r.URL.RawQuery)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "completed",
			})
		}))
		defer ts.Close()

		adapter := client.NewBookSageAPIAdapter(ts.URL)
		status, errMsg, err := adapter.GetStatusByHash(context.Background(), fileHash)
		assert.NoError(t, err)
		assert.Equal(t, "completed", status)
		assert.Empty(t, errMsg)
	})

	t.Run("Failed", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "failed",
				"error":  "extraction error",
			})
		}))
		defer ts.Close()

		adapter := client.NewBookSageAPIAdapter(ts.URL)
		status, errMsg, err := adapter.GetStatusByHash(context.Background(), fileHash)
		assert.NoError(t, err)
		assert.Equal(t, "failed", status)
		assert.Equal(t, "extraction error", errMsg)
	})

	t.Run("NotFound", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		adapter := client.NewBookSageAPIAdapter(ts.URL)
		status, _, err := adapter.GetStatusByHash(context.Background(), fileHash)
		assert.NoError(t, err)
		assert.Equal(t, "NOT_FOUND", status)
	})
}
