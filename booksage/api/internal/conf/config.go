package conf

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Client  ClientConfig
	Model   ModelConfig
	DB      DBConfig
	Timeout TimeoutConfig
}

type ClientConfig struct {
	WorkerAddr string
}

type ModelConfig struct {
	OllamaHost  string
	OllamaLLM   string
	OllamaEmbed string
	InfinityURL string // URL for Infinity Tensor Engine
}

type DBConfig struct {
	QdrantHost       string
	QdrantPort       int
	QdrantCollection string
	Neo4jURI         string
	Neo4jUser        string
	Neo4jPassword    string
}

type TimeoutConfig struct {
	Default   time.Duration
	Embedding time.Duration
	Parser    time.Duration
}

func (c *Config) Validate() error {
	if c.Client.WorkerAddr == "" {
		return fmt.Errorf("SAGE_CLIENT_WORKER_ADDR is required")
	}
	return nil
}

func Load() *Config {
	cfg := &Config{
		Client: ClientConfig{
			WorkerAddr: getEnv("SAGE_CLIENT_WORKER_ADDR", "localhost:50051"),
		},
		Model: ModelConfig{
			OllamaHost:  getEnv("SAGE_MODEL_OLLAMA_HOST", "http://localhost:11434"),
			OllamaLLM:   getEnv("SAGE_MODEL_OLLAMA_LLM", "llama3"),
			OllamaEmbed: getEnv("SAGE_MODEL_OLLAMA_EMBED", "nomic-embed-text"),
			InfinityURL: getEnv("SAGE_MODEL_INFINITY_URL", "http://localhost:7997"),
		},
		DB: DBConfig{
			QdrantHost:       getEnv("SAGE_DB_QDRANT_HOST", "localhost"),
			QdrantPort:       getEnvInt("SAGE_DB_QDRANT_PORT", 6334),
			QdrantCollection: getEnv("SAGE_DB_QDRANT_COLLECTION", "booksage"),
			Neo4jURI:         getEnv("SAGE_DB_NEO4J_URI", "neo4j://localhost:7687"),
			Neo4jUser:        getEnv("SAGE_DB_NEO4J_USER", "neo4j"),
			Neo4jPassword:    getEnv("SAGE_DB_NEO4J_PASSWORD", "booksage_dev"),
		},
		Timeout: TimeoutConfig{
			Default:   getEnvDuration("SAGE_TIMEOUT_DEFAULT", 30) * time.Second,
			Embedding: getEnvDuration("SAGE_TIMEOUT_EMBEDDING", 5) * time.Second,
			Parser:    getEnvDuration("SAGE_TIMEOUT_PARSER", 60) * time.Second,
		},
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("[Config] Validation failed: %v", err)
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return fallback
}

func getEnvDuration(key string, fallback int) time.Duration {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return time.Duration(fallback)
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("[Config] Warning: Invalid duration for %s: %v. Using fallback %d", key, err, fallback)
		return time.Duration(fallback)
	}
	return time.Duration(value)
}

func getEnvInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return fallback
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("[Config] Warning: Invalid int for %s: %v. Using fallback %d", key, err, fallback)
		return fallback
	}
	return value
}
