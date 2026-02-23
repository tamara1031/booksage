package app

import (
	"bookscout/internal/scout/domain"
	"context"
	"log"
)

type StatusSyncer struct {
	ingestor domain.Ingestor
	repo     domain.StateRepository
}

func NewStatusSyncer(ingestor domain.Ingestor, repo domain.StateRepository) *StatusSyncer {
	return &StatusSyncer{ingestor: ingestor, repo: repo}
}

func (s *StatusSyncer) Sync(ctx context.Context) error {
	log.Println("--- Phase 1: Status Synchronization ---")
	docs, err := s.repo.GetProcessingDocuments(ctx)
	if err != nil {
		return err
	}

	for _, d := range docs {
		status, errMsg, err := s.ingestor.GetStatusByHash(ctx, d.FileHash)
		if err != nil {
			continue
		}

		switch status {
		case "completed":
			_ = s.repo.UpdateStatusByHash(ctx, d.FileHash, domain.StatusCompleted, "")
		case "failed":
			_ = s.repo.UpdateStatusByHash(ctx, d.FileHash, domain.StatusFailed, errMsg)
		}
	}
	return nil
}
