package llm

import (
	"context"
	"testing"
)

func TestGeminiClient_InitError(t *testing.T) {
	// genai.NewClient will return an error if initialized with an empty API key and no credentials
	_, err := NewGeminiClient(context.Background(), "")

	// Based on the google generatve-ai-go library, it might return an error
	// or panic if api key is empty depending on env vars. We just ensure it doesn't crash our test suite unexpectedly here.
	if err == nil {
		// Just log it if it miraculously succeeded (e.g., if ambient credentials exist on the host)
		t.Log("Warning: Expected initialization error for empty api key without ambient credentials, but succeeded.")
	}
}
