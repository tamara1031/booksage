package embedding

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/booksage/booksage-api/internal/domain/repository"
	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
)

// Batcher handles safely chunking embedding requests to respect memory limits.
type Batcher struct {
	client    repository.EmbeddingClient
	batchSize int
}

// NewBatcher creates a new embedding batcher.
func NewBatcher(client repository.EmbeddingClient, batchSize int) *Batcher {
	return &Batcher{
		client:    client,
		batchSize: batchSize,
	}
}

// GenerateEmbeddingsBatched splits large text arrays into smaller batches and executes them.
// Provides concurrent execution while bounding memory limits.
func (b *Batcher) GenerateEmbeddingsBatched(ctx context.Context, texts []string, embType, taskType string) ([]*pb.EmbeddingResult, int32, error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}

	totalItems := len(texts)
	numBatches := (totalItems + b.batchSize - 1) / b.batchSize

	log.Printf("[Embedding Batcher] Splitting %d texts into %d batches (max %d/batch)", totalItems, numBatches, b.batchSize)

	results := make([]*pb.EmbeddingResult, totalItems)
	var totalTokens int32
	var mu sync.Mutex

	// We use an errgroup or WaitGroup to dispatch the batches concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, numBatches)

	for i := 0; i < numBatches; i++ {
		start := i * b.batchSize
		end := start + b.batchSize
		if end > totalItems {
			end = totalItems
		}

		batchTexts := texts[start:end]
		batchIndex := i // captured for goroutine

		wg.Add(1)
		go func(pts []string, startIdx int, bIdx int) {
			defer wg.Done()

			// Call local/cloud endpoint directly instead of gRPC
			embeddings, err := b.client.Embed(ctx, pts)
			if err != nil {
				log.Printf("[Embedding Batcher] Batch %d failed: %v", bIdx, err)
				errCh <- fmt.Errorf("batch %d failed: %w", bIdx, err)
				return
			}

			mu.Lock()
			// Reassemble results based on original indexing
			for j, vec := range embeddings {
				results[startIdx+j] = &pb.EmbeddingResult{
					Text: pts[j],
					Vector: &pb.EmbeddingResult_Dense{
						Dense: &pb.DenseVector{Values: vec},
					},
				}
			}
			// Approximate token count (simplified)
			totalTokens += int32(len(pts) * 10) // Mock token count
			mu.Unlock()

			log.Printf("[Embedding Batcher] Batch %d completed successfully.", bIdx)
		}(batchTexts, start, batchIndex)
	}

	wg.Wait()
	close(errCh)

	// Determine if any errors occurred during processing
	for err := range errCh {
		if err != nil {
			return nil, 0, err
		}
	}

	return results, totalTokens, nil
}
