package embedding

import (
	"context"
	"fmt"
	"log"
	"sync"

	pb "github.com/booksage/booksage-api/internal/pb/booksage/v1"
)

// Batcher handles safely chunking embedding requests to respect gRPC memory limits (4MB default).
type Batcher struct {
	client    pb.EmbeddingServiceClient
	batchSize int
}

// NewBatcher creates a new embedding batcher.
// Standard recommended batch size is 100 texts to prevent gRPC resource exhausted errors.
func NewBatcher(client pb.EmbeddingServiceClient, batchSize int) *Batcher {
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

			req := &pb.EmbeddingRequest{
				Texts:         pts,
				EmbeddingType: embType,
				TaskType:      taskType,
			}

			// Call gRPC endpoint
			resp, err := b.client.GenerateEmbeddings(ctx, req)
			if err != nil {
				log.Printf("[Embedding Batcher] Batch %d failed: %v", bIdx, err)
				errCh <- fmt.Errorf("batch %d failed: %w", bIdx, err)
				return
			}

			mu.Lock()
			// Reassemble results based on original indexing
			for j, res := range resp.Results {
				results[startIdx+j] = res
			}
			totalTokens += resp.TotalTokens
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
