package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/provider"
)

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

func TestProcessorPublishesOutboxEventsAndSpendsCredits(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	userID := uuid.New()
	projectID := uuid.New()

	repo := &fakeRepo{
		task: &GenerationTask{
			ID:             taskID,
			UserID:         userID,
			ProjectID:      projectID,
			Type:           TypeScript,
			Provider:       "fake",
			Status:         StatusQueued,
			IdempotencyKey: "processor-success",
			InputJSON:      json.RawMessage(`{"prompt":"test"}`),
		},
	}
	outbox := &fakeOutbox{}
	billingSvc := &fakeBilling{}
	registry := provider.NewRegistry()
	registry.Register(fakeProvider{})

	processor := NewProcessor(repo, registry, outbox, billingSvc, testLogger())
	asynqTask, err := NewAsynqTask(taskID)
	if err != nil {
		t.Fatalf("NewAsynqTask returned error: %v", err)
	}

	if err := processor.ProcessTask(ctx, asynqTask); err != nil {
		t.Fatalf("ProcessTask returned error: %v", err)
	}

	if repo.statuses[0] != StatusRunning || repo.statuses[1] != StatusSucceeded {
		t.Fatalf("unexpected statuses: %#v", repo.statuses)
	}
	if repo.costCredits != 7 {
		t.Fatalf("expected cost 7, got %d", repo.costCredits)
	}
	if billingSvc.amount != 7 || billingSvc.referenceID != taskID {
		t.Fatalf("unexpected billing call: amount=%d reference=%s", billingSvc.amount, billingSvc.referenceID)
	}
	assertEventTypes(t, outbox.events, []string{"task_started", "task_succeeded"})
}

func TestProcessorPublishesFailedEventForUnknownProvider(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	repo := &fakeRepo{
		task: &GenerationTask{
			ID:             taskID,
			UserID:         uuid.New(),
			ProjectID:      uuid.New(),
			Type:           TypeScript,
			Provider:       "missing",
			Status:         StatusQueued,
			IdempotencyKey: "processor-failure",
		},
	}
	outbox := &fakeOutbox{}

	processor := NewProcessor(repo, provider.NewRegistry(), outbox, nil, testLogger())
	asynqTask, err := NewAsynqTask(taskID)
	if err != nil {
		t.Fatalf("NewAsynqTask returned error: %v", err)
	}

	if err := processor.ProcessTask(ctx, asynqTask); err != nil {
		t.Fatalf("ProcessTask returned error: %v", err)
	}

	if repo.statuses[len(repo.statuses)-1] != StatusFailed {
		t.Fatalf("expected final status failed, got %#v", repo.statuses)
	}
	assertEventTypes(t, outbox.events, []string{"task_started", "task_failed"})
}

func assertEventTypes(t *testing.T, events []eventbus.Event, want []string) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %d: %#v", len(want), len(events), events)
	}
	for i := range want {
		if events[i].EventType != want[i] {
			t.Fatalf("event %d: expected %q, got %q", i, want[i], events[i].EventType)
		}
	}
}

type fakeRepo struct {
	task        *GenerationTask
	events      []string
	statuses    []string
	costCredits int
}

func (f *fakeRepo) Create(context.Context, *GenerationTask) error { return nil }
func (f *fakeRepo) GetByID(context.Context, uuid.UUID) (*GenerationTask, error) {
	if f.task == nil {
		return nil, ErrTaskNotFound
	}
	return f.task, nil
}
func (f *fakeRepo) GetByIdempotencyKey(context.Context, string) (*GenerationTask, error) {
	return nil, ErrTaskNotFound
}
func (f *fakeRepo) ListByProject(context.Context, uuid.UUID, int, int) ([]GenerationTask, int, error) {
	return nil, 0, nil
}
func (f *fakeRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status string, _ *json.RawMessage, _ *string) error {
	f.statuses = append(f.statuses, status)
	f.task.Status = status
	return nil
}
func (f *fakeRepo) UpdateCost(_ context.Context, _ uuid.UUID, costCredits int) error {
	f.costCredits = costCredits
	return nil
}
func (f *fakeRepo) SetRunning(context.Context, uuid.UUID) error {
	f.statuses = append(f.statuses, StatusRunning)
	f.task.Status = StatusRunning
	return nil
}
func (f *fakeRepo) IncrRetry(context.Context, uuid.UUID) error { return nil }
func (f *fakeRepo) Cancel(context.Context, uuid.UUID) error {
	f.statuses = append(f.statuses, StatusCanceled)
	f.task.Status = StatusCanceled
	return nil
}
func (f *fakeRepo) AddEvent(_ context.Context, _ uuid.UUID, eventType string, _ any) error {
	f.events = append(f.events, eventType)
	return nil
}
func (f *fakeRepo) ListEvents(context.Context, uuid.UUID) ([]TaskEvent, error) { return nil, nil }

type fakeOutbox struct {
	events []eventbus.Event
}

func (f *fakeOutbox) Publish(_ context.Context, _ string, event eventbus.Event) error {
	f.events = append(f.events, event)
	return nil
}

type fakeBilling struct {
	amount      int
	referenceID uuid.UUID
}

func (f *fakeBilling) Spend(_ context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*billing.LedgerEntry, error) {
	if referenceType != "generation_task" {
		return nil, errors.New("unexpected reference type")
	}
	f.amount = amount
	f.referenceID = referenceID
	return &billing.LedgerEntry{
		ID:            uuid.New(),
		UserID:        userID,
		Amount:        -amount,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
	}, nil
}

type fakeProvider struct{}

func (fakeProvider) Name() string         { return "fake" }
func (fakeProvider) Supports(string) bool { return true }
func (fakeProvider) Generate(context.Context, provider.Request) (*provider.Result, error) {
	return &provider.Result{
		Output:      json.RawMessage(`{"ok":true}`),
		CostCredits: 7,
	}, nil
}
