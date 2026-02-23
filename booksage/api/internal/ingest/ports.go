package ingest

import (
	"context"
	"errors"

	"github.com/booksage/booksage-api/internal/ports"
)

var ErrNotFound = errors.New("record not found")
var ErrConcurrentUpdate = errors.New("concurrent update detected: version mismatch")

// DocumentRepository handles book metadata persistence
type DocumentRepository interface {
	CreateDocument(ctx context.Context, doc *Document) (int64, error)
	GetDocumentByID(ctx context.Context, id int64) (*Document, error)
	GetDocumentByHash(ctx context.Context, hash []byte) (*Document, error)
	DeleteDocument(ctx context.Context, id int64) error
}

// SagaRepository handles Ingest Saga state persistence
type SagaRepository interface {
	CreateSaga(ctx context.Context, saga *IngestSaga) (int64, error)
	GetSagaByID(ctx context.Context, id int64) (*IngestSaga, error)
	GetLatestSagaByDocumentID(ctx context.Context, docID int64) (*IngestSaga, error)
	UpdateSagaStatus(ctx context.Context, sagaID int64, currentVersion int, status SagaStatus, currentStep IngestStep, errorMsg string) error

	UpsertSagaStep(ctx context.Context, step *SagaStep) (int64, error)
	GetSagaSteps(ctx context.Context, sagaID int64) ([]*SagaStep, error)
}

// Type Aliases for shared ports
type SearchResult = ports.SearchResult
type VectorRepository = ports.VectorRepository
type GraphRepository = ports.GraphRepository
type LLMClient = ports.LLMClient
type TensorEngine = ports.TensorEngine // Replaced EmbeddingClient
type TaskType = ports.TaskType
type LLMRouter = ports.LLMRouter

const (
	TaskEmbedding               = ports.TaskEmbedding
	TaskSimpleKeywordExtraction = ports.TaskSimpleKeywordExtraction
	TaskAgenticReasoning        = ports.TaskAgenticReasoning
	TaskDeepSummarization       = ports.TaskDeepSummarization
	TaskMultimodalParsing       = ports.TaskMultimodalParsing
)
