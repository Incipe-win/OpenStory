// Package asset manages media assets stored in MinIO/S3 (upload, download, metadata).
package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

var (
	ErrAssetNotFound        = errors.New("asset not found")
	ErrAssetProjectRequired = errors.New("asset must belong to a project before it can be submitted")
)

const (
	StatusDraft         = "draft"
	StatusPendingReview = "pending_review"
	StatusPublished     = "published"
	StatusRejected      = "rejected"
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
	DurationMs    *int            `json:"duration_ms,omitempty"`
	Width         *int            `json:"width,omitempty"`
	Height        *int            `json:"height,omitempty"`
	Checksum      string          `json:"checksum"`
	StorageKey    string          `json:"storage_key"`
	StorageBucket string          `json:"storage_bucket"`
	URL           string          `json:"url"`
	ThumbnailURL  string          `json:"thumbnail_url"`
	Status        string          `json:"status"`
	MetadataJSON  json.RawMessage `json:"metadata"`
	PublishedAt   *time.Time      `json:"published_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
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
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.MetadataJSON == nil {
		a.MetadataJSON = json.RawMessage(`{}`)
	}
	if a.StorageBucket == "" {
		a.StorageBucket = "openstory"
	}
	if a.Status == "" {
		a.Status = StatusDraft
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin asset create tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := tx.QueryRow(ctx,
		`INSERT INTO assets
			(id, user_id, project_id, type, name, mime_type, size_bytes, duration_ms, width, height,
			 checksum, storage_key, storage_bucket, url, thumbnail_url, status, metadata_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 RETURNING id, created_at, updated_at`,
		a.ID, a.UserID, a.ProjectID, a.Type, a.Name, a.MimeType, a.SizeBytes, a.DurationMs,
		a.Width, a.Height, a.Checksum, a.StorageKey, a.StorageBucket, a.URL, a.ThumbnailURL, a.Status, a.MetadataJSON,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt); err != nil {
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
				"duration_ms":    a.DurationMs,
				"width":          a.Width,
				"height":         a.Height,
				"checksum":       a.Checksum,
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

func (c *Creator) Get(ctx context.Context, id uuid.UUID) (*Asset, error) {
	return scanAsset(c.pool.QueryRow(ctx,
		`SELECT id, user_id, project_id, type, name, mime_type, size_bytes, duration_ms, width, height,
		        checksum, storage_key, storage_bucket, url, thumbnail_url, status, metadata_json, published_at, created_at, updated_at
		 FROM assets WHERE id = $1`, id))
}

func (c *Creator) ListByProject(ctx context.Context, projectID uuid.UUID, page, pageSize int) ([]Asset, int, error) {
	var total int
	if err := c.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM assets WHERE project_id = $1`, projectID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count assets: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := c.pool.Query(ctx,
		`SELECT id, user_id, project_id, type, name, mime_type, size_bytes, duration_ms, width, height,
		        checksum, storage_key, storage_bucket, url, thumbnail_url, status, metadata_json, published_at, created_at, updated_at
		 FROM assets WHERE project_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, err
		}
		assets = append(assets, *a)
	}
	return assets, total, nil
}

func (c *Creator) SubmitForReview(ctx context.Context, id, userID uuid.UUID) (*Asset, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin asset review submission tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var projectID *uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT project_id FROM assets WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		id, userID,
	).Scan(&projectID); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	} else if err != nil {
		return nil, fmt.Errorf("query asset for review submission: %w", err)
	}
	if projectID == nil {
		return nil, ErrAssetProjectRequired
	}

	if _, err := tx.Exec(ctx,
		`UPDATE assets
		    SET status = $3, published_at = NULL
		  WHERE id = $1 AND user_id = $2`,
		id, userID, StatusPendingReview,
	); err != nil {
		return nil, fmt.Errorf("mark asset pending review: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO moderation_records (user_id, target_type, target_id, provider, status, categories_json)
		 VALUES ($1, 'asset', $2, 'internal', 'pending', '{}')
		 ON CONFLICT (target_type, target_id)
		 DO UPDATE SET status = 'pending', reason = NULL, reviewed_by = NULL, reviewed_at = NULL`,
		userID, id,
	); err != nil {
		return nil, fmt.Errorf("upsert asset moderation record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit asset review submission tx: %w", err)
	}
	return c.Get(ctx, id)
}

func scanAsset(row pgx.Row) (*Asset, error) {
	var a Asset
	err := row.Scan(&a.ID, &a.UserID, &a.ProjectID, &a.Type, &a.Name, &a.MimeType,
		&a.SizeBytes, &a.DurationMs, &a.Width, &a.Height, &a.Checksum,
		&a.StorageKey, &a.StorageBucket, &a.URL, &a.ThumbnailURL, &a.Status,
		&a.MetadataJSON, &a.PublishedAt, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan asset: %w", err)
	}
	return &a, nil
}
