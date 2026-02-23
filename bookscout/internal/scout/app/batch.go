package app

import (
	"bookscout/internal/scout/config"
	"bookscout/internal/scout/domain"
	"context"
	"log"
	"sync"
	"time"
)

type BatchProcessor struct {
	cfg  *config.Config
	src  domain.Scraper
	dest domain.Ingestor
	repo domain.StateRepository
}

func NewBatchProcessor(cfg *config.Config, src domain.Scraper, dest domain.Ingestor, repo domain.StateRepository) *BatchProcessor {
	return &BatchProcessor{cfg: cfg, src: src, dest: dest, repo: repo}
}

func (p *BatchProcessor) Process(ctx context.Context) error {
	log.Println("--- Phase 2: Scrape & Ingest ---")

	watermark, _ := p.repo.GetWatermark(ctx)
	since := p.cfg.WorkerSinceTimestamp
	if since == 0 {
		since = watermark
	}

	books, err := p.src.Scrape(ctx, time.Unix(since, 0))
	if err != nil {
		return err
	}

	actionable := p.filter(ctx, books)
	if len(actionable) == 0 {
		return nil
	}

	if p.cfg.WorkerBatchSize > 0 && len(actionable) > p.cfg.WorkerBatchSize {
		actionable = actionable[:p.cfg.WorkerBatchSize]
	}

	maxTS, success, fail := p.processBatch(ctx, actionable, since)

	// Finalize
	log.Printf("Batch finished. Success: %d, Fail: %d", success, fail)
	if fail == 0 && maxTS > since {
		_ = p.repo.UpdateWatermark(ctx, maxTS)
	}
	return nil
}

func (p *BatchProcessor) filter(ctx context.Context, books []domain.Book) []domain.Book {
	var results []domain.Book
	for _, b := range books {
		if processed, _ := p.repo.IsProcessed(ctx, b.ID); !processed {
			results = append(results, b)
		}
	}
	return results
}

func (p *BatchProcessor) processBatch(ctx context.Context, books []domain.Book, since int64) (int64, int, int) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		maxTS   = since
		success = 0
		fail    = 0
	)
	sem := make(chan struct{}, p.cfg.WorkerConcurrency)

	for _, b := range books {
		wg.Add(1)
		sem <- struct{}{}
		go func(book domain.Book) {
			defer wg.Done()
			defer func() { <-sem }()

			hash, err := p.ingestOne(ctx, book)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				fail++
				return
			}
			success++
			_ = p.repo.RecordIngestion(ctx, book.ID, hash)
			if book.AddedAt.Unix() > maxTS {
				maxTS = book.AddedAt.Unix()
			}
		}(b)
	}
	wg.Wait()
	return maxTS, success, fail
}

func (p *BatchProcessor) ingestOne(ctx context.Context, book domain.Book) (string, error) {
	content, err := p.src.DownloadContent(ctx, book)
	if err != nil {
		return "", err
	}
	defer content.Close()
	return p.dest.Ingest(ctx, book, content)
}
