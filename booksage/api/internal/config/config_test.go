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
	if cfg.OllamaModel != "llama3" {
		t.Errorf("expected OllamaModel to be llama3, got %v", cfg.OllamaModel)
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
	_ = os.Setenv("SAGE_OLLAMA_MODEL", "llama2")
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
	if cfg.OllamaModel != "llama2" {
		t.Errorf("expected OllamaModel to be llama2, got %v", cfg.OllamaModel)
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
