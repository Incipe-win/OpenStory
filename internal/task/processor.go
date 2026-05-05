package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/provider"
)

// Payload is the Asynq task payload for generation tasks.
type Payload struct {
	TaskID uuid.UUID `json:"task_id"`
}

// NewAsynqTask creates a new Asynq task for a generation task.
func NewAsynqTask(taskID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(Payload{TaskID: taskID})
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
	repo       Repository
	registry   *provider.Registry
	outbox     eventbus.EventBus
	billingSvc billing.Service
	log        zerolog.Logger
}

// NewProcessor creates a new task processor.
func NewProcessor(repo Repository, registry *provider.Registry, outbox eventbus.EventBus, billingSvc billing.Service, log zerolog.Logger) *Processor {
	return &Processor{repo: repo, registry: registry, outbox: outbox, billingSvc: billingSvc, log: log}
}

// ProcessTask is called by Asynq when a generation task is dequeued.
func (p *Processor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log := p.log.With().Str("task_id", payload.TaskID.String()).Logger()
	log.Info().Msg("processing generation task")

	// Get task from DB
	task, err := p.repo.GetByID(ctx, payload.TaskID)
	if err != nil {
		log.Error().Err(err).Msg("task not found")
		return fmt.Errorf("get task: %w", err)
	}

	// Skip if already terminal
	if IsTerminal(task.Status) {
		log.Info().Str("status", task.Status).Msg("task already terminal, skipping")
		return nil
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
	prov, ok := p.registry.Get(providerName)
	if !ok {
		errMsg := fmt.Sprintf("provider %q not registered", providerName)
		p.failTask(ctx, task, errMsg, log)
		return nil // Don't retry unknown provider
	}

	// Execute
	req := provider.Request{
		TaskType: task.Type,
		Provider: providerName,
		Input:    task.InputJSON,
	}

	result, err := prov.Generate(ctx, req)
	if err != nil {
		// Check if context was canceled (task cancel or timeout)
		if ctx.Err() != nil {
			_ = p.repo.Cancel(ctx, task.ID)
			_ = p.repo.AddEvent(context.Background(), task.ID, EventCanceled, map[string]any{
				"reason": ctx.Err().Error(),
			})
			p.publishTaskEvent(context.Background(), "task_canceled", task, map[string]any{
				"status": StatusCanceled,
				"reason": ctx.Err().Error(),
			})
			log.Info().Msg("task canceled via context")
			return nil
		}

		errMsg := err.Error()
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

	if p.billingSvc != nil && result.CostCredits > 0 {
		if _, err := p.billingSvc.Spend(ctx, task.UserID, result.CostCredits, "generation_task", task.ID, "AI generation task"); err != nil {
			log.Warn().Err(err).Int("cost", result.CostCredits).Msg("credit spend skipped")
		}
	}

	log.Info().
		Int("cost", result.CostCredits).
		Msg("task completed successfully")

	return nil
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
