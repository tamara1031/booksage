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

// LocalOllamaClient implements LLMClient by calling a local Ollama server.
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
