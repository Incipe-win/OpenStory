package consumer

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

const (
	NameAnalytics    = "analytics-consumer"
	NameNotification = "notification-consumer"
	NameFeed         = "feed-consumer"
	NameModeration   = "moderation-consumer"
	NameAudit        = "audit-consumer"
)

// MessageMeta is the Kafka metadata persisted for idempotency and failures.
type MessageMeta struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
}

// Handler applies one event inside the consumer idempotency transaction.
type Handler interface {
	Handle(ctx context.Context, tx pgx.Tx, event eventbus.Event) error
}

// HandlerFunc adapts a function into a Handler.
type HandlerFunc func(ctx context.Context, tx pgx.Tx, event eventbus.Event) error

func (f HandlerFunc) Handle(ctx context.Context, tx pgx.Tx, event eventbus.Event) error {
	return f(ctx, tx, event)
}

// Definition describes one long-running consumer process.
type Definition struct {
	Name    string
	GroupID string
	Topic   string
	Handler Handler
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
