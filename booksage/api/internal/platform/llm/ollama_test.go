package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalOllamaClient_Generate_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.Prompt != "test prompt" {
			t.Errorf("Expected test prompt, got %s", req.Prompt)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Response: "mocked response",
		})
	}))
	defer ts.Close()

	client := NewLocalOllamaClient(ts.URL, "test-model")

	resp, err := client.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp != "mocked response" {
		t.Errorf("Expected mocked response, got %s", resp)
	}
	if client.Name() != "Ollama (test-model) [Local]" {
		t.Errorf("Unexpected name: %s", client.Name())
	}
}

func TestLocalOllamaClient_Generate_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	client := NewLocalOllamaClient(ts.URL, "")

	_, err := client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != "ollama returned error status 500: internal error" {
		t.Errorf("Unexpected error messaging: %v", err)
	}
}

func TestLocalOllamaClient_Generate_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer ts.Close()

	client := NewLocalOllamaClient(ts.URL, "")

	_, err := client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
}

func TestLocalOllamaClient_ConnectionError(t *testing.T) {
	client := NewLocalOllamaClient("http://localhost:1", "model")

	_, err := client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Expected connection error, got nil")
	}
}

func TestLocalOllamaClient_Defaults(t *testing.T) {
	client := NewLocalOllamaClient("", "")
	if client.host != "http://localhost:11434" {
		t.Errorf("expected default host, got %s", client.host)
	}
	if client.model != "llama3" {
		t.Errorf("expected default model, got %s", client.model)
	}
}
