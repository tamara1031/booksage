package domain

import (
	"context"
	"time"
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
	ID           int64
	DocumentID   int64
	Document     *Document
	Status       SagaStatus
	Version      int
	CurrentStep  IngestStep
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SagaStep represents a detailed log of a single step
type SagaStep struct {
	ID        int64
	SagaID    int64
	Saga      *IngestSaga
	Name      IngestStep
	Status    SagaStatus
	Metadata  string // JSON blob
	ErrorLog  string
	CreatedAt time.Time
	UpdatedAt time.Time
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
