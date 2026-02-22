package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear the environment block to test defaults
	os.Clearenv()
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")

	cfg := Load()

	if cfg.WorkerAddr != "localhost:50051" {
		t.Errorf("expected WorkerAddr to be localhost:50051, got %v", cfg.WorkerAddr)
	}
	if cfg.GeminiAPIKey != "dummy" {
		t.Errorf("expected GeminiAPIKey to be dummy, got %v", cfg.GeminiAPIKey)
	}
	if cfg.OllamaHost != "http://localhost:11434" {
		t.Errorf("expected OllamaHost to be http://localhost:11434, got %v", cfg.OllamaHost)
	}
	if cfg.OllamaLLMModel != "llama3" {
		t.Errorf("expected OllamaLLMModel to be llama3, got %v", cfg.OllamaLLMModel)
	}
	if cfg.OllamaEmbedModel != "nomic-embed-text" {
		t.Errorf("expected OllamaEmbedModel to be nomic-embed-text, got %v", cfg.OllamaEmbedModel)
	}
	if cfg.UseLocalOnlyLLM != false {
		t.Errorf("expected UseLocalOnlyLLM to be false, got %v", cfg.UseLocalOnlyLLM)
	}
	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout to be 30s, got %v", cfg.DefaultTimeout)
	}
	if cfg.EmbeddingTimeout != 5*time.Second {
		t.Errorf("expected EmbeddingTimeout to be 5s, got %v", cfg.EmbeddingTimeout)
	}
	if cfg.ParserTimeout != 60*time.Second {
		t.Errorf("expected ParserTimeout to be 60s, got %v", cfg.ParserTimeout)
	}
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Setup test environment variables
	_ = os.Setenv("SAGE_WORKER_ADDR", "worker:50051")
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "test-key")
	_ = os.Setenv("SAGE_OLLAMA_HOST", "http://ollama:11434")
	_ = os.Setenv("SAGE_OLLAMA_LLM_MODEL", "llama2")
	_ = os.Setenv("SAGE_OLLAMA_EMBED_MODEL", "all-minilm")
	_ = os.Setenv("SAGE_USE_LOCAL_ONLY_LLM", "true")
	_ = os.Setenv("SAGE_DEFAULT_TIMEOUT_SEC", "45")
	_ = os.Setenv("SAGE_EMBEDDING_TIMEOUT_SEC", "10")
	_ = os.Setenv("SAGE_PARSER_TIMEOUT_SEC", "120")
	defer os.Clearenv()

	cfg := Load()

	if cfg.WorkerAddr != "worker:50051" {
		t.Errorf("expected WorkerAddr to be worker:50051, got %v", cfg.WorkerAddr)
	}
	if cfg.GeminiAPIKey != "test-key" {
		t.Errorf("expected GeminiAPIKey to be test-key, got %v", cfg.GeminiAPIKey)
	}
	if cfg.OllamaHost != "http://ollama:11434" {
		t.Errorf("expected OllamaHost to be http://ollama:11434, got %v", cfg.OllamaHost)
	}
	if cfg.OllamaLLMModel != "llama2" {
		t.Errorf("expected OllamaLLMModel to be llama2, got %v", cfg.OllamaLLMModel)
	}
	if cfg.OllamaEmbedModel != "all-minilm" {
		t.Errorf("expected OllamaEmbedModel to be all-minilm, got %v", cfg.OllamaEmbedModel)
	}
	if cfg.UseLocalOnlyLLM != true {
		t.Errorf("expected UseLocalOnlyLLM to be true, got %v", cfg.UseLocalOnlyLLM)
	}
	if cfg.DefaultTimeout != 45*time.Second {
		t.Errorf("expected DefaultTimeout to be 45s, got %v", cfg.DefaultTimeout)
	}
	if cfg.EmbeddingTimeout != 10*time.Second {
		t.Errorf("expected EmbeddingTimeout to be 10s, got %v", cfg.EmbeddingTimeout)
	}
	if cfg.ParserTimeout != 120*time.Second {
		t.Errorf("expected ParserTimeout to be 120s, got %v", cfg.ParserTimeout)
	}
}

func TestLoadWithInvalidDuration(t *testing.T) {
	os.Clearenv()
	// Setup an invalid duration
	_ = os.Setenv("SAGE_DEFAULT_TIMEOUT_SEC", "invalid")
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")
	defer os.Clearenv()

	cfg := Load()

	// Should fallback to default 30
	if cfg.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout to fallback to 30s, got %v", cfg.DefaultTimeout)
	}
}

func TestGetEnvBoolEdgeCases(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_USE_LOCAL_ONLY_LLM", "1")
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")
	cfg := Load()
	if !cfg.UseLocalOnlyLLM {
		t.Errorf("expected UseLocalOnlyLLM to be true for '1', got %v", cfg.UseLocalOnlyLLM)
	}

	_ = os.Setenv("SAGE_USE_LOCAL_ONLY_LLM", "TRUE")
	cfg = Load()
	if !cfg.UseLocalOnlyLLM {
		t.Errorf("expected UseLocalOnlyLLM to be true for 'TRUE', got %v", cfg.UseLocalOnlyLLM)
	}

	_ = os.Setenv("SAGE_USE_LOCAL_ONLY_LLM", "false")
	cfg = Load()
	if cfg.UseLocalOnlyLLM {
		t.Errorf("expected UseLocalOnlyLLM to be false for 'false', got %v", cfg.UseLocalOnlyLLM)
	}

	defer os.Clearenv()
}

func TestLoadQdrantNeo4jDefaults(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")
	defer os.Clearenv()

	cfg := Load()

	if cfg.QdrantHost != "localhost" {
		t.Errorf("expected QdrantHost localhost, got %v", cfg.QdrantHost)
	}
	if cfg.QdrantPort != 6334 {
		t.Errorf("expected QdrantPort 6334, got %v", cfg.QdrantPort)
	}
	if cfg.QdrantCollection != "booksage" {
		t.Errorf("expected QdrantCollection booksage, got %v", cfg.QdrantCollection)
	}
	if cfg.Neo4jURI != "neo4j://localhost:7687" {
		t.Errorf("expected Neo4jURI neo4j://localhost:7687, got %v", cfg.Neo4jURI)
	}
	if cfg.Neo4jUser != "neo4j" {
		t.Errorf("expected Neo4jUser neo4j, got %v", cfg.Neo4jUser)
	}
	if cfg.Neo4jPassword != "booksage_dev" {
		t.Errorf("expected Neo4jPassword booksage_dev, got %v", cfg.Neo4jPassword)
	}
}

func TestLoadQdrantNeo4jOverrides(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")
	_ = os.Setenv("SAGE_QDRANT_HOST", "qdrant-host")
	_ = os.Setenv("SAGE_QDRANT_PORT", "6335")
	_ = os.Setenv("SAGE_QDRANT_COLLECTION", "custom-col")
	_ = os.Setenv("SAGE_NEO4J_URI", "neo4j://custom:7688")
	_ = os.Setenv("SAGE_NEO4J_USER", "admin")
	_ = os.Setenv("SAGE_NEO4J_PASSWORD", "secret")
	defer os.Clearenv()

	cfg := Load()

	if cfg.QdrantHost != "qdrant-host" {
		t.Errorf("expected QdrantHost qdrant-host, got %v", cfg.QdrantHost)
	}
	if cfg.QdrantPort != 6335 {
		t.Errorf("expected QdrantPort 6335, got %v", cfg.QdrantPort)
	}
	if cfg.QdrantCollection != "custom-col" {
		t.Errorf("expected QdrantCollection custom-col, got %v", cfg.QdrantCollection)
	}
	if cfg.Neo4jURI != "neo4j://custom:7688" {
		t.Errorf("expected Neo4jURI neo4j://custom:7688, got %v", cfg.Neo4jURI)
	}
	if cfg.Neo4jUser != "admin" {
		t.Errorf("expected Neo4jUser admin, got %v", cfg.Neo4jUser)
	}
	if cfg.Neo4jPassword != "secret" {
		t.Errorf("expected Neo4jPassword secret, got %v", cfg.Neo4jPassword)
	}
}

func TestGetEnvIntInvalid(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_GEMINI_API_KEY", "dummy")
	_ = os.Setenv("SAGE_QDRANT_PORT", "not-a-number")
	defer os.Clearenv()

	cfg := Load()

	// Should fallback to default 6334
	if cfg.QdrantPort != 6334 {
		t.Errorf("expected QdrantPort to fallback to 6334, got %v", cfg.QdrantPort)
	}
}

func TestValidate_MissingWorkerAddr(t *testing.T) {
	cfg := &Config{
		WorkerAddr:   "",
		GeminiAPIKey: "key",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for empty WorkerAddr")
	}
}

func TestValidate_MissingGeminiKey(t *testing.T) {
	cfg := &Config{
		WorkerAddr:      "localhost:50051",
		GeminiAPIKey:    "",
		UseLocalOnlyLLM: false,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing Gemini key when not local-only")
	}
}

func TestValidate_Success_LocalOnly(t *testing.T) {
	cfg := &Config{
		WorkerAddr:      "localhost:50051",
		UseLocalOnlyLLM: true,
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error for local-only mode, got %v", err)
	}
}
