package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// LocalOllamaClient implements repository.LLMClient by calling a local Ollama server.
type LocalOllamaClient struct {
	host  string
	model string
}

// NewLocalOllamaClient initializes a new client for a local Ollama instance.
func NewLocalOllamaClient(host string, model string) *LocalOllamaClient {
	if host == "" {
		host = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	return &LocalOllamaClient{
		host:  host,
		model: model,
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

type ollamaEmbeddingRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt,omitempty"`
	Input  []string `json:"input,omitempty"`
}

type ollamaEmbeddingResponse struct {
	Embedding  []float32   `json:"embedding,omitempty"`
	Embeddings [][]float32 `json:"embeddings,omitempty"`
}

type ollamaPullRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// Generate sends a prompt to the local Ollama instance.
func (c *LocalOllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	log.Printf("[Ollama] 🏠 Sending request to Local Ollama (%s)...", c.model)

	apiURL := fmt.Sprintf("%s/api/generate", c.host)

	reqBody, err := json.Marshal(ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned error status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %w", err)
	}

	log.Printf("[Ollama] 🏠 Response received from local model.")
	return ollamaResp.Response, nil
}

// Name returns the descriptive name of the client.
func (c *LocalOllamaClient) Name() string {
	return fmt.Sprintf("Ollama (%s) [Local]", c.model)
}

// Embed generates embeddings for the given texts using Ollama's embedding API.
func (c *LocalOllamaClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	log.Printf("[Ollama] 🏠 Generating embeddings for %d texts using %s...", len(texts), c.model)

	apiURL := fmt.Sprintf("%s/api/embed", c.host)

	reqBody, err := json.Marshal(ollamaEmbeddingRequest{
		Model: c.model,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embedding request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned error status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama embedding response: %w", err)
	}

	if len(ollamaResp.Embeddings) > 0 {
		return ollamaResp.Embeddings, nil
	}

	if len(ollamaResp.Embedding) > 0 {
		return [][]float32{ollamaResp.Embedding}, nil
	}

	return nil, fmt.Errorf("no embeddings returned from ollama")
}

// PullModel pulls the specified model from the Ollama library.
func (c *LocalOllamaClient) PullModel(ctx context.Context, model string) error {
	log.Printf("[Ollama] 📥 Pulling model '%s'...", model)

	apiURL := fmt.Sprintf("%s/api/pull", c.host)

	reqBody, err := json.Marshal(ollamaPullRequest{
		Model:  model,
		Stream: false,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal ollama pull request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create ollama pull request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama pull request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama pull returned error status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[Ollama] 📥 Model '%s' pulled successfully.", model)
	return nil
}
