package ports

import (
	"bookscout/internal/core/domain/models"
	"context"
	"io"
)

type BookDataSource interface {
	FetchNewBooks(ctx context.Context, lastCheckTimestamp int64) ([]models.BookMetadata, error)
	DownloadBookContent(ctx context.Context, book models.BookMetadata) (io.ReadCloser, error)
}
