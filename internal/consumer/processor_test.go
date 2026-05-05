package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

func TestProcessorConsumesEventIdempotently(t *testing.T) {
	ctx := context.Background()
	event := eventbus.NewEvent("task_succeeded", "generation_task", uuid.New(), map[string]any{
		"status": "succeeded",
	})
	value := mustJSON(t, event)

	store := newMemoryStore()
	handled := 0
	def := Definition{
		Name:    NameAnalytics,
		GroupID: NameAnalytics,
		Topic:   eventbus.TopicGenerationTaskEvents,
		Handler: HandlerFunc(func(context.Context, pgx.Tx, eventbus.Event) error {
			handled++
			return nil
		}),
	}
	processor := NewProcessor(def, store)
	meta := MessageMeta{Topic: def.Topic, Partition: 0, Offset: 42, Value: value}

	if _, processed, err := processor.Process(ctx, meta); err != nil || !processed {
		t.Fatalf("first process: processed=%v err=%v", processed, err)
	}
	if _, processed, err := processor.Process(ctx, meta); err != nil || processed {
		t.Fatalf("second process should be duplicate: processed=%v err=%v", processed, err)
	}
	if handled != 1 {
		t.Fatalf("handler called %d times, want 1", handled)
	}
}

func TestProcessorRollsBackIdempotencyOnHandlerError(t *testing.T) {
	ctx := context.Background()
	event := eventbus.NewEvent("task_succeeded", "generation_task", uuid.New(), nil)
	store := newMemoryStore()
	def := Definition{
		Name:  NameAnalytics,
		Topic: eventbus.TopicGenerationTaskEvents,
		Handler: HandlerFunc(func(context.Context, pgx.Tx, eventbus.Event) error {
			return errors.New("boom")
		}),
	}
	processor := NewProcessor(def, store)
	meta := MessageMeta{Topic: def.Topic, Partition: 0, Offset: 1, Value: mustJSON(t, event)}

	if _, _, err := processor.Process(ctx, meta); err == nil {
		t.Fatal("expected handler error")
	}
	if store.seen[event.EventID] {
		t.Fatal("event should not be marked processed after handler failure")
	}
}

func TestProcessorRejectsMissingEventID(t *testing.T) {
	def := Definition{Name: NameAnalytics, Topic: eventbus.TopicGenerationTaskEvents}
	processor := NewProcessor(def, newMemoryStore())

	_, _, err := processor.Process(context.Background(), MessageMeta{Topic: def.Topic, Value: []byte(`{"event_type":"x"}`)})
	if err == nil {
		t.Fatal("expected missing event_id error")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return data
}

type memoryStore struct {
	seen map[string]bool
}

func newMemoryStore() *memoryStore {
	return &memoryStore{seen: map[string]bool{}}
}

func (s *memoryStore) ProcessOnce(ctx context.Context, consumerName string, meta MessageMeta, event eventbus.Event, handle func(context.Context, pgx.Tx) error) (bool, error) {
	key := consumerName + ":" + event.EventID
	if s.seen[key] {
		return false, nil
	}
	if err := handle(ctx, nil); err != nil {
		return false, err
	}
	s.seen[key] = true
	s.seen[event.EventID] = true
	return true, nil
}

func (s *memoryStore) RecordFailure(context.Context, string, MessageMeta, *eventbus.Event, error) (int, error) {
	return 1, nil
}
