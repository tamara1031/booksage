package sqlite

import (
	"bookscout/internal/scout/domain"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

type trackedDocument struct {
	bun.BaseModel `bun:"table:documents,alias:d"`

	ID           string                `bun:",pk"`
	FileHash     string                `bun:",notnull"`
	Status       domain.DocumentStatus `bun:",notnull"`
	ErrorMessage string                `bun:",nullzero"`
	UpdatedAt    time.Time             `bun:",nullzero,notnull"`
}

type watermark struct {
	bun.BaseModel `bun:"table:watermarks,alias:wm"`

	ID        uint  `bun:",pk,default:1"`
	Timestamp int64 `bun:",default:0"`
}

type SQLiteRepository struct {
	db *bun.DB
}

func NewSQLiteRepository(path string) (*SQLiteRepository, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	sqldb, err := sql.Open(sqliteshim.ShimName, path)
	if err != nil {
		return nil, err
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())
	repo := &SQLiteRepository{db: db}

	if err := repo.init(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

func (r *SQLiteRepository) init(ctx context.Context) error {
	models := []interface{}{
		(*trackedDocument)(nil),
		(*watermark)(nil),
	}

	for _, m := range models {
		if _, err := r.db.NewCreateTable().Model(m).IfNotExists().Exec(ctx); err != nil {
			return err
		}
	}

	// Create Indexes
	for _, idx := range []string{"idx_documents_status", "idx_documents_hash"} {
		cols := []string{"status"}
		if idx == "idx_documents_hash" {
			cols = []string{"file_hash"}
		}
		if _, err := r.db.NewCreateIndex().Model((*trackedDocument)(nil)).Index(idx).Column(cols...).IfNotExists().Exec(ctx); err != nil {
			return err
		}
	}

	// Init watermark
	exists, err := r.db.NewSelect().Model((*watermark)(nil)).Where("id = 1").Exists(ctx)
	if err == nil && !exists {
		_, _ = r.db.NewInsert().Model(&watermark{ID: 1, Timestamp: 0}).Exec(ctx)
	}

	return nil
}

func (r *SQLiteRepository) GetWatermark(ctx context.Context) (int64, error) {
	wm := new(watermark)
	if err := r.db.NewSelect().Model(wm).Where("id = 1").Scan(ctx); err != nil {
		return 0, nil
	}
	return wm.Timestamp, nil
}

func (r *SQLiteRepository) UpdateWatermark(ctx context.Context, timestamp int64) error {
	_, err := r.db.NewUpdate().
		Model((*watermark)(nil)).
		Set("timestamp = MAX(timestamp, ?)", timestamp).
		Where("id = 1").
		Exec(ctx)
	return err
}

func (r *SQLiteRepository) IsProcessed(ctx context.Context, bookID string) (bool, error) {
	return r.db.NewSelect().
		Model((*trackedDocument)(nil)).
		Where("id = ? AND status = ?", bookID, domain.StatusCompleted).
		Exists(ctx)
}

func (r *SQLiteRepository) GetStatus(ctx context.Context, bookID string) (domain.DocumentStatus, bool, error) {
	doc := new(trackedDocument)
	if err := r.db.NewSelect().Model(doc).Where("id = ?", bookID).Scan(ctx); err != nil {
		return "", false, nil
	}
	return doc.Status, true, nil
}

func (r *SQLiteRepository) GetProcessingDocuments(ctx context.Context) ([]domain.TrackedDocument, error) {
	var results []trackedDocument
	if err := r.db.NewSelect().Model(&results).Where("status = ?", domain.StatusProcessing).Scan(ctx); err != nil {
		return nil, err
	}

	domainDocs := make([]domain.TrackedDocument, len(results))
	for i, d := range results {
		domainDocs[i] = domain.TrackedDocument{
			ID:           d.ID,
			FileHash:     d.FileHash,
			Status:       d.Status,
			ErrorMessage: d.ErrorMessage,
			UpdatedAt:    d.UpdatedAt,
		}
	}
	return domainDocs, nil
}

func (r *SQLiteRepository) RecordIngestion(ctx context.Context, bookID string, fileHash string) error {
	doc := &trackedDocument{
		ID:        bookID,
		FileHash:  fileHash,
		Status:    domain.StatusProcessing,
		UpdatedAt: time.Now(),
	}
	_, err := r.db.NewInsert().Model(doc).On("CONFLICT (id) DO UPDATE").
		Set("file_hash = EXCLUDED.file_hash").
		Set("status = EXCLUDED.status").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (r *SQLiteRepository) UpdateStatusByHash(ctx context.Context, fileHash string, status domain.DocumentStatus, errMsg string) error {
	_, err := r.db.NewUpdate().Model((*trackedDocument)(nil)).
		Set("status = ?", status).
		Set("error_message = ?", errMsg).
		Set("updated_at = ?", time.Now()).
		Where("file_hash = ?", fileHash).
		Exec(ctx)
	return err
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}
