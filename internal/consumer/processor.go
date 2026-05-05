package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

// Processor decodes envelopes and runs the handler through the idempotency store.
type Processor struct {
	def   Definition
	store Store
}

func NewProcessor(def Definition, store Store) *Processor {
	return &Processor{def: def, store: store}
}

func (p *Processor) Process(ctx context.Context, meta MessageMeta) (*eventbus.Event, bool, error) {
	var event eventbus.Event
	if err := json.Unmarshal(meta.Value, &event); err != nil {
		return nil, false, err
	}
	if event.EventID == "" {
		return &event, false, fmt.Errorf("event_id is required")
	}
	ctx = observability.ContextWithTraceParentHeader(ctx, event.TraceParent)
	ctx = observability.ContextWithIDs(ctx, event.RequestID, event.TraceID)
	ctx = observability.ContextWithTraceParent(ctx, event.TraceParent)
	ctx, span := otel.Tracer("openstory/consumer").Start(ctx, "kafka.consume")
	defer span.End()
	span.SetAttributes(
		attribute.String("consumer.name", p.def.Name),
		attribute.String("messaging.destination", p.def.Topic),
		attribute.String("event.id", event.EventID),
		attribute.String("event.type", event.EventType),
	)

	processed, err := p.store.ProcessOnce(ctx, p.def.Name, meta, event, func(ctx context.Context, tx pgx.Tx) error {
		return p.def.Handler.Handle(ctx, tx, event)
	})
	if err != nil {
		return &event, false, err
	}
	return &event, processed, nil
}
