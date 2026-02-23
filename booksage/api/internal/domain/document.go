package domain

import (
	"context"
	"time"
)

// Document represents the metadata of a book
type Document struct {
	ID        int64
	FileHash  []byte
	Title     string
	Author    string
	FilePath  string
	FileSize  int64
	MimeType  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DocumentRepository handles book metadata persistence.
type DocumentRepository interface {
	CreateDocument(ctx context.Context, doc *Document) (int64, error)
	GetDocumentByID(ctx context.Context, id int64) (*Document, error)
	GetDocumentByHash(ctx context.Context, hash []byte) (*Document, error)
	DeleteDocument(ctx context.Context, id int64) error
}
