package app_test

import (
	"bookscout/internal/scout/app"
	"bookscout/internal/scout/config"
	"bookscout/internal/scout/domain"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockScraper struct{ mock.Mock }

func (m *mockScraper) Scrape(ctx context.Context, since time.Time) ([]domain.Book, error) {
	args := m.Called(ctx, since)
	return args.Get(0).([]domain.Book), args.Error(1)
}
func (m *mockScraper) DownloadContent(ctx context.Context, book domain.Book) (io.ReadCloser, error) {
	args := m.Called(ctx, book)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

type mockIngestor struct{ mock.Mock }

func (m *mockIngestor) Ingest(ctx context.Context, book domain.Book, content io.Reader) (string, error) {
	args := m.Called(ctx, book, content)
	return args.String(0), args.Error(1)
}
func (m *mockIngestor) GetStatusByHash(ctx context.Context, hash string) (string, string, error) {
	args := m.Called(ctx, hash)
	return args.String(0), args.String(1), args.Error(2)
}

type mockRepo struct{ mock.Mock }

func (m *mockRepo) GetWatermark(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return int64(args.Int(0)), args.Error(1)
}
func (m *mockRepo) UpdateWatermark(ctx context.Context, ts int64) error {
	return m.Called(ctx, ts).Error(0)
}
func (m *mockRepo) IsProcessed(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}
func (m *mockRepo) RecordIngestion(ctx context.Context, id, hash string) error {
	return m.Called(ctx, id, hash).Error(0)
}
func (m *mockRepo) GetProcessingDocuments(ctx context.Context) ([]domain.TrackedDocument, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.TrackedDocument), args.Error(1)
}
func (m *mockRepo) UpdateStatusByHash(ctx context.Context, hash string, status domain.DocumentStatus, msg string) error {
	return m.Called(ctx, hash, status, msg).Error(0)
}
func (m *mockRepo) GetStatus(ctx context.Context, id string) (domain.DocumentStatus, bool, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.DocumentStatus), args.Bool(1), args.Error(2)
}

func TestScoutWorker_Run(t *testing.T) {
	cfg := &config.Config{WorkerConcurrency: 1}
	scraper := new(mockScraper)
	ingestor := new(mockIngestor)
	repo := new(mockRepo)

	worker := app.NewScoutWorker(cfg, scraper, ingestor, repo)

	// Setup expectations
	repo.On("GetProcessingDocuments", mock.Anything).Return([]domain.TrackedDocument{}, nil)
	repo.On("GetWatermark", mock.Anything).Return(0, nil)
	scraper.On("Scrape", mock.Anything, mock.Anything).Return([]domain.Book{
		{ID: "1", Title: "Test", AddedAt: time.Now()},
	}, nil)
	repo.On("IsProcessed", mock.Anything, "1").Return(false, nil)
	scraper.On("DownloadContent", mock.Anything, mock.Anything).Return(io.NopCloser(strings.NewReader("data")), nil)
	ingestor.On("Ingest", mock.Anything, mock.Anything, mock.Anything).Return("hash1", nil)
	repo.On("RecordIngestion", mock.Anything, "1", "hash1").Return(nil)
	repo.On("UpdateWatermark", mock.Anything, mock.Anything).Return(nil)

	err := worker.Run(context.Background())
	assert.NoError(t, err)

	scraper.AssertExpectations(t)
	ingestor.AssertExpectations(t)
	repo.AssertExpectations(t)
}
