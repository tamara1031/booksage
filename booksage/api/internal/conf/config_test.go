package conf

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear the environment block to test defaults
	os.Clearenv()
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "localhost:50051")

	cfg := Load()

	if cfg.Client.WorkerAddr != "localhost:50051" {
		t.Errorf("expected WorkerAddr to be localhost:50051, got %v", cfg.Client.WorkerAddr)
	}
	if cfg.Model.OllamaHost != "http://localhost:11434" {
		t.Errorf("expected OllamaHost to be http://localhost:11434, got %v", cfg.Model.OllamaHost)
	}
	if cfg.Model.OllamaLLM != "llama3" {
		t.Errorf("expected OllamaLLM to be llama3, got %v", cfg.Model.OllamaLLM)
	}
	if cfg.Model.OllamaEmbed != "nomic-embed-text" {
		t.Errorf("expected OllamaEmbed to be nomic-embed-text, got %v", cfg.Model.OllamaEmbed)
	}
	if cfg.Timeout.Default != 30*time.Second {
		t.Errorf("expected Default timeout to be 30s, got %v", cfg.Timeout.Default)
	}
	if cfg.Timeout.Embedding != 5*time.Second {
		t.Errorf("expected Embedding timeout to be 5s, got %v", cfg.Timeout.Embedding)
	}
	if cfg.Timeout.Parser != 60*time.Second {
		t.Errorf("expected Parser timeout to be 60s, got %v", cfg.Timeout.Parser)
	}
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Setup test environment variables
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "worker:50051")
	_ = os.Setenv("SAGE_MODEL_OLLAMA_HOST", "http://ollama:11434")
	_ = os.Setenv("SAGE_MODEL_OLLAMA_LLM", "llama2")
	_ = os.Setenv("SAGE_MODEL_OLLAMA_EMBED", "all-minilm")
	_ = os.Setenv("SAGE_TIMEOUT_DEFAULT", "45")
	_ = os.Setenv("SAGE_TIMEOUT_EMBEDDING", "10")
	_ = os.Setenv("SAGE_TIMEOUT_PARSER", "120")
	defer os.Clearenv()

	cfg := Load()

	if cfg.Client.WorkerAddr != "worker:50051" {
		t.Errorf("expected WorkerAddr to be worker:50051, got %v", cfg.Client.WorkerAddr)
	}
	if cfg.Model.OllamaHost != "http://ollama:11434" {
		t.Errorf("expected OllamaHost to be http://ollama:11434, got %v", cfg.Model.OllamaHost)
	}
	if cfg.Model.OllamaLLM != "llama2" {
		t.Errorf("expected OllamaLLM to be llama2, got %v", cfg.Model.OllamaLLM)
	}
	if cfg.Model.OllamaEmbed != "all-minilm" {
		t.Errorf("expected OllamaEmbed to be all-minilm, got %v", cfg.Model.OllamaEmbed)
	}
	if cfg.Timeout.Default != 45*time.Second {
		t.Errorf("expected Default timeout to be 45s, got %v", cfg.Timeout.Default)
	}
	if cfg.Timeout.Embedding != 10*time.Second {
		t.Errorf("expected Embedding timeout to be 10s, got %v", cfg.Timeout.Embedding)
	}
	if cfg.Timeout.Parser != 120*time.Second {
		t.Errorf("expected Parser timeout to be 120s, got %v", cfg.Timeout.Parser)
	}
}

func TestLoadWithInvalidDuration(t *testing.T) {
	os.Clearenv()
	// Setup an invalid duration
	_ = os.Setenv("SAGE_TIMEOUT_DEFAULT", "invalid")
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "dummy")
	defer os.Clearenv()

	cfg := Load()

	// Should fallback to default 30
	if cfg.Timeout.Default != 30*time.Second {
		t.Errorf("expected Default timeout to fallback to 30s, got %v", cfg.Timeout.Default)
	}
}

func TestLoadQdrantNeo4jDefaults(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "dummy")
	defer os.Clearenv()

	cfg := Load()

	if cfg.DB.QdrantHost != "localhost" {
		t.Errorf("expected QdrantHost localhost, got %v", cfg.DB.QdrantHost)
	}
	if cfg.DB.QdrantPort != 6334 {
		t.Errorf("expected QdrantPort 6334, got %v", cfg.DB.QdrantPort)
	}
	if cfg.DB.QdrantCollection != "booksage" {
		t.Errorf("expected QdrantCollection booksage, got %v", cfg.DB.QdrantCollection)
	}
	if cfg.DB.Neo4jURI != "neo4j://localhost:7687" {
		t.Errorf("expected Neo4jURI neo4j://localhost:7687, got %v", cfg.DB.Neo4jURI)
	}
	if cfg.DB.Neo4jUser != "neo4j" {
		t.Errorf("expected Neo4jUser neo4j, got %v", cfg.DB.Neo4jUser)
	}
	if cfg.DB.Neo4jPassword != "booksage_dev" {
		t.Errorf("expected Neo4jPassword booksage_dev, got %v", cfg.DB.Neo4jPassword)
	}
}

func TestLoadQdrantNeo4jOverrides(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "dummy")
	_ = os.Setenv("SAGE_DB_QDRANT_HOST", "qdrant-host")
	_ = os.Setenv("SAGE_DB_QDRANT_PORT", "6335")
	_ = os.Setenv("SAGE_DB_QDRANT_COLLECTION", "custom-col")
	_ = os.Setenv("SAGE_DB_NEO4J_URI", "neo4j://custom:7688")
	_ = os.Setenv("SAGE_DB_NEO4J_USER", "admin")
	_ = os.Setenv("SAGE_DB_NEO4J_PASSWORD", "secret")
	defer os.Clearenv()

	cfg := Load()

	if cfg.DB.QdrantHost != "qdrant-host" {
		t.Errorf("expected QdrantHost qdrant-host, got %v", cfg.DB.QdrantHost)
	}
	if cfg.DB.QdrantPort != 6335 {
		t.Errorf("expected QdrantPort 6335, got %v", cfg.DB.QdrantPort)
	}
	if cfg.DB.QdrantCollection != "custom-col" {
		t.Errorf("expected QdrantCollection custom-col, got %v", cfg.DB.QdrantCollection)
	}
	if cfg.DB.Neo4jURI != "neo4j://custom:7688" {
		t.Errorf("expected Neo4jURI neo4j://custom:7688, got %v", cfg.DB.Neo4jURI)
	}
	if cfg.DB.Neo4jUser != "admin" {
		t.Errorf("expected Neo4jUser admin, got %v", cfg.DB.Neo4jUser)
	}
	if cfg.DB.Neo4jPassword != "secret" {
		t.Errorf("expected Neo4jPassword secret, got %v", cfg.DB.Neo4jPassword)
	}
}

func TestGetEnvIntInvalid(t *testing.T) {
	os.Clearenv()
	_ = os.Setenv("SAGE_CLIENT_WORKER_ADDR", "dummy")
	_ = os.Setenv("SAGE_DB_QDRANT_PORT", "not-a-number")
	defer os.Clearenv()

	cfg := Load()

	// Should fallback to default 6334
	if cfg.DB.QdrantPort != 6334 {
		t.Errorf("expected QdrantPort to fallback to 6334, got %v", cfg.DB.QdrantPort)
	}
}

func TestValidate_MissingWorkerAddr(t *testing.T) {
	cfg := &Config{
		Client: ClientConfig{
			WorkerAddr: "",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for empty WorkerAddr")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := &Config{
		Client: ClientConfig{
			WorkerAddr: "localhost:50051",
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid config, got %v", err)
	}
}
