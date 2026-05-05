package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CallLog struct {
	TaskID       uuid.UUID
	Provider     string
	Capability   Capability
	Model        string
	Status       string
	Duration     time.Duration
	Usage        Usage
	CostCredits  int
	ErrorMessage *string
}

type CallRecorder interface {
	Record(ctx context.Context, log CallLog) error
}

type PgCallRecorder struct {
	pool *pgxpool.Pool
}

func NewPgCallRecorder(pool *pgxpool.Pool) *PgCallRecorder {
	return &PgCallRecorder{pool: pool}
}

func (r *PgCallRecorder) Record(ctx context.Context, log CallLog) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO provider_call_logs
			(task_id, provider, capability, model, status, duration_ms, prompt_tokens,
			 completion_tokens, total_tokens, cost_credits, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		log.TaskID, log.Provider, string(log.Capability), log.Model, log.Status,
		log.Duration.Milliseconds(), log.Usage.PromptTokens, log.Usage.CompletionTokens,
		log.Usage.TotalTokens, log.CostCredits, log.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("insert provider call log: %w", err)
	}
	return nil
}
