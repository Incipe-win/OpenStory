package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxWriter implements EventBus by writing events to the outbox_events table.
// This is the primary EventBus used by business code — events are later relayed to Kafka.
type OutboxWriter struct {
	pool *pgxpool.Pool
}

var _ EventBus = (*OutboxWriter)(nil) // compile-time check

func NewOutboxWriter(pool *pgxpool.Pool) *OutboxWriter {
	return &OutboxWriter{pool: pool}
}

// Publish writes an event to the outbox_events table.
func (w *OutboxWriter) Publish(ctx context.Context, topic string, event Event) error {
	event = enrichEventFromContext(ctx, event)
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = w.pool.Exec(ctx,
		`INSERT INTO outbox_events
			(event_type, aggregate_type, aggregate_id, payload_json, schema_version, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		topic+"."+event.EventType,
		event.AggregateType,
		event.AggregateID,
		payloadJSON,
		event.SchemaVersion,
		event.TraceID,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

// PublishTx writes an event inside an existing transaction, enabling
// atomic writes with the business data.
func (w *OutboxWriter) PublishTx(ctx context.Context, tx pgx.Tx, topic string, event Event) error {
	event = enrichEventFromContext(ctx, event)
	payloadJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO outbox_events
			(event_type, aggregate_type, aggregate_id, payload_json, schema_version, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		topic+"."+event.EventType,
		event.AggregateType,
		event.AggregateID,
		payloadJSON,
		event.SchemaVersion,
		event.TraceID,
	)
	if err != nil {
		return fmt.Errorf("insert outbox event in tx: %w", err)
	}
	return nil
}

func enrichEventFromContext(ctx context.Context, event Event) Event {
	if event.RequestID == "" {
		event.RequestID = observability.RequestIDFromContext(ctx)
	}
	if event.TraceID == "" {
		event.TraceID = observability.TraceIDFromContext(ctx)
	}
	if event.TraceParent == "" {
		event.TraceParent = observability.TraceParentFromContext(ctx)
	}
	return event
}
