// Package eventbus provides the EventBus interface and event envelope definition.
package eventbus

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ── Topics ──────────────────────────────────────────

const (
	TopicGenerationTaskEvents = "generation.task.events"
	TopicAssetEvents          = "asset.events"
	TopicWorkEvents           = "work.events"
	TopicCreditEvents         = "credit.events"
	TopicModerationEvents     = "moderation.events"
	TopicAuditEvents          = "audit.events"
	TopicNotificationEvents   = "notification.events"
	TopicDLQEvents            = "openstory.dlq"
)

// AllTopics lists all Kafka topics to auto-create.
var AllTopics = []string{
	TopicGenerationTaskEvents,
	TopicAssetEvents,
	TopicWorkEvents,
	TopicCreditEvents,
	TopicModerationEvents,
	TopicAuditEvents,
	TopicNotificationEvents,
	TopicDLQEvents,
}

// ── Event Envelope ──────────────────────────────────

const EnvelopeSchemaVersion = 1

// EnvelopeJSONSchema documents the canonical JSON envelope published to Kafka.
const EnvelopeJSONSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "OpenStoryEventEnvelope",
  "type": "object",
  "required": ["event_id", "event_type", "aggregate_type", "aggregate_id", "schema_version", "occurred_at", "payload"],
  "properties": {
    "event_id": {"type": "string", "format": "uuid"},
    "event_type": {"type": "string"},
    "aggregate_type": {"type": "string"},
    "aggregate_id": {"type": "string", "format": "uuid"},
    "user_id": {"type": "string", "format": "uuid"},
    "request_id": {"type": "string"},
    "traceparent": {"type": "string"},
    "trace_id": {"type": "string"},
    "schema_version": {"type": "integer", "minimum": 1},
    "occurred_at": {"type": "string", "format": "date-time"},
    "payload": {"type": "object"}
  }
}`

// Event is the canonical event envelope written to outbox and published to Kafka.
type Event struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   uuid.UUID `json:"aggregate_id"`
	UserID        uuid.UUID `json:"user_id,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	TraceParent   string    `json:"traceparent,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	SchemaVersion int       `json:"schema_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	Payload       any       `json:"payload"`
}

// NewEvent creates a new event with a unique ID and current timestamp.
func NewEvent(eventType, aggregateType string, aggregateID uuid.UUID, payload any) Event {
	return Event{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		SchemaVersion: EnvelopeSchemaVersion,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}
}

// WithUser sets the user ID on the event.
func (e Event) WithUser(userID uuid.UUID) Event {
	e.UserID = userID
	return e
}

// WithTrace sets the trace ID on the event.
func (e Event) WithTrace(traceID string) Event {
	e.TraceID = traceID
	return e
}

// ── EventBus Interface ──────────────────────────────

// EventBus defines the interface for publishing events.
// Business code should use this interface — never publish to Kafka directly.
type EventBus interface {
	// Publish writes an event. For OutboxWriter this goes to the DB outbox table.
	Publish(ctx context.Context, topic string, event Event) error
}
