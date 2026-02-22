package bunstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/booksage/booksage-api/internal/database"
	"github.com/booksage/booksage-api/internal/database/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

type BunStore struct {
	db *bun.DB
}

func NewBunStore(db *sql.DB, dialect schema.Dialect) (*BunStore, error) {
	bunDB := bun.NewDB(db, dialect)

	store := &BunStore{db: bunDB}

	// Create tables if they don't exist
	ctx := context.Background()
	if _, err := bunDB.NewCreateTable().Model((*models.Document)(nil)).IfNotExists().Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to create documents table: %w", err)
	}
	if _, err := bunDB.NewCreateTable().Model((*models.IngestSaga)(nil)).IfNotExists().Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to create ingest_sagas table: %w", err)
	}
	if _, err := bunDB.NewCreateTable().Model((*models.SagaStep)(nil)).IfNotExists().Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to create saga_steps table: %w", err)
	}

	return store, nil
}

// DocumentRepository Implementation
func (s *BunStore) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	if _, err := s.db.NewInsert().Model(doc).Exec(ctx); err != nil {
		return 0, err
	}
	return doc.ID, nil
}

func (s *BunStore) GetDocumentByID(ctx context.Context, id int64) (*models.Document, error) {
	doc := new(models.Document)
	if err := s.db.NewSelect().Model(doc).Where("id = ?", id).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	return doc, nil
}

func (s *BunStore) GetDocumentByHash(ctx context.Context, hash []byte) (*models.Document, error) {
	doc := new(models.Document)
	if err := s.db.NewSelect().Model(doc).Where("file_hash = ?", hash).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	return doc, nil
}

func (s *BunStore) DeleteDocument(ctx context.Context, id int64) error {
	if _, err := s.db.NewDelete().Model((*models.Document)(nil)).Where("id = ?", id).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// SagaRepository Implementation
func (s *BunStore) CreateSaga(ctx context.Context, saga *models.IngestSaga) (int64, error) {
	if _, err := s.db.NewInsert().Model(saga).Exec(ctx); err != nil {
		return 0, err
	}
	return saga.ID, nil
}

func (s *BunStore) GetSagaByID(ctx context.Context, id int64) (*models.IngestSaga, error) {
	saga := new(models.IngestSaga)
	if err := s.db.NewSelect().Model(saga).Where("id = ?", id).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	return saga, nil
}

func (s *BunStore) GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*models.IngestSaga, error) {
	saga := new(models.IngestSaga)
	if err := s.db.NewSelect().Model(saga).Where("document_id = ?", docID).Order("created_at DESC").Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, database.ErrNotFound
		}
		return nil, err
	}
	return saga, nil
}

func (s *BunStore) UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status models.SagaStatus, currentStep models.IngestStep, errorMsg string) error {
	res, err := s.db.NewUpdate().Model((*models.IngestSaga)(nil)).
		Set("status = ?", status).
		Set("current_step = ?", currentStep).
		Set("error_message = ?", errorMsg).
		Set("version = version + 1").
		Set("updated_at = current_timestamp").
		Where("id = ? AND version = ?", sagaID, currentVersion).
		Exec(ctx)

	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return database.ErrConcurrentUpdate
	}
	return nil
}

func (s *BunStore) UpsertSagaStep(ctx context.Context, step *models.SagaStep) (int64, error) {
	if step.ID == 0 {
		if _, err := s.db.NewInsert().Model(step).Exec(ctx); err != nil {
			return 0, err
		}
	} else {
		if _, err := s.db.NewUpdate().Model(step).WherePK().Exec(ctx); err != nil {
			return 0, err
		}
	}
	return step.ID, nil
}

func (s *BunStore) GetSagaSteps(ctx context.Context, sagaID int64) ([]*models.SagaStep, error) {
	var steps []*models.SagaStep
	if err := s.db.NewSelect().Model(&steps).Where("saga_id = ?", sagaID).Order("created_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return steps, nil
}
