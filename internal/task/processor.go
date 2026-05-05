package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

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
	repo     Repository
	registry *provider.Registry
	log      zerolog.Logger
}

// NewProcessor creates a new task processor.
func NewProcessor(repo Repository, registry *provider.Registry, log zerolog.Logger) *Processor {
	return &Processor{repo: repo, registry: registry, log: log}
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

	// Get provider
	providerName := task.Provider
	if providerName == "" {
		providerName = "mock"
	}
	prov, ok := p.registry.Get(providerName)
	if !ok {
		errMsg := fmt.Sprintf("provider %q not registered", providerName)
		p.failTask(ctx, task.ID, errMsg, log)
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
			log.Info().Msg("task canceled via context")
			return nil
		}

		errMsg := err.Error()
		// Increment retry count
		_ = p.repo.IncrRetry(ctx, task.ID)
		_ = p.repo.AddEvent(ctx, task.ID, EventRetried, map[string]any{
			"error": errMsg,
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

	// Update cost
	_, _ = p.repo.(*PgRepository).pool.Exec(ctx,
		`UPDATE generation_tasks SET cost_credits = $2 WHERE id = $1`,
		task.ID, result.CostCredits)

	log.Info().
		Int("cost", result.CostCredits).
		Msg("task completed successfully")

	return nil
}

func (p *Processor) failTask(ctx context.Context, taskID uuid.UUID, errMsg string, log zerolog.Logger) {
	_ = p.repo.UpdateStatus(ctx, taskID, StatusFailed, nil, &errMsg)
	_ = p.repo.AddEvent(ctx, taskID, EventFailed, map[string]any{"error": errMsg})
	log.Error().Str("error", errMsg).Msg("task failed permanently")
}
