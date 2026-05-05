// Package asset manages media assets stored in MinIO/S3 (upload, download, metadata).
package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

// Asset represents a stored media asset.
type Asset struct {
	ID            uuid.UUID       `json:"id"`
	UserID        uuid.UUID       `json:"user_id"`
	ProjectID     *uuid.UUID      `json:"project_id,omitempty"`
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	MimeType      string          `json:"mime_type"`
	SizeBytes     int64           `json:"size_bytes"`
	StorageKey    string          `json:"storage_key"`
	StorageBucket string          `json:"storage_bucket"`
	URL           string          `json:"url"`
	ThumbnailURL  string          `json:"thumbnail_url"`
	MetadataJSON  json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Creator creates asset metadata and emits asset events through the outbox.
type Creator struct {
	pool   *pgxpool.Pool
	outbox *eventbus.OutboxWriter
}

// NewCreator creates an asset metadata creator.
func NewCreator(pool *pgxpool.Pool, outbox *eventbus.OutboxWriter) *Creator {
	return &Creator{pool: pool, outbox: outbox}
}

// Create inserts an asset metadata row and appends an asset_created event atomically.
func (c *Creator) Create(ctx context.Context, a *Asset) error {
	if a.MetadataJSON == nil {
		a.MetadataJSON = json.RawMessage(`{}`)
	}
	if a.StorageBucket == "" {
		a.StorageBucket = "openstory"
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin asset create tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := tx.QueryRow(ctx,
		`INSERT INTO assets
			(user_id, project_id, type, name, mime_type, size_bytes, storage_key, storage_bucket, url, thumbnail_url, metadata_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at`,
		a.UserID, a.ProjectID, a.Type, a.Name, a.MimeType, a.SizeBytes, a.StorageKey,
		a.StorageBucket, a.URL, a.ThumbnailURL, a.MetadataJSON,
	).Scan(&a.ID, &a.CreatedAt); err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}

	if c.outbox != nil {
		if err := c.outbox.PublishTx(ctx, tx, eventbus.TopicAssetEvents,
			eventbus.NewEvent("asset_created", "asset", a.ID, map[string]any{
				"project_id":     a.ProjectID,
				"type":           a.Type,
				"name":           a.Name,
				"mime_type":      a.MimeType,
				"size_bytes":     a.SizeBytes,
				"storage_bucket": a.StorageBucket,
				"storage_key":    a.StorageKey,
			}).WithUser(a.UserID)); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit asset create tx: %w", err)
	}
	return nil
}
