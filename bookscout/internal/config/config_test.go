package config_test

import (
	"bookscout/internal/config"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		// Clean env for this test
		t.Setenv("SCOUT_OPDS_BASE_URL", "http://example.com") // Required by validation

		cfg, err := config.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, "info", cfg.LogLevel)
		assert.Equal(t, "opds", cfg.BookSourceType)
		assert.Equal(t, "http://api:8080/api/v1", cfg.APIBaseURL)
		assert.Equal(t, int64(0), cfg.WorkerSinceTimestamp)
		assert.Equal(t, 5, cfg.WorkerConcurrency)
		assert.Equal(t, "1h", cfg.WorkerTimeoutStr)
		assert.Equal(t, "scout_state.db", cfg.StateFilePath)
	})

	t.Run("EnvOverrides", func(t *testing.T) {
		t.Setenv("SCOUT_LOG_LEVEL", "debug")
		t.Setenv("SCOUT_OPDS_BASE_URL", "http://opds.local")
		t.Setenv("SCOUT_WORKER_CONCURRENCY", "10")
		t.Setenv("SCOUT_STATE_FILE_PATH", "custom.db")

		cfg, err := config.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, "debug", cfg.LogLevel)
		assert.Equal(t, "http://opds.local", cfg.OPDSBaseURL)
		assert.Equal(t, 10, cfg.WorkerConcurrency)
		assert.Equal(t, "custom.db", cfg.StateFilePath)
	})

	t.Run("Validation", func(t *testing.T) {
		// Missing required OPDS URL when type is opds
		t.Setenv("SCOUT_BOOK_SOURCE_TYPE", "opds")
		t.Setenv("SCOUT_OPDS_BASE_URL", "")

		_, err := config.LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SCOUT_OPDS_BASE_URL is required")

		// Negative concurrency
		t.Setenv("SCOUT_OPDS_BASE_URL", "http://example.com")
		t.Setenv("SCOUT_WORKER_CONCURRENCY", "0")
		_, err = config.LoadConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SCOUT_WORKER_CONCURRENCY must be at least 1")
	})

	t.Run("TimeoutParsing", func(t *testing.T) {
		t.Setenv("SCOUT_OPDS_BASE_URL", "http://example.com")
		t.Setenv("SCOUT_WORKER_TIMEOUT", "2h")

		cfg, err := config.LoadConfig()
		require.NoError(t, err)

		timeout := cfg.GetWorkerTimeout()
		assert.Equal(t, 2.0, timeout.Hours())

		// Test invalid duration
		cfg.WorkerTimeoutStr = "invalid"
		timeout = cfg.GetWorkerTimeout()
		assert.Equal(t, 1.0, timeout.Hours(), "Should default to 1h on error")
	})
}
