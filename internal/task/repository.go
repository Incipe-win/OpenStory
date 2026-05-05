package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrIdempotencyConflict = errors.New("task with this idempotency key already exists")
)

// Repository defines generation task database operations.
type Repository interface {
	Create(ctx context.Context, t *GenerationTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*GenerationTask, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*GenerationTask, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, page, pageSize int) ([]GenerationTask, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, output *json.RawMessage, errMsg *string) error
	SetRunning(ctx context.Context, id uuid.UUID) error
	IncrRetry(ctx context.Context, id uuid.UUID) error
	Cancel(ctx context.Context, id uuid.UUID) error
	AddEvent(ctx context.Context, taskID uuid.UUID, eventType string, payload any) error
	ListEvents(ctx context.Context, taskID uuid.UUID) ([]TaskEvent, error)
}

// PgRepository implements Repository using pgxpool.
type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) Create(ctx context.Context, t *GenerationTask) error {
	if t.InputJSON == nil {
		t.InputJSON = json.RawMessage(`{}`)
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO generation_tasks
			(user_id, project_id, workflow_id, node_id, type, provider, idempotency_key, input_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, status, cost_credits, retry_count, created_at, updated_at`,
		t.UserID, t.ProjectID, t.WorkflowID, t.NodeID, t.Type, t.Provider, t.IdempotencyKey, t.InputJSON,
	).Scan(&t.ID, &t.Status, &t.CostCredits, &t.RetryCount, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if contains(err.Error(), "uq_tasks_idempotency") {
			return ErrIdempotencyConflict
		}
		return fmt.Errorf("inserting task: %w", err)
	}
	return nil
}

func (r *PgRepository) GetByID(ctx context.Context, id uuid.UUID) (*GenerationTask, error) {
	return r.scanTask(r.pool.QueryRow(ctx,
		`SELECT id, user_id, project_id, workflow_id, node_id, type, provider, status,
		        idempotency_key, input_json, output_json, error_message, cost_credits,
		        retry_count, started_at, finished_at, created_at, updated_at
		 FROM generation_tasks WHERE id = $1`, id))
}

func (r *PgRepository) GetByIdempotencyKey(ctx context.Context, key string) (*GenerationTask, error) {
	return r.scanTask(r.pool.QueryRow(ctx,
		`SELECT id, user_id, project_id, workflow_id, node_id, type, provider, status,
		        idempotency_key, input_json, output_json, error_message, cost_credits,
		        retry_count, started_at, finished_at, created_at, updated_at
		 FROM generation_tasks WHERE idempotency_key = $1`, key))
}

func (r *PgRepository) scanTask(row pgx.Row) (*GenerationTask, error) {
	t := &GenerationTask{}
	err := row.Scan(&t.ID, &t.UserID, &t.ProjectID, &t.WorkflowID, &t.NodeID,
		&t.Type, &t.Provider, &t.Status, &t.IdempotencyKey, &t.InputJSON,
		&t.OutputJSON, &t.ErrorMessage, &t.CostCredits, &t.RetryCount,
		&t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scanning task: %w", err)
	}
	return t, nil
}

func (r *PgRepository) ListByProject(ctx context.Context, projectID uuid.UUID, page, pageSize int) ([]GenerationTask, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM generation_tasks WHERE project_id = $1`, projectID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting tasks: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, project_id, workflow_id, node_id, type, provider, status,
		        idempotency_key, input_json, output_json, error_message, cost_credits,
		        retry_count, started_at, finished_at, created_at, updated_at
		 FROM generation_tasks WHERE project_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing tasks: %w", err)
	}
	defer rows.Close()

	var tasks []GenerationTask
	for rows.Next() {
		var t GenerationTask
		if err := rows.Scan(&t.ID, &t.UserID, &t.ProjectID, &t.WorkflowID, &t.NodeID,
			&t.Type, &t.Provider, &t.Status, &t.IdempotencyKey, &t.InputJSON,
			&t.OutputJSON, &t.ErrorMessage, &t.CostCredits, &t.RetryCount,
			&t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning task row: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, total, nil
}

func (r *PgRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, output *json.RawMessage, errMsg *string) error {
	var tag string
	if IsTerminal(status) {
		_, err := r.pool.Exec(ctx,
			`UPDATE generation_tasks SET status = $2, output_json = $3, error_message = $4, finished_at = NOW()
			 WHERE id = $1`, id, status, output, errMsg)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		tag = status
	} else {
		_, err := r.pool.Exec(ctx,
			`UPDATE generation_tasks SET status = $2 WHERE id = $1`, id, status)
		if err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		tag = status
	}
	_ = tag
	return nil
}

func (r *PgRepository) SetRunning(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE generation_tasks SET status = 'running', started_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("set running: %w", err)
	}
	return nil
}

func (r *PgRepository) IncrRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE generation_tasks SET retry_count = retry_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("incr retry: %w", err)
	}
	return nil
}

func (r *PgRepository) Cancel(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE generation_tasks SET status = 'canceled', finished_at = NOW()
		 WHERE id = $1 AND status NOT IN ('succeeded', 'failed', 'canceled')`, id)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (r *PgRepository) AddEvent(ctx context.Context, taskID uuid.UUID, eventType string, payload any) error {
	payloadJSON, _ := json.Marshal(payload)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO task_events (task_id, type, payload_json) VALUES ($1, $2, $3)`,
		taskID, eventType, payloadJSON)
	if err != nil {
		return fmt.Errorf("add event: %w", err)
	}
	return nil
}

func (r *PgRepository) ListEvents(ctx context.Context, taskID uuid.UUID) ([]TaskEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, task_id, type, payload_json, created_at
		 FROM task_events WHERE task_id = $1 ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []TaskEvent
	for rows.Next() {
		var e TaskEvent
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Type, &e.PayloadJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}
	return events, nil
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
