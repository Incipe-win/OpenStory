package project

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Project represents a video creation project.
type Project struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	CoverURL     string    `json:"cover_url"`
	Status       string    `json:"status"`
	SettingsJSON any       `json:"settings"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Work represents a published video work.
type Work struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	DurationMs   *int       `json:"duration_ms,omitempty"`
	Resolution   *string    `json:"resolution,omitempty"`
	Format       *string    `json:"format,omitempty"`
	FileURL      string     `json:"file_url"`
	ThumbnailURL string     `json:"thumbnail_url"`
	Status       string     `json:"status"`
	MetadataJSON any        `json:"metadata"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
