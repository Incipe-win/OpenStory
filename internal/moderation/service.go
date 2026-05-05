package moderation

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

	record, err := queryRecordTx(ctx, tx, workID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit moderation review tx: %w", err)
	}
	return record, nil
}

func queryRecordTx(ctx context.Context, tx pgx.Tx, workID uuid.UUID) (*Record, error) {
	return scanRecord(tx.QueryRow(ctx,
		`SELECT id, user_id, target_type, target_id, provider, status, categories_json, score, reason, reviewed_by, reviewed_at, created_at
		 FROM moderation_records WHERE target_type = 'work' AND target_id = $1`,
		workID,
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

func insertAudit(ctx context.Context, tx pgx.Tx, adminID, workID uuid.UUID, ipAddress, userAgent, oldStatus, newStatus, reason string) error {
	oldJSON, _ := json.Marshal(map[string]any{"status": oldStatus})
	newJSON, _ := json.Marshal(map[string]any{"status": newStatus, "reason": reason})
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, user_agent, old_values_json, new_values_json, trace_id)
		 VALUES ($1, 'review_work', 'work', $2, $3, $4, $5, $6, $7)`,
		adminID, workID, ipAddress, userAgent, oldJSON, newJSON, observability.TraceIDFromContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("insert moderation audit log: %w", err)
	}
	return nil
}
