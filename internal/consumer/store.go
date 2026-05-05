package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

// Store provides DB-backed idempotency and failure tracking.
type Store interface {
	ProcessOnce(ctx context.Context, consumerName string, meta MessageMeta, event eventbus.Event, handle func(context.Context, pgx.Tx) error) (bool, error)
	RecordFailure(ctx context.Context, consumerName string, meta MessageMeta, event *eventbus.Event, cause error) (int, error)
}

type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

func (s *PgStore) ProcessOnce(ctx context.Context, consumerName string, meta MessageMeta, event eventbus.Event, handle func(context.Context, pgx.Tx) error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin consumer tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO consumer_processed_events
			(consumer_name, event_id, topic, partition_id, offset_id, event_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		consumerName, event.EventID, meta.Topic, meta.Partition, meta.Offset, event.EventType,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, nil
		}
		return false, fmt.Errorf("mark event processed: %w", err)
	}

	if err := handle(ctx, tx); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit consumer tx: %w", err)
	}
	return true, nil
}

func (s *PgStore) RecordFailure(ctx context.Context, consumerName string, meta MessageMeta, event *eventbus.Event, cause error) (int, error) {
	eventID := ""
	eventType := ""
	if event != nil {
		eventID = event.EventID
		eventType = event.EventType
	}

	var attempts int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO consumer_failures
			(consumer_name, topic, partition_id, offset_id, event_id, event_type, last_error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (consumer_name, topic, partition_id, offset_id)
		 DO UPDATE SET
			attempts = consumer_failures.attempts + 1,
			event_id = EXCLUDED.event_id,
			event_type = EXCLUDED.event_type,
			last_error = EXCLUDED.last_error,
			last_failed_at = NOW()
		 RETURNING attempts`,
		consumerName, meta.Topic, meta.Partition, meta.Offset, eventID, eventType, cause.Error(),
	).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("record consumer failure: %w", err)
	}
	return attempts, nil
}
