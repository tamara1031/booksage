package sqlite_test

import (
	"bookscout/internal/scout/domain"
	"bookscout/internal/scout/infra/sqlite"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteRepository(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := sqlite.NewSQLiteRepository(dbPath)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	wm, _ := repo.GetWatermark(ctx)
	assert.Equal(t, int64(0), wm)

	err = repo.UpdateWatermark(ctx, 100)
	assert.NoError(t, err)
	wm, _ = repo.GetWatermark(ctx)
	assert.Equal(t, int64(100), wm)

	err = repo.RecordIngestion(ctx, "book1", "hash1")
	assert.NoError(t, err)
	status, exists, _ := repo.GetStatus(ctx, "book1")
	assert.True(t, exists)
	assert.Equal(t, domain.StatusProcessing, status)
}
