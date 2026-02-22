package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all environmentally dependent settings for the BookSage API.
type Config struct {
	WorkerAddr       string
	GeminiAPIKey     string
	OllamaHost       string
	OllamaModel      string
	UseLocalOnlyLLM  bool
	DefaultTimeout   time.Duration
	EmbeddingTimeout time.Duration
	ParserTimeout    time.Duration

	// Qdrant Vector DB
	QdrantHost       string
	QdrantPort       int
	QdrantCollection string

	// Neo4j Graph DB
	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string
}

// Validate ensures that all required configuration is present and valid.
func (c *Config) Validate() error {
	if !c.UseLocalOnlyLLM && c.GeminiAPIKey == "" {
		return fmt.Errorf("SAGE_GEMINI_API_KEY is required when SAGE_USE_LOCAL_ONLY_LLM is false")
	}
	if c.WorkerAddr == "" {
		return fmt.Errorf("SAGE_WORKER_ADDR is required")
	}
	return nil
}

// Load reads settings from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		WorkerAddr:       getEnv("SAGE_WORKER_ADDR", "localhost:50051"),
		GeminiAPIKey:     getEnv("SAGE_GEMINI_API_KEY", ""),
		OllamaHost:       getEnv("SAGE_OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:      getEnv("SAGE_OLLAMA_MODEL", "llama3"),
		UseLocalOnlyLLM:  getEnvBool("SAGE_USE_LOCAL_ONLY_LLM", false),
		DefaultTimeout:   getEnvDuration("SAGE_DEFAULT_TIMEOUT_SEC", 30) * time.Second,
		EmbeddingTimeout: getEnvDuration("SAGE_EMBEDDING_TIMEOUT_SEC", 5) * time.Second,
		ParserTimeout:    getEnvDuration("SAGE_PARSER_TIMEOUT_SEC", 60) * time.Second,

		QdrantHost:       getEnv("SAGE_QDRANT_HOST", "localhost"),
		QdrantPort:       getEnvInt("SAGE_QDRANT_PORT", 6334),
		QdrantCollection: getEnv("SAGE_QDRANT_COLLECTION", "booksage"),

		Neo4jURI:      getEnv("SAGE_NEO4J_URI", "neo4j://localhost:7687"),
		Neo4jUser:     getEnv("SAGE_NEO4J_USER", "neo4j"),
		Neo4jPassword: getEnv("SAGE_NEO4J_PASSWORD", "booksage_dev"),
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
