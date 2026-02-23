package db

import (
	"time"

	"github.com/booksage/booksage-api/internal/domain"
	"github.com/uptrace/bun"
)

// documentModel mirrors domain.Document but adds Bun tags.
type documentModel struct {
	bun.BaseModel `bun:"table:documents,alias:d"`

	ID        int64     `bun:",pk,autoincrement"`
	FileHash  []byte    `bun:",unique,notnull"`
	Title     string    `bun:",notnull"`
	Author    string    `bun:",nullzero"`
	FilePath  string    `bun:",notnull"`
	FileSize  int64     `bun:",notnull"`
	MimeType  string    `bun:",notnull"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}

func toDocumentModel(d *domain.Document) *documentModel {
	if d == nil {
		return nil
	}
	return &documentModel{
		ID:        d.ID,
		FileHash:  d.FileHash,
		Title:     d.Title,
		Author:    d.Author,
		FilePath:  d.FilePath,
		FileSize:  d.FileSize,
		MimeType:  d.MimeType,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

func (m *documentModel) toDomain() *domain.Document {
	if m == nil {
		return nil
	}
	return &domain.Document{
		ID:        m.ID,
		FileHash:  m.FileHash,
		Title:     m.Title,
		Author:    m.Author,
		FilePath:  m.FilePath,
		FileSize:  m.FileSize,
		MimeType:  m.MimeType,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ingestSagaModel mirrors domain.IngestSaga but adds Bun tags.
type ingestSagaModel struct {
	bun.BaseModel `bun:"table:ingest_sagas,alias:is"`

	ID           int64             `bun:",pk,autoincrement"`
	DocumentID   int64             `bun:",notnull"`
	Document     *documentModel    `bun:"rel:belongs-to,join:document_id=id"`
	Status       domain.SagaStatus `bun:",notnull"`
	Version      int               `bun:",notnull,default:1"`
	CurrentStep  domain.IngestStep `bun:",nullzero"`
	ErrorMessage string            `bun:",nullzero"`
	CreatedAt    time.Time         `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time         `bun:",nullzero,notnull,default:current_timestamp"`
}

func toIngestSagaModel(s *domain.IngestSaga) *ingestSagaModel {
	if s == nil {
		return nil
	}
	return &ingestSagaModel{
		ID:           s.ID,
		DocumentID:   s.DocumentID,
		Status:       s.Status,
		Version:      s.Version,
		CurrentStep:  s.CurrentStep,
		ErrorMessage: s.ErrorMessage,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func (m *ingestSagaModel) toDomain() *domain.IngestSaga {
	if m == nil {
		return nil
	}
	return &domain.IngestSaga{
		ID:           m.ID,
		DocumentID:   m.DocumentID,
		Status:       m.Status,
		Version:      m.Version,
		CurrentStep:  m.CurrentStep,
		ErrorMessage: m.ErrorMessage,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		Document:     m.Document.toDomain(),
	}
}

// sagaStepModel mirrors domain.SagaStep but adds Bun tags.
type sagaStepModel struct {
	bun.BaseModel `bun:"table:saga_steps,alias:ss"`

	ID        int64             `bun:",pk,autoincrement"`
	SagaID    int64             `bun:",notnull"`
	Saga      *ingestSagaModel  `bun:"rel:belongs-to,join:saga_id=id"`
	Name      domain.IngestStep `bun:",notnull"`
	Status    domain.SagaStatus `bun:",notnull"`
	Metadata  string            `bun:",nullzero"` // JSON blob
	ErrorLog  string            `bun:",nullzero"`
	CreatedAt time.Time         `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time         `bun:",nullzero,notnull,default:current_timestamp"`
}

func toSagaStepModel(s *domain.SagaStep) *sagaStepModel {
	if s == nil {
		return nil
	}
	return &sagaStepModel{
		ID:        s.ID,
		SagaID:    s.SagaID,
		Name:      s.Name,
		Status:    s.Status,
		Metadata:  s.Metadata,
		ErrorLog:  s.ErrorLog,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func (m *sagaStepModel) toDomain() *domain.SagaStep {
	if m == nil {
		return nil
	}
	return &domain.SagaStep{
		ID:        m.ID,
		SagaID:    m.SagaID,
		Name:      m.Name,
		Status:    m.Status,
		Metadata:  m.Metadata,
		ErrorLog:  m.ErrorLog,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		Saga:      m.Saga.toDomain(),
	}
}
