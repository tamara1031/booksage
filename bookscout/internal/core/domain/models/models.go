package models

import "time"

type BookMetadata struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Author       string    `json:"author"`
	Description  string    `json:"description,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	DownloadURL  string    `json:"download_url"`
	Source       string    `json:"source"`
	AddedAt      time.Time `json:"added_at"`
}
