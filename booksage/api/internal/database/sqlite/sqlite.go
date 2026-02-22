package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/booksage/booksage-api/internal/database"
	"github.com/booksage/booksage-api/internal/database/models"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}

	// Enable Foreign Keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_hash BLOB UNIQUE NOT NULL,
		title TEXT NOT NULL,
		author TEXT,
		file_path TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		mime_type TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_documents_file_hash ON documents(file_hash);

	CREATE TABLE IF NOT EXISTS ingest_sagas (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id INTEGER NOT NULL,
		status INTEGER NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		current_step INTEGER,
		error_message TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_ingest_sagas_document_id ON ingest_sagas(document_id);

	CREATE TABLE IF NOT EXISTS saga_steps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		saga_id INTEGER NOT NULL,
		name INTEGER NOT NULL,
		status INTEGER NOT NULL,
		metadata TEXT,
		error_log TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (saga_id) REFERENCES ingest_sagas(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_saga_steps_saga_id ON saga_steps(saga_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Implement DocumentRepository
func (s *SQLiteStore) CreateDocument(ctx context.Context, doc *models.Document) (int64, error) {
	now := time.Now().Unix()
	query := `INSERT INTO documents (file_hash, title, author, file_path, file_size, mime_type, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, doc.FileHash, doc.Title, doc.Author, doc.FilePath, doc.FileSize, doc.MimeType, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetDocumentByID(ctx context.Context, id int64) (*models.Document, error) {
	query := `SELECT id, file_hash, title, author, file_path, file_size, mime_type, created_at, updated_at FROM documents WHERE id = ?`
	doc := &models.Document{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, id).Scan(&doc.ID, &doc.FileHash, &doc.Title, &doc.Author, &doc.FilePath, &doc.FileSize, &doc.MimeType, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	doc.CreatedAt = time.Unix(createdAt, 0)
	doc.UpdatedAt = time.Unix(updatedAt, 0)
	return doc, nil
}

func (s *SQLiteStore) GetDocumentByHash(ctx context.Context, hash []byte) (*models.Document, error) {
	query := `SELECT id, file_hash, title, author, file_path, file_size, mime_type, created_at, updated_at FROM documents WHERE file_hash = ?`
	doc := &models.Document{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, hash).Scan(&doc.ID, &doc.FileHash, &doc.Title, &doc.Author, &doc.FilePath, &doc.FileSize, &doc.MimeType, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	doc.CreatedAt = time.Unix(createdAt, 0)
	doc.UpdatedAt = time.Unix(updatedAt, 0)
	return doc, nil
}

func (s *SQLiteStore) DeleteDocument(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", id)
	return err
}

// Implement SagaRepository
func (s *SQLiteStore) CreateSaga(ctx context.Context, saga *models.IngestSaga) (int64, error) {
	now := time.Now().Unix()
	query := `INSERT INTO ingest_sagas (document_id, status, version, current_step, error_message, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, saga.DocumentID, saga.Status, 1, saga.CurrentStep, saga.ErrorMessage, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetSagaByID(ctx context.Context, id int64) (*models.IngestSaga, error) {
	query := `SELECT id, document_id, status, version, current_step, error_message, created_at, updated_at FROM ingest_sagas WHERE id = ?`
	saga := &models.IngestSaga{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, id).Scan(&saga.ID, &saga.DocumentID, &saga.Status, &saga.Version, &saga.CurrentStep, &saga.ErrorMessage, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	saga.CreatedAt = time.Unix(createdAt, 0)
	saga.UpdatedAt = time.Unix(updatedAt, 0)
	return saga, nil
}

func (s *SQLiteStore) GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*models.IngestSaga, error) {
	query := `SELECT id, document_id, status, version, current_step, error_message, created_at, updated_at FROM ingest_sagas WHERE document_id = ? ORDER BY created_at DESC LIMIT 1`
	saga := &models.IngestSaga{}
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, query, docID).Scan(&saga.ID, &saga.DocumentID, &saga.Status, &saga.Version, &saga.CurrentStep, &saga.ErrorMessage, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	saga.CreatedAt = time.Unix(createdAt, 0)
	saga.UpdatedAt = time.Unix(updatedAt, 0)
	return saga, nil
}

func (s *SQLiteStore) UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status models.SagaStatus, currentStep models.IngestStep, errorMsg string) error {
	now := time.Now().Unix()
	query := `UPDATE ingest_sagas SET status = ?, version = version + 1, current_step = ?, error_message = ?, updated_at = ? WHERE id = ? AND version = ?`
	res, err := s.db.ExecContext(ctx, query, status, currentStep, errorMsg, now, sagaID, currentVersion)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return database.ErrConcurrentUpdate
	}
	return nil
}

func (s *SQLiteStore) UpsertSagaStep(ctx context.Context, step *models.SagaStep) (int64, error) {
	now := time.Now().Unix()
	if step.ID == 0 {
		// Insert
		query := `INSERT INTO saga_steps (saga_id, name, status, metadata, error_log, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
		res, err := s.db.ExecContext(ctx, query, step.SagaID, step.Name, step.Status, step.Metadata, step.ErrorLog, now, now)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	} else {
		// Update
		query := `UPDATE saga_steps SET status = ?, metadata = ?, error_log = ?, updated_at = ? WHERE id = ?`
		_, err := s.db.ExecContext(ctx, query, step.Status, step.Metadata, step.ErrorLog, now, step.ID)
		if err != nil {
			return 0, err
		}
		return step.ID, nil
	}
}

func (s *SQLiteStore) GetSagaSteps(ctx context.Context, sagaID int64) ([]*models.SagaStep, error) {
	query := `SELECT id, saga_id, name, status, metadata, error_log, created_at, updated_at FROM saga_steps WHERE saga_id = ? ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, sagaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*models.SagaStep
	for rows.Next() {
		step := &models.SagaStep{}
		var createdAt, updatedAt int64
		if err := rows.Scan(&step.ID, &step.SagaID, &step.Name, &step.Status, &step.Metadata, &step.ErrorLog, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		step.CreatedAt = time.Unix(createdAt, 0)
		step.UpdatedAt = time.Unix(updatedAt, 0)
		steps = append(steps, step)
	}
	return steps, nil
}
