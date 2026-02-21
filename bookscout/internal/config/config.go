package config

import (
	"fmt"
	"log"
	"sync"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	APIPort  int    `env:"BS_PORT" envDefault:"8000"`
	LogLevel string `env:"BS_LOG_LEVEL" envDefault:"info"`

	BookSourceType string `env:"BS_BOOK_SOURCE_TYPE" envDefault:"opds"`
	OPDSBaseURL    string `env:"BS_OPDS_BASE_URL"`
	OPDSUsername   string `env:"BS_OPDS_USERNAME"`
	OPDSPassword   string `env:"BS_OPDS_PASSWORD"`

	APIBaseURL string `env:"BS_API_BASE_URL" envDefault:"http://api:8080/api/v1"`

	WorkerSinceTimestamp int64 `env:"BS_WORKER_SINCE_TIMESTAMP" envDefault:"0"`
	WorkerConcurrency    int   `env:"BS_WORKER_CONCURRENCY" envDefault:"5"`
	WorkerBatchSize      int   `env:"BS_WORKER_BATCH_SIZE" envDefault:"0"`

	MaxBookSizeBytes int64 `env:"BS_MAX_BOOK_SIZE_BYTES" envDefault:"52428800"`
}

func (c *Config) Validate() error {
	if c.BookSourceType == "opds" && c.OPDSBaseURL == "" {
		return fmt.Errorf("BS_OPDS_BASE_URL is required when BS_BOOK_SOURCE_TYPE is opds")
	}

	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("BS_PORT must be between 1 and 65535")
	}

	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("BS_WORKER_CONCURRENCY must be at least 1")
	}

	if c.WorkerBatchSize < 0 {
		return fmt.Errorf("BS_WORKER_BATCH_SIZE cannot be negative")
	}

	if c.WorkerSinceTimestamp < 0 {
		return fmt.Errorf("BS_WORKER_SINCE_TIMESTAMP cannot be negative")
	}

	return nil
}

var (
	cfg  *Config
	once sync.Once
)

func GetConfig() *Config {
	once.Do(func() {
		_ = godotenv.Load()
		cfg = &Config{}
		if err := env.Parse(cfg); err != nil {
			log.Fatalf("failed to parse config: %v", err)
		}
		if err := cfg.Validate(); err != nil {
			log.Fatalf("config validation failed: %v", err)
		}
	})
	return cfg
}
