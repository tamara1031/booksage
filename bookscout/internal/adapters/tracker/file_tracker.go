package tracker

import (
	"bookscout/internal/core/domain/ports"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileStateStore implements ports.StateStore using a local JSON file.
// Ensure FileStateStore implements StateStore
var _ ports.StateStore = (*FileStateStore)(nil)

type FileStateStore struct {
	filepath string
	mu       sync.RWMutex
	state    stateData
}

type stateData struct {
	Watermark    int64           `json:"watermark"`
	ProcessedIDs map[string]bool `json:"processed_ids"`
}

// NewFileStateStore initializes a state store from a file path.
func NewFileStateStore(path string) (*FileStateStore, error) {
	store := &FileStateStore{
		filepath: path,
		state: stateData{
			Watermark:    0,
			ProcessedIDs: make(map[string]bool),
		},
	}

	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load state file: %w", err)
	}

	return store, nil
}

func (s *FileStateStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.filepath), 0755); err != nil {
		return err
	}

	f, err := os.Open(s.filepath)
	if os.IsNotExist(err) {
		// File doesn't exist, start fresh
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&s.state); err != nil {
		if err == io.EOF {
			return nil // Empty file is fine
		}
		return err
	}

	if s.state.ProcessedIDs == nil {
		s.state.ProcessedIDs = make(map[string]bool)
	}

	return nil
}

// GetWatermark returns the timestamp of the last successfully processed batch.
func (s *FileStateStore) GetWatermark() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Watermark
}

// IsProcessed checks if a specific book ID has already been processed.
func (s *FileStateStore) IsProcessed(bookID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.ProcessedIDs[bookID]
}

// MarkProcessed records a book ID as processed in memory.
func (s *FileStateStore) MarkProcessed(bookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.ProcessedIDs[bookID] = true
	return nil
}

// UpdateWatermark updates the global high-water mark in memory.
func (s *FileStateStore) UpdateWatermark(timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Monotonicity check: ensure watermark only moves forward
	if timestamp > s.state.Watermark {
		s.state.Watermark = timestamp
	}
	return nil
}

// Save persists the current state to storage.
func (s *FileStateStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Atomic write: write to temp file then rename
	tmpFile := s.filepath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(s.state); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if err := os.Rename(tmpFile, s.filepath); err != nil {
		return err
	}

	return nil
}
