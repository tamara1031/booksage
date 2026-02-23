package domain

import (
	"context"
	"time"

	"github.com/uptrace/bun"
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

// DocumentRepository handles book metadata persistence.
type DocumentRepository interface {
	CreateDocument(ctx context.Context, doc *Document) (int64, error)
	GetDocumentByID(ctx context.Context, id int64) (*Document, error)
	GetDocumentByHash(ctx context.Context, hash []byte) (*Document, error)
	DeleteDocument(ctx context.Context, id int64) error
}
