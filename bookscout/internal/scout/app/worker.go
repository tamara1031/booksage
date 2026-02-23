package app

import (
	"bookscout/internal/scout/config"
	"bookscout/internal/scout/domain"
	"context"
)

// ScoutWorker coordinates the sync and scrape phases.
type ScoutWorker struct {
	cfg    *config.Config
	syncer *StatusSyncer
	batch  *BatchProcessor
}

func NewScoutWorker(
	cfg *config.Config,
	scraper domain.Scraper,
	ingestor domain.Ingestor,
	repo domain.StateRepository,
) *ScoutWorker {
	return &ScoutWorker{
		cfg:    cfg,
		syncer: NewStatusSyncer(ingestor, repo),
		batch:  NewBatchProcessor(cfg, scraper, ingestor, repo),
	}
}

func (w *ScoutWorker) Run(ctx context.Context) error {
	_ = w.syncer.Sync(ctx)
	return w.batch.Process(ctx)
}
