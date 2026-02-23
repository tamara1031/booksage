package config_test

import (
	"bookscout/internal/scout/config"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		os.Setenv("SCOUT_OPDS_BASE_URL", "http://debug.example.com")
		defer os.Unsetenv("SCOUT_OPDS_BASE_URL")

		cfg, err := config.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, "http://debug.example.com", cfg.OPDSBaseURL)
	})
}
