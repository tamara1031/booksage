package database

import (
	"context"
	"errors"

	"github.com/booksage/booksage-api/internal/database/models"
)

var (
	ErrNotFound         = errors.New("record not found")
	ErrConcurrentUpdate = errors.New("concurrent update detected: version mismatch")
)

// DocumentRepository handles book metadata persistence
type DocumentRepository interface {
	CreateDocument(ctx context.Context, doc *models.Document) (int64, error)
	GetDocumentByID(ctx context.Context, id int64) (*models.Document, error)
	GetDocumentByHash(ctx context.Context, hash []byte) (*models.Document, error)
	DeleteDocument(ctx context.Context, id int64) error
}

// SagaRepository handles Ingest Saga state persistence
type SagaRepository interface {
	CreateSaga(ctx context.Context, saga *models.IngestSaga) (int64, error)
	GetSagaByID(ctx context.Context, id int64) (*models.IngestSaga, error)
	GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*models.IngestSaga, error)
	UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status models.SagaStatus, currentStep models.IngestStep, errorMsg string) error

	UpsertSagaStep(ctx context.Context, step *models.SagaStep) (int64, error)
	GetSagaSteps(ctx context.Context, sagaID int64) ([]*models.SagaStep, error)
}
