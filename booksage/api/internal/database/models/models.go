package models

import (
	"time"

	"github.com/uptrace/bun"
)

// SagaStatus represents the state of a saga or a step
type SagaStatus int

const (
	SagaStatusPending    SagaStatus = 0
	SagaStatusProcessing SagaStatus = 1
	SagaStatusCompleted  SagaStatus = 2
	SagaStatusFailed     SagaStatus = 3
)

// IngestStep represents the individual steps in the ingestion process
type IngestStep int

const (
	StepParsing   IngestStep = 0
	StepChunking  IngestStep = 1
	StepEmbedding IngestStep = 2
	StepIndexing  IngestStep = 3
)

// Document represents the metadata of a book
type Document struct {
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

// IngestSaga represents the state machine for an ingestion process
type IngestSaga struct {
	bun.BaseModel `bun:"table:ingest_sagas,alias:is"`

	ID           int64      `bun:",pk,autoincrement"`
	DocumentID   int64      `bun:",notnull"`
	Document     *Document  `bun:"rel:belongs-to,join:document_id=id"`
	Status       SagaStatus `bun:",notnull"`
	Version      int        `bun:",notnull,default:1"`
	CurrentStep  IngestStep `bun:",nullzero"`
	ErrorMessage string     `bun:",nullzero"`
	CreatedAt    time.Time  `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time  `bun:",nullzero,notnull,default:current_timestamp"`
}

// SagaStep represents a detailed log of a single step
type SagaStep struct {
	bun.BaseModel `bun:"table:saga_steps,alias:ss"`

	ID        int64       `bun:",pk,autoincrement"`
	SagaID    int64       `bun:",notnull"`
	Saga      *IngestSaga `bun:"rel:belongs-to,join:saga_id=id"`
	Name      IngestStep  `bun:",notnull"`
	Status    SagaStatus  `bun:",notnull"`
	Metadata  string      `bun:",nullzero"` // JSON blob
	ErrorLog  string      `bun:",nullzero"`
	CreatedAt time.Time   `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time   `bun:",nullzero,notnull,default:current_timestamp"`
}
