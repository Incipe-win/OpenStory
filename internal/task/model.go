package task

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ── Task Statuses ───────────────────────────────────

const (
	StatusPending   = "pending"
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
)

// IsTerminal returns true if the task has reached a final state.
func IsTerminal(status string) bool {
	return status == StatusSucceeded || status == StatusFailed || status == StatusCanceled
}

// ── Task Types (maps to workflow node types) ────────

const (
	TypeIdea             = "idea"
	TypeScript           = "script"
	TypeCharacter        = "character"
	TypeCharacters       = "characters"
	TypeScene            = "scene"
	TypeStoryboard       = "storyboard"
	TypeImagePrompt      = "image_prompt"
	TypeVideoPrompt      = "video_prompt"
	TypeCreativePipeline = "creative_pipeline"
	TypeImageGeneration  = "image_generation"
	TypeVideoGeneration  = "video_generation"
	TypeAudio            = "audio"
	TypeSubtitle         = "subtitle"
	TypeCompose          = "compose"
)

// EstimateCostCredits returns the credits reserved before a task starts.
// Providers may report a lower final cost; unused reserved credits are refunded.
func EstimateCostCredits(taskType string) int {
	costs := map[string]int{
		TypeIdea:             1,
		TypeScript:           2,
		TypeCharacter:        1,
		TypeCharacters:       1,
		TypeScene:            1,
		TypeStoryboard:       2,
		TypeImagePrompt:      1,
		TypeVideoPrompt:      1,
		TypeCreativePipeline: 5,
		TypeImageGeneration:  5,
		TypeVideoGeneration:  10,
		TypeAudio:            3,
		TypeSubtitle:         1,
		TypeCompose:          5,
	}
	if cost, ok := costs[taskType]; ok {
		return cost
	}
	return 1
}

// ── Models ──────────────────────────────────────────

// GenerationTask represents an AI generation task.
type GenerationTask struct {
	ID             uuid.UUID        `json:"id"`
	UserID         uuid.UUID        `json:"user_id"`
	ProjectID      uuid.UUID        `json:"project_id"`
	WorkflowID     *uuid.UUID       `json:"workflow_id,omitempty"`
	NodeID         *uuid.UUID       `json:"node_id,omitempty"`
	Type           string           `json:"type"`
	Provider       string           `json:"provider"`
	Status         string           `json:"status"`
	IdempotencyKey string           `json:"idempotency_key"`
	InputJSON      json.RawMessage  `json:"input"`
	OutputJSON     *json.RawMessage `json:"output,omitempty"`
	ErrorMessage   *string          `json:"error_message,omitempty"`
	CostCredits    int              `json:"cost_credits"`
	RetryCount     int              `json:"retry_count"`
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// TaskEvent records a state transition for a generation task.
type TaskEvent struct {
	ID          uuid.UUID       `json:"id"`
	TaskID      uuid.UUID       `json:"task_id"`
	Type        string          `json:"type"`
	PayloadJSON json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ── Event Types ─────────────────────────────────────

const (
	EventCreated   = "created"
	EventQueued    = "queued"
	EventStarted   = "started"
	EventCompleted = "completed"
	EventFailed    = "failed"
	EventCanceled  = "canceled"
	EventRetried   = "retried"
)

// ── Asynq Task Type Name ────────────────────────────

const AsynqTaskType = "generation:execute"
