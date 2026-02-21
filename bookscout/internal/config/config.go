package config

import (
	"fmt"
	"log"
	"sync"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string `env:"SCOUT_LOG_LEVEL" envDefault:"info"`

	BookSourceType string `env:"SCOUT_BOOK_SOURCE_TYPE" envDefault:"opds"`
	OPDSBaseURL    string `env:"SCOUT_OPDS_BASE_URL"`
	OPDSUsername   string `env:"SCOUT_OPDS_USERNAME"`
	OPDSPassword   string `env:"SCOUT_OPDS_PASSWORD"`

	APIBaseURL string `env:"SCOUT_API_BASE_URL" envDefault:"http://api:8080/api/v1"`

	WorkerSinceTimestamp int64 `env:"SCOUT_WORKER_SINCE_TIMESTAMP" envDefault:"0"`
	WorkerConcurrency    int   `env:"SCOUT_WORKER_CONCURRENCY" envDefault:"5"`
	WorkerBatchSize      int   `env:"SCOUT_WORKER_BATCH_SIZE" envDefault:"0"`

	MaxBookSizeBytes int64 `env:"SCOUT_MAX_BOOK_SIZE_BYTES" envDefault:"52428800"`
}

func (c *Config) Validate() error {
	if c.BookSourceType == "opds" && c.OPDSBaseURL == "" {
		return fmt.Errorf("SCOUT_OPDS_BASE_URL is required when SCOUT_BOOK_SOURCE_TYPE is opds")
	}

	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("SCOUT_WORKER_CONCURRENCY must be at least 1")
	}

	if c.WorkerBatchSize < 0 {
		return fmt.Errorf("SCOUT_WORKER_BATCH_SIZE cannot be negative")
	}

	if c.WorkerSinceTimestamp < 0 {
		return fmt.Errorf("SCOUT_WORKER_SINCE_TIMESTAMP cannot be negative")
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
