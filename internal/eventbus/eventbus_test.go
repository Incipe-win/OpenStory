package eventbus

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNewEventUsesEnvelopeSchema(t *testing.T) {
	aggregateID := uuid.New()
	userID := uuid.New()

	event := NewEvent("task_created", "generation_task", aggregateID, map[string]any{"status": "queued"}).WithUser(userID).WithTrace("trace-1")

	if event.EventID == "" {
		t.Fatal("expected event_id to be set")
	}
	if event.AggregateID != aggregateID || event.UserID != userID {
		t.Fatalf("unexpected aggregate/user IDs: %#v", event)
	}
	if event.SchemaVersion != EnvelopeSchemaVersion {
		t.Fatalf("schema version = %d, want %d", event.SchemaVersion, EnvelopeSchemaVersion)
	}
	if event.TraceID != "trace-1" {
		t.Fatalf("trace id = %q", event.TraceID)
	}
}

func TestEnvelopeJSONSchemaIsValidJSON(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(EnvelopeJSONSchema), &schema); err != nil {
		t.Fatalf("EnvelopeJSONSchema is invalid JSON: %v", err)
	}
	if schema["title"] != "OpenStoryEventEnvelope" {
		t.Fatalf("unexpected schema title: %v", schema["title"])
	}
}
