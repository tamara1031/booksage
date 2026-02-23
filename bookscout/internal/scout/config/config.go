package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	LogLevel string `mapstructure:"log_level"`

	BookSourceType string `mapstructure:"book_source_type"`
	OPDSBaseURL    string `mapstructure:"opds_base_url"`
	OPDSUsername   string `mapstructure:"opds_username"`
	OPDSPassword   string `mapstructure:"opds_password"`

	APIBaseURL string `mapstructure:"api_base_url"`

	WorkerSinceTimestamp int64 `mapstructure:"worker_since_timestamp"`
	WorkerConcurrency    int   `mapstructure:"worker_concurrency"`
	WorkerBatchSize      int   `mapstructure:"worker_batch_size"`
	WorkerDelayMS        int   `mapstructure:"worker_delay_ms"`

	MaxBookSizeBytes int64 `mapstructure:"max_book_size_bytes"`

	WorkerTimeoutStr string `mapstructure:"worker_timeout"`

	StateFilePath string `mapstructure:"state_file_path"`
}

func (c *Config) GetWorkerTimeout() time.Duration {
	d, err := time.ParseDuration(c.WorkerTimeoutStr)
	if err != nil {
		log.Printf("Invalid duration string %s, defaulting to 1h", c.WorkerTimeoutStr)
		return time.Hour
	}
	return d
}

func (c *Config) Validate() error {
	if c.BookSourceType == "opds" && c.OPDSBaseURL == "" {
		return fmt.Errorf("SCOUT_OPDS_BASE_URL is required when SCOUT_BOOK_SOURCE_TYPE is opds")
	}
	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("SCOUT_WORKER_CONCURRENCY must be at least 1")
	}
	return nil
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("SCOUT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("log_level", "info")
	v.SetDefault("book_source_type", "opds")
	v.SetDefault("opds_base_url", "")
	v.SetDefault("opds_username", "")
	v.SetDefault("opds_password", "")
	v.SetDefault("api_base_url", "http://api:8080/api/v1")
	v.SetDefault("worker_since_timestamp", 0)
	v.SetDefault("worker_concurrency", 5)
	v.SetDefault("worker_batch_size", 0)
	v.SetDefault("worker_delay_ms", 0)
	v.SetDefault("worker_timeout", "1h")
	v.SetDefault("state_file_path", "scout_state.db")
	v.SetDefault("max_book_size_bytes", 52428800)

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
