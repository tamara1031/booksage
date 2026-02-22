package llm

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements repository.LLMClient.
type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key must not be empty")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	// Default to gemini-1.5-pro for high-cognitive tasks (2M context)
	model := client.GenerativeModel("gemini-1.5-pro")

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

func (c *GeminiClient) Generate(ctx context.Context, prompt string) (string, error) {
	log.Printf("[Gemini] ☁️ Sending request to Gemini 1.5 Pro...")

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini generation failed: %w", err)
	}

	text, err := extractText(resp)
	if err != nil {
		return "", err
	}

	log.Printf("[Gemini] ☁️ Response received successfully.")
	return text, nil
}

func extractText(resp *genai.GenerateContentResponse) (string, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from gemini")
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			return string(text), nil
		}
	}

	return "", fmt.Errorf("unexpected response format from gemini")
}

func (c *GeminiClient) Name() string {
	return "Gemini 1.5 Pro (Cloud)"
}

func (c *GeminiClient) Close() error {
	return c.client.Close()
}

// Embed generates embeddings for the given texts using Gemini's embedding API.
func (c *GeminiClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	log.Printf("[Gemini] ☁️ Generating embeddings for %d texts...", len(texts))

	// Gemini Pro is typically used for text, but for embeddings we use a specialized model
	em := c.client.EmbeddingModel("text-embedding-004")

	batch := em.NewBatch()
	for _, t := range texts {
		batch.AddContent(genai.Text(t))
	}

	resp, err := em.BatchEmbedContents(ctx, batch)
	if err != nil {
		return nil, fmt.Errorf("gemini embedding failed: %w", err)
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Values
	}

	log.Printf("[Gemini] ☁️ Embeddings generated successfully.")
	return embeddings, nil
}
