package tracker

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

type DocumentStatus string

const (
	StatusProcessing DocumentStatus = "PROCESSING"
	StatusCompleted  DocumentStatus = "COMPLETED"
	StatusFailed     DocumentStatus = "FAILED"
)

type TrackedDocument struct {
	bun.BaseModel `bun:"table:documents,alias:d"`

	ID           string         `bun:",pk"`
	FileHash     string         `bun:",notnull"`
	Status       DocumentStatus `bun:",notnull"`
	ErrorMessage string         `bun:",nullzero"`
	UpdatedAt    time.Time      `bun:",nullzero,notnull"`
}

type Watermark struct {
	bun.BaseModel `bun:"table:watermarks,alias:wm"`

	ID        uint  `bun:",pk,default:1"`
	Timestamp int64 `bun:",default:0"`
}

type SQLiteStateStore struct {
	db *bun.DB
}

// NewSQLiteStateStore initializes a persistent SQLite store using Bun.
func NewSQLiteStateStore(path string) (*SQLiteStateStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for state store: %w", err)
	}

	sqldb, err := sql.Open(sqliteshim.ShimName, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	store := &SQLiteStateStore{db: db}
	if err := store.init(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStateStore) init(ctx context.Context) error {
	// Create tables if not exists
	models := []interface{}{
		(*TrackedDocument)(nil),
		(*Watermark)(nil),
	}

	for _, model := range models {
		if _, err := s.db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create Indexes
	if _, err := s.db.NewCreateIndex().
		Model((*TrackedDocument)(nil)).
		Index("idx_documents_status").
		Column("status").
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to create status index: %w", err)
	}

	if _, err := s.db.NewCreateIndex().
		Model((*TrackedDocument)(nil)).
		Index("idx_documents_hash").
		Column("file_hash").
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to create hash index: %w", err)
	}

	// Initialize watermark if not exists
	exists, err := s.db.NewSelect().Model((*Watermark)(nil)).Where("id = 1").Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check watermark existence: %w", err)
	}
	if !exists {
		if _, err := s.db.NewInsert().Model(&Watermark{ID: 1, Timestamp: 0}).Exec(ctx); err != nil {
			return fmt.Errorf("failed to initialize watermark: %w", err)
		}
	}

	return nil
}

// GetWatermark returns the timestamp of the last successfully processed batch.
func (s *SQLiteStateStore) GetWatermark() int64 {
	wm := new(Watermark)
	if err := s.db.NewSelect().Model(wm).Where("id = 1").Scan(context.Background()); err != nil {
		return 0
	}
	return wm.Timestamp
}

// UpdateWatermark updates the global high-water mark.
func (s *SQLiteStateStore) UpdateWatermark(timestamp int64) error {
	_, err := s.db.NewUpdate().
		Model((*Watermark)(nil)).
		Set("timestamp = MAX(timestamp, ?)", timestamp).
		Where("id = 1").
		Exec(context.Background())
	return err
}

// IsProcessed returns true if the document has reached a terminal successful state.
func (s *SQLiteStateStore) IsProcessed(bookID string) bool {
	exists, err := s.db.NewSelect().
		Model((*TrackedDocument)(nil)).
		Where("id = ? AND status = ?", bookID, StatusCompleted).
		Exists(context.Background())
	if err != nil {
		return false
	}
	return exists
}

// GetStatus returns the current local status of a document.
func (s *SQLiteStateStore) GetStatus(bookID string) (DocumentStatus, bool) {
	doc := new(TrackedDocument)
	if err := s.db.NewSelect().Model(doc).Where("id = ?", bookID).Scan(context.Background()); err != nil {
		return "", false
	}
	return doc.Status, true
}

// RecordIngestion starts tracking a document in the PROCESSING state.
func (s *SQLiteStateStore) RecordIngestion(bookID string, fileHash string) error {
	doc := &TrackedDocument{
		ID:           bookID,
		FileHash:     fileHash,
		Status:       StatusProcessing,
		ErrorMessage: "",
		UpdatedAt:    time.Now(),
	}

	_, err := s.db.NewInsert().
		Model(doc).
		On("CONFLICT (id) DO UPDATE").
		Set("file_hash = EXCLUDED.file_hash").
		Set("status = EXCLUDED.status").
		Set("error_message = EXCLUDED.error_message").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(context.Background())
	return err
}

// UpdateStatusByHash updates the state of a tracking document using its file hash.
func (s *SQLiteStateStore) UpdateStatusByHash(fileHash string, status DocumentStatus, errMsg string) error {
	_, err := s.db.NewUpdate().
		Model((*TrackedDocument)(nil)).
		Set("status = ?", status).
		Set("error_message = ?", errMsg).
		Set("updated_at = ?", time.Now()).
		Where("file_hash = ?", fileHash).
		Exec(context.Background())
	return err
}

// GetProcessingDocuments returns all documents currently in the PROCESSING state.
func (s *SQLiteStateStore) GetProcessingDocuments() ([]TrackedDocument, error) {
	var docs []TrackedDocument
	if err := s.db.NewSelect().
		Model(&docs).
		Where("status = ?", StatusProcessing).
		Scan(context.Background()); err != nil {
		return nil, err
	}
	return docs, nil
}

func (s *SQLiteStateStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStateStore) Save() error {
	return nil
}
