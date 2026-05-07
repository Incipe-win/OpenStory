package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

var (
	ErrRecordNotFound = errors.New("moderation record not found")
	ErrInvalidStatus  = errors.New("moderation status must be approved or rejected")
)

type Record struct {
	ID             uuid.UUID       `json:"id"`
	UserID         uuid.UUID       `json:"user_id"`
	TargetType     string          `json:"target_type"`
	TargetID       uuid.UUID       `json:"target_id"`
	Provider       string          `json:"provider"`
	Status         string          `json:"status"`
	CategoriesJSON json.RawMessage `json:"categories"`
	Score          *float64        `json:"score,omitempty"`
	Reason         *string         `json:"reason,omitempty"`
	ReviewedBy     *uuid.UUID      `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Service struct {
	pool   *pgxpool.Pool
	outbox *eventbus.OutboxWriter
}

func NewService(pool *pgxpool.Pool, outbox *eventbus.OutboxWriter) *Service {
	return &Service{pool: pool, outbox: outbox}
}

func (s *Service) List(ctx context.Context, status string, page, pageSize int) ([]Record, int, error) {
	if status == "" {
		status = StatusPending
	}
	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM moderation_records WHERE status = $1`, status,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count moderation records: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, target_type, target_id, provider, status, categories_json, score, reason, reviewed_by, reviewed_at, created_at
		 FROM moderation_records
		 WHERE status = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		status, pageSize, (page-1)*pageSize,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list moderation records: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, *record)
	}
	return records, total, nil
}

func (s *Service) ReviewWork(ctx context.Context, workID, adminID uuid.UUID, status, reason, ipAddress, userAgent string) (*Record, error) {
	if status != StatusApproved && status != StatusRejected {
		return nil, ErrInvalidStatus
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin moderation review tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var workUserID, projectID uuid.UUID
	var title, oldWorkStatus string
	if err := tx.QueryRow(ctx,
		`SELECT user_id, project_id, title, status FROM works WHERE id = $1 FOR UPDATE`,
		workID,
	).Scan(&workUserID, &projectID, &title, &oldWorkStatus); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRecordNotFound
	} else if err != nil {
		return nil, fmt.Errorf("query work for moderation: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO moderation_records (user_id, target_type, target_id, provider, status, categories_json)
		 VALUES ($1, 'work', $2, 'admin', 'pending', '{}')
		 ON CONFLICT (target_type, target_id) DO NOTHING`,
		workUserID, workID,
	); err != nil {
		return nil, fmt.Errorf("ensure moderation record: %w", err)
	}

	var oldStatus string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM moderation_records WHERE target_type = 'work' AND target_id = $1 FOR UPDATE`,
		workID,
	).Scan(&oldStatus); err != nil {
		return nil, fmt.Errorf("lock moderation record: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE moderation_records
		    SET status = $2, reason = $3, reviewed_by = $4, reviewed_at = NOW()
		  WHERE target_type = 'work' AND target_id = $1`,
		workID, status, reason, adminID,
	); err != nil {
		return nil, fmt.Errorf("update moderation record: %w", err)
	}

	if status == StatusApproved {
		if _, err := tx.Exec(ctx,
			`UPDATE works SET status = 'published', published_at = COALESCE(published_at, NOW()) WHERE id = $1`,
			workID,
		); err != nil {
			return nil, fmt.Errorf("publish approved work: %w", err)
		}
		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, eventbus.TopicWorkEvents,
				eventbus.NewEvent("work_published", "work", workID, map[string]any{
					"project_id":          projectID,
					"title":               title,
					"status":              "published",
					"moderation_status":   StatusApproved,
					"previous_status":     oldWorkStatus,
					"previous_moderation": oldStatus,
				}).WithUser(workUserID)); err != nil {
				return nil, err
			}
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE works SET status = 'rejected', published_at = NULL WHERE id = $1`,
			workID,
		); err != nil {
			return nil, fmt.Errorf("reject work: %w", err)
		}
		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, eventbus.TopicModerationEvents,
				eventbus.NewEvent("work_rejected", "work", workID, map[string]any{
					"project_id":        projectID,
					"title":             title,
					"status":            "rejected",
					"moderation_status": StatusRejected,
					"reason":            reason,
				}).WithUser(workUserID)); err != nil {
				return nil, err
			}
		}
	}

	if err := insertAudit(ctx, tx, adminID, workID, ipAddress, userAgent, oldStatus, status, reason); err != nil {
		return nil, err
	}

	record, err := queryRecordTx(ctx, tx, "work", workID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit moderation review tx: %w", err)
	}
	return record, nil
}

func (s *Service) ReviewAsset(ctx context.Context, assetID, adminID uuid.UUID, status, reason, ipAddress, userAgent string) (*Record, error) {
	if status != StatusApproved && status != StatusRejected {
		return nil, ErrInvalidStatus
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin asset moderation review tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var projectID *uuid.UUID
	var assetUserID uuid.UUID
	var name, assetType, mimeType, rawURL, thumbnailURL, oldAssetStatus string
	var durationMs, width, height *int
	var publishedAt *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT user_id, project_id, name, type, mime_type, duration_ms, width, height,
		        url, thumbnail_url, status, published_at
		   FROM assets WHERE id = $1 FOR UPDATE`,
		assetID,
	).Scan(&assetUserID, &projectID, &name, &assetType, &mimeType, &durationMs, &width, &height,
		&rawURL, &thumbnailURL, &oldAssetStatus, &publishedAt); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRecordNotFound
	} else if err != nil {
		return nil, fmt.Errorf("query asset for moderation: %w", err)
	}
	if projectID == nil {
		return nil, ErrRecordNotFound
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO moderation_records (user_id, target_type, target_id, provider, status, categories_json)
		 VALUES ($1, 'asset', $2, 'admin', 'pending', '{}')
		 ON CONFLICT (target_type, target_id) DO NOTHING`,
		assetUserID, assetID,
	); err != nil {
		return nil, fmt.Errorf("ensure asset moderation record: %w", err)
	}

	var oldModerationStatus string
	if err := tx.QueryRow(ctx,
		`SELECT status FROM moderation_records WHERE target_type = 'asset' AND target_id = $1 FOR UPDATE`,
		assetID,
	).Scan(&oldModerationStatus); err != nil {
		return nil, fmt.Errorf("lock asset moderation record: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE moderation_records
		    SET status = $2, reason = $3, reviewed_by = $4, reviewed_at = NOW()
		  WHERE target_type = 'asset' AND target_id = $1`,
		assetID, status, reason, adminID,
	); err != nil {
		return nil, fmt.Errorf("update asset moderation record: %w", err)
	}

	if status == StatusApproved {
		if _, err := tx.Exec(ctx,
			`UPDATE assets
			    SET status = $2, published_at = COALESCE(published_at, NOW())
			  WHERE id = $1`,
			assetID, asset.StatusPublished,
		); err != nil {
			return nil, fmt.Errorf("publish approved asset: %w", err)
		}

		meta, _ := json.Marshal(map[string]any{
			"source":            "asset",
			"asset_id":          assetID,
			"asset_type":        assetType,
			"mime_type":         mimeType,
			"moderation_status": oldModerationStatus,
			"previous_status":   oldAssetStatus,
			"published_source":  "asset_review",
		})
		resolution := resolutionString(width, height)
		if strings.EqualFold(assetType, "image") && thumbnailURL == "" {
			thumbnailURL = rawURL
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO works
			    (id, project_id, user_id, title, description, duration_ms, resolution, format,
			     file_url, thumbnail_url, status, metadata_json, published_at)
			 VALUES
			    ($1, $2, $3, $4, '', $5, $6, $7, $8, $9, 'published', $10, COALESCE($11::timestamptz, NOW()))
			 ON CONFLICT (id)
			 DO UPDATE SET
			    title = EXCLUDED.title,
			    duration_ms = EXCLUDED.duration_ms,
			    resolution = EXCLUDED.resolution,
			    format = EXCLUDED.format,
			    file_url = EXCLUDED.file_url,
			    thumbnail_url = EXCLUDED.thumbnail_url,
			    status = 'published',
			    metadata_json = EXCLUDED.metadata_json,
			    published_at = COALESCE(works.published_at, EXCLUDED.published_at)`,
			assetID, *projectID, assetUserID, name, durationMs, resolution, mimeType,
			rawURL, thumbnailURL, string(meta), publishedAt,
		); err != nil {
			return nil, fmt.Errorf("publish asset as work: %w", err)
		}

		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, eventbus.TopicWorkEvents,
				eventbus.NewEvent("work_published", "work", assetID, map[string]any{
					"project_id":          *projectID,
					"title":               name,
					"status":              "published",
					"moderation_status":   StatusApproved,
					"source_asset_id":     assetID,
					"previous_status":     oldAssetStatus,
					"previous_moderation": oldModerationStatus,
				}).WithUser(assetUserID)); err != nil {
				return nil, err
			}
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE assets SET status = $2, published_at = NULL WHERE id = $1`,
			assetID, asset.StatusRejected,
		); err != nil {
			return nil, fmt.Errorf("reject asset: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE works SET status = 'rejected', published_at = NULL WHERE id = $1`,
			assetID,
		); err != nil {
			return nil, fmt.Errorf("reject asset work projection: %w", err)
		}
		if s.outbox != nil {
			if err := s.outbox.PublishTx(ctx, tx, eventbus.TopicModerationEvents,
				eventbus.NewEvent("asset_rejected", "asset", assetID, map[string]any{
					"project_id":        *projectID,
					"name":              name,
					"status":            asset.StatusRejected,
					"moderation_status": StatusRejected,
					"reason":            reason,
				}).WithUser(assetUserID)); err != nil {
				return nil, err
			}
		}
	}

	if err := insertAudit(ctx, tx, adminID, assetID, ipAddress, userAgent, oldModerationStatus, status, reason, "asset"); err != nil {
		return nil, err
	}

	record, err := queryRecordTx(ctx, tx, "asset", assetID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit asset moderation review tx: %w", err)
	}
	return record, nil
}

func queryRecordTx(ctx context.Context, tx pgx.Tx, targetType string, targetID uuid.UUID) (*Record, error) {
	return scanRecord(tx.QueryRow(ctx,
		`SELECT id, user_id, target_type, target_id, provider, status, categories_json, score, reason, reviewed_by, reviewed_at, created_at
		 FROM moderation_records WHERE target_type = $1 AND target_id = $2`,
		targetType, targetID,
	))
}

func scanRecord(row pgx.Row) (*Record, error) {
	record := &Record{}
	err := row.Scan(&record.ID, &record.UserID, &record.TargetType, &record.TargetID, &record.Provider,
		&record.Status, &record.CategoriesJSON, &record.Score, &record.Reason, &record.ReviewedBy,
		&record.ReviewedAt, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan moderation record: %w", err)
	}
	return record, nil
}

func insertAudit(ctx context.Context, tx pgx.Tx, adminID, targetID uuid.UUID, ipAddress, userAgent, oldStatus, newStatus, reason string, targetType ...string) error {
	resourceType := "work"
	action := "review_work"
	if len(targetType) > 0 && targetType[0] != "" {
		resourceType = targetType[0]
		action = "review_" + targetType[0]
	}
	oldJSON, _ := json.Marshal(map[string]any{"status": oldStatus})
	newJSON, _ := json.Marshal(map[string]any{"status": newStatus, "reason": reason})
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, user_agent, old_values_json, new_values_json, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		adminID, action, resourceType, targetID, ipAddress, userAgent, oldJSON, newJSON, observability.TraceIDFromContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("insert moderation audit log: %w", err)
	}
	return nil
}

func resolutionString(width, height *int) *string {
	if width == nil || height == nil || *width <= 0 || *height <= 0 {
		return nil
	}
	resolution := fmt.Sprintf("%dx%d", *width, *height)
	return &resolution
}
