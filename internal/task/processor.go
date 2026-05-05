package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/compose"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/Incipe-win/OpenStory/internal/provider"
)

// Payload is the Asynq task payload for generation tasks.
type Payload struct {
	TaskID      uuid.UUID `json:"task_id"`
	RequestID   string    `json:"request_id,omitempty"`
	TraceID     string    `json:"trace_id,omitempty"`
	TraceParent string    `json:"traceparent,omitempty"`
}

// NewAsynqTask creates a new Asynq task for a generation task.
func NewAsynqTask(taskID uuid.UUID) (*asynq.Task, error) {
	return NewAsynqTaskWithContext(context.Background(), taskID)
}

func NewAsynqTaskWithContext(ctx context.Context, taskID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(Payload{
		TaskID:      taskID,
		RequestID:   observability.RequestIDFromContext(ctx),
		TraceID:     observability.TraceIDFromContext(ctx),
		TraceParent: observability.TraceParentFromContext(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(AsynqTaskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Queue("generation"),
		asynq.TaskID(taskID.String()),
	), nil
}

// Processor handles Asynq generation tasks.
type Processor struct {
	repo        Repository
	registry    *provider.Registry
	outbox      eventbus.EventBus
	billingSvc  billing.Service
	recorder    provider.CallRecorder
	composer    *compose.Service
	maxAttempts int
	log         zerolog.Logger
}

// NewProcessor creates a new task processor.
func NewProcessor(repo Repository, registry *provider.Registry, outbox eventbus.EventBus, billingSvc billing.Service, recorder provider.CallRecorder, maxAttempts int, log zerolog.Logger) *Processor {
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	return &Processor{repo: repo, registry: registry, outbox: outbox, billingSvc: billingSvc, recorder: recorder, maxAttempts: maxAttempts, log: log}
}

func (p *Processor) SetComposer(composer *compose.Service) {
	p.composer = composer
}

// ProcessTask is called by Asynq when a generation task is dequeued.
func (p *Processor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	started := time.Now()
	var payload Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	ctx = observability.ContextWithTraceParentHeader(ctx, payload.TraceParent)
	ctx = observability.ContextWithIDs(ctx, payload.RequestID, payload.TraceID)
	ctx, span := otel.Tracer("openstory/worker").Start(ctx, "generation.execute")
	defer span.End()
	ctx = observability.ContextWithTraceID(ctx, observability.TraceIDFromContext(ctx))
	ctx = observability.ContextWithTraceParent(ctx, observability.TraceParentFromContext(ctx))
	span.SetAttributes(attribute.String("task.id", payload.TaskID.String()))

	log := p.log.With().
		Str("task_id", payload.TaskID.String()).
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("trace_id", observability.TraceIDFromContext(ctx)).
		Logger()
	log.Info().Msg("processing generation task")

	// Get task from DB
	task, err := p.repo.GetByID(ctx, payload.TaskID)
	if err != nil {
		log.Error().Err(err).Msg("task not found")
		observability.ObserveTask("unknown", "unknown", StatusFailed, time.Since(started))
		return fmt.Errorf("get task: %w", err)
	}
	span.SetAttributes(
		attribute.String("task.type", task.Type),
		attribute.String("task.provider", task.Provider),
		attribute.String("user.id", task.UserID.String()),
	)

	// Skip if already terminal
	if IsTerminal(task.Status) {
		log.Info().Str("status", task.Status).Msg("task already terminal, skipping")
		return nil
	}

	reservedCredits := EstimateCostCredits(task.Type)
	if reservedCredits > 0 && p.billingSvc != nil {
		if _, err := p.billingSvc.Reserve(ctx, task.UserID, reservedCredits, "generation_task", task.ID, "Reserve credits for generation task"); err != nil {
			if errors.Is(err, billing.ErrInsufficientCredits) || errors.Is(err, billing.ErrAccountNotFound) {
				p.failTask(ctx, task, err.Error(), log)
				observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
				return nil
			}
			log.Error().Err(err).Msg("failed to reserve credits")
			return fmt.Errorf("reserve credits: %w", err)
		}
	}

	// Set running
	if err := p.repo.SetRunning(ctx, task.ID); err != nil {
		log.Error().Err(err).Msg("failed to set running")
		return fmt.Errorf("set running: %w", err)
	}
	_ = p.repo.AddEvent(ctx, task.ID, EventStarted, map[string]any{
		"retry_count": task.RetryCount,
	})
	p.publishTaskEvent(ctx, "task_started", task, map[string]any{
		"status":      StatusRunning,
		"retry_count": task.RetryCount,
	})

	// Get provider
	providerName := task.Provider
	if providerName == "" {
		providerName = "mock"
		task.Provider = providerName
	}
	result, _, err := p.execute(ctx, task, providerName)
	if err != nil {
		// Check if context was canceled (task cancel or timeout)
		if ctx.Err() != nil {
			_ = p.repo.Cancel(ctx, task.ID)
			_ = p.repo.AddEvent(context.Background(), task.ID, EventCanceled, map[string]any{
				"reason": ctx.Err().Error(),
			})
			p.refundReservedCredits(context.Background(), task, "Task canceled")
			p.publishTaskEvent(context.Background(), "task_canceled", task, map[string]any{
				"status": StatusCanceled,
				"reason": ctx.Err().Error(),
			})
			observability.ObserveTask(task.Type, task.Provider, StatusCanceled, time.Since(started))
			log.Info().Msg("task canceled via context")
			return nil
		}

		errMsg := err.Error()
		if strings.Contains(errMsg, "not registered") || strings.Contains(errMsg, "not configured") {
			p.refundReservedCredits(ctx, task, "Task failed before provider execution")
			p.failTask(ctx, task, errMsg, log)
			observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
			return nil
		}
		if isFinalAsynqAttempt(ctx) {
			p.refundReservedCredits(ctx, task, "Task failed after retries")
			p.failTask(ctx, task, errMsg, log)
			observability.ObserveTask(task.Type, task.Provider, StatusFailed, time.Since(started))
			return nil
		}
		// Increment retry count
		_ = p.repo.IncrRetry(ctx, task.ID)
		_ = p.repo.AddEvent(ctx, task.ID, EventRetried, map[string]any{
			"error": errMsg,
		})
		p.publishTaskEvent(ctx, "task_retried", task, map[string]any{
			"status": StatusRunning,
			"error":  errMsg,
		})
		log.Warn().Err(err).Msg("provider returned error, will retry")
		return err // Asynq will retry
	}

	// Success - update task
	outputJSON := result.Output
	if p.billingSvc != nil {
		if _, err := p.billingSvc.Confirm(ctx, task.UserID, result.CostCredits, "generation_task", task.ID, "Confirm generation task credits"); err != nil {
			log.Error().Err(err).Msg("failed to confirm credits")
			return fmt.Errorf("confirm credits: %w", err)
		}
	}
	if err := p.repo.UpdateStatus(ctx, task.ID, StatusSucceeded, &outputJSON, nil); err != nil {
		log.Error().Err(err).Msg("failed to update succeeded status")
		return fmt.Errorf("update status: %w", err)
	}

	_ = p.repo.AddEvent(ctx, task.ID, EventCompleted, map[string]any{
		"cost_credits": result.CostCredits,
	})
	p.publishTaskEvent(ctx, "task_succeeded", task, map[string]any{
		"status":       StatusSucceeded,
		"cost_credits": result.CostCredits,
	})

	_ = p.repo.UpdateCost(ctx, task.ID, result.CostCredits)

	log.Info().
		Int("cost", result.CostCredits).
		Msg("task completed successfully")
	observability.ObserveTask(task.Type, task.Provider, StatusSucceeded, time.Since(started))

	return nil
}

func (p *Processor) execute(ctx context.Context, task *GenerationTask, providerName string) (*provider.Result, time.Duration, error) {
	if task.Type == TypeCompose && providerName == "ffmpeg" {
		if p.composer == nil {
			err := fmt.Errorf("ffmpeg composer is not configured")
			p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "failed", 0, provider.Usage{}, 0, err)
			return nil, 0, err
		}
		started := time.Now()
		result, err := p.composer.Compose(ctx, task.ID, task.UserID, task.ProjectID, task.InputJSON)
		duration := time.Since(started)
		if err != nil {
			p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "failed", duration, provider.Usage{}, 0, err)
			return nil, duration, err
		}
		output, _ := json.Marshal(result)
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityVideoGenerate, "ffmpeg", "succeeded", duration, provider.Usage{}, 0, nil)
		return &provider.Result{
			Output:     output,
			Capability: provider.CapabilityVideoGenerate,
			Model:      "ffmpeg",
		}, duration, nil
	}

	prov, ok := p.registry.Get(providerName)
	if !ok {
		err := fmt.Errorf("provider %q not registered", providerName)
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityForTask(task.Type), "", "failed", 0, provider.Usage{}, 0, err)
		return nil, 0, err
	}

	req := provider.Request{
		TaskType: task.Type,
		Provider: providerName,
		Input:    task.InputJSON,
	}

	started := time.Now()
	result, err := provider.Execute(ctx, prov, req, p.maxAttempts)
	duration := time.Since(started)
	if err != nil {
		p.recordProviderCall(ctx, task.ID, providerName, provider.CapabilityForTask(task.Type), "", "failed", duration, provider.Usage{}, 0, err)
		return nil, duration, err
	}
	p.recordProviderCall(ctx, task.ID, providerName, result.Capability, result.Model, "succeeded", duration, result.Usage, result.CostCredits, nil)
	return result, duration, nil
}

func (p *Processor) recordProviderCall(ctx context.Context, taskID uuid.UUID, providerName string, capability provider.Capability, model, status string, duration time.Duration, usage provider.Usage, costCredits int, callErr error) {
	observability.ObserveProviderCall(providerName, string(capability), status, duration)
	if p.recorder == nil {
		return
	}
	var errMsg *string
	if callErr != nil {
		msg := callErr.Error()
		errMsg = &msg
	}
	if err := p.recorder.Record(ctx, provider.CallLog{
		TaskID:       taskID,
		Provider:     providerName,
		Capability:   capability,
		Model:        model,
		Status:       status,
		Duration:     duration,
		Usage:        usage,
		CostCredits:  costCredits,
		ErrorMessage: errMsg,
	}); err != nil {
		p.log.Warn().Err(err).Str("provider", providerName).Msg("failed to record provider call")
	}
}

func (p *Processor) failTask(ctx context.Context, task *GenerationTask, errMsg string, log zerolog.Logger) {
	_ = p.repo.UpdateStatus(ctx, task.ID, StatusFailed, nil, &errMsg)
	_ = p.repo.AddEvent(ctx, task.ID, EventFailed, map[string]any{"error": errMsg})
	p.publishTaskEvent(ctx, "task_failed", task, map[string]any{
		"status": StatusFailed,
		"error":  errMsg,
	})
	log.Error().Str("error", errMsg).Msg("task failed permanently")
}

func (p *Processor) refundReservedCredits(ctx context.Context, task *GenerationTask, reason string) {
	if p.billingSvc == nil {
		return
	}
	if _, err := p.billingSvc.Refund(ctx, task.UserID, "generation_task", task.ID, reason); err != nil &&
		!errors.Is(err, billing.ErrReservationNotFound) &&
		!errors.Is(err, billing.ErrReservationConfirmed) {
		p.log.Warn().Err(err).Str("task_id", task.ID.String()).Msg("failed to refund reserved credits")
	}
}

func isFinalAsynqAttempt(ctx context.Context) bool {
	retryCount, okRetry := asynq.GetRetryCount(ctx)
	maxRetry, okMax := asynq.GetMaxRetry(ctx)
	return okRetry && okMax && retryCount >= maxRetry
}

func (p *Processor) publishTaskEvent(ctx context.Context, eventType string, task *GenerationTask, payload map[string]any) {
	if p.outbox == nil {
		return
	}
	payload["task_type"] = task.Type
	payload["provider"] = task.Provider
	payload["project_id"] = task.ProjectID
	_ = p.outbox.Publish(ctx, eventbus.TopicGenerationTaskEvents,
		eventbus.NewEvent(eventType, "generation_task", task.ID, payload).WithUser(task.UserID))
}
