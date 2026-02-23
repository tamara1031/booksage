package tracker_test

import (
	"bookscout/internal/tracker"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStateStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := tracker.NewSQLiteStateStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	t.Run("Watermark", func(t *testing.T) {
		// Default watermark should be 0
		assert.Equal(t, int64(0), store.GetWatermark())

		// Update watermark
		err := store.UpdateWatermark(100)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), store.GetWatermark())

		// Update with smaller value should not change watermark
		err = store.UpdateWatermark(50)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), store.GetWatermark())

		// Update with larger value
		err = store.UpdateWatermark(200)
		assert.NoError(t, err)
		assert.Equal(t, int64(200), store.GetWatermark())
	})

	t.Run("DocumentTracking", func(t *testing.T) {
		bookID := "book-1"
		fileHash := "hash-123"

		// Record initial ingestion
		err := store.RecordIngestion(bookID, fileHash)
		assert.NoError(t, err)

		// Check status
		status, exists := store.GetStatus(bookID)
		assert.True(t, exists)
		assert.Equal(t, tracker.StatusProcessing, status)
		assert.False(t, store.IsProcessed(bookID))

		// Update status by hash
		err = store.UpdateStatusByHash(fileHash, tracker.StatusCompleted, "")
		assert.NoError(t, err)

		status, exists = store.GetStatus(bookID)
		assert.True(t, exists)
		assert.Equal(t, tracker.StatusCompleted, status)
		assert.True(t, store.IsProcessed(bookID))

		// Check processing documents (should be empty now)
		docs, err := store.GetProcessingDocuments()
		assert.NoError(t, err)
		assert.Empty(t, docs)

		// Record another and check processing
		err = store.RecordIngestion("book-2", "hash-456")
		assert.NoError(t, err)
		docs, err = store.GetProcessingDocuments()
		assert.NoError(t, err)
		assert.Len(t, docs, 1)
		assert.Equal(t, "book-2", docs[0].ID)
		assert.Equal(t, "hash-456", docs[0].FileHash)
		assert.Equal(t, tracker.StatusProcessing, docs[0].Status)
	})

	t.Run("FailedStatus", func(t *testing.T) {
		bookID := "book-fail"
		fileHash := "hash-fail"

		err := store.RecordIngestion(bookID, fileHash)
		assert.NoError(t, err)

		err = store.UpdateStatusByHash(fileHash, tracker.StatusFailed, "some error")
		assert.NoError(t, err)

		status, exists := store.GetStatus(bookID)
		assert.True(t, exists)
		assert.Equal(t, tracker.StatusFailed, status)
		assert.False(t, store.IsProcessed(bookID))
	})
}
