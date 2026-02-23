package domain

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// IngestStep represents the individual steps in the ingestion process
type IngestStep int

const (
	StepParsing   IngestStep = 0
	StepChunking  IngestStep = 1
	StepEmbedding IngestStep = 2
	StepIndexing  IngestStep = 3
)

// SagaStatus represents the state of a saga or a step
type SagaStatus int

const (
	SagaStatusPending    SagaStatus = 0
	SagaStatusProcessing SagaStatus = 1
	SagaStatusCompleted  SagaStatus = 2
	SagaStatusFailed     SagaStatus = 3
)

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

// SagaRepository handles Ingest Saga state persistence.
type SagaRepository interface {
	CreateSaga(ctx context.Context, saga *IngestSaga) (int64, error)
	GetSagaByID(ctx context.Context, id int64) (*IngestSaga, error)
	GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*IngestSaga, error)
	UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status SagaStatus, currentStep IngestStep, errorMsg string) error

	UpsertSagaStep(ctx context.Context, step *SagaStep) (int64, error)
	GetSagaSteps(ctx context.Context, sagaID int64) ([]*SagaStep, error)
}
