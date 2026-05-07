package handler

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/task"
)

// TaskHandler handles generation task API endpoints.
type TaskHandler struct {
	repo        task.Repository
	asynqClient *asynq.Client
	auditLog    *audit.Logger
	billing     billing.Service
	outbox      eventbus.EventBus
	taskLimit   int
	log         zerolog.Logger
}

func NewTaskHandler(repo task.Repository, asynqClient *asynq.Client, auditLog *audit.Logger, billingSvc billing.Service, outbox eventbus.EventBus, taskLimit int, log zerolog.Logger) *TaskHandler {
	return &TaskHandler{repo: repo, asynqClient: asynqClient, auditLog: auditLog, billing: billingSvc, outbox: outbox, taskLimit: taskLimit, log: log}
}

// ── Create ──────────────────────────────────────────

type createTaskRequest struct {
	ProjectID      uuid.UUID  `json:"project_id"      binding:"required"`
	WorkflowID     *uuid.UUID `json:"workflow_id"`
	NodeID         *uuid.UUID `json:"node_id"`
	Type           string     `json:"type"             binding:"required"`
	Provider       string     `json:"provider"`
	IdempotencyKey string     `json:"idempotency_key"  binding:"required"`
	Input          any        `json:"input"`
}

// Create handles POST /api/generation/tasks
func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	if req.Provider == "" {
		req.Provider = "openai-compatible"
	}

	userID := middleware.GetUserID(c)

	// Check idempotency: if task already exists, return it
	existing, err := h.repo.GetByIdempotencyKey(c.Request.Context(), req.IdempotencyKey)
	if err == nil {
		OK(c, existing)
		return
	}

	if h.taskLimit > 0 {
		active, err := h.repo.CountActiveByUser(c.Request.Context(), userID)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to count active tasks")
			InternalError(c, "internal error")
			return
		}
		if active >= h.taskLimit {
			TooManyRequests(c, "task concurrency limit exceeded")
			return
		}
	}

	t := &task.GenerationTask{
		UserID:         userID,
		ProjectID:      req.ProjectID,
		WorkflowID:     req.WorkflowID,
		NodeID:         req.NodeID,
		Type:           req.Type,
		Provider:       req.Provider,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Marshal input if provided
	if req.Input != nil {
		inputJSON, _ := marshalJSON(req.Input)
		t.InputJSON = inputJSON
	}

	if err := h.repo.Create(c.Request.Context(), t); err != nil {
		if errors.Is(err, task.ErrIdempotencyConflict) {
			existing, _ := h.repo.GetByIdempotencyKey(c.Request.Context(), req.IdempotencyKey)
			if existing != nil {
				OK(c, existing)
				return
			}
			Conflict(c, "task with this idempotency key already exists")
			return
		}
		h.log.Error().Err(err).Msg("failed to create task")
		InternalError(c, "internal error")
		return
	}

	// Record creation event
	_ = h.repo.AddEvent(c.Request.Context(), t.ID, task.EventCreated, map[string]any{
		"type":     t.Type,
		"provider": t.Provider,
	})
	h.publishTaskEvent(c, "task_created", t, map[string]any{
		"status": task.StatusPending,
	})

	// Enqueue Asynq job
	asynqTask, err := task.NewAsynqTaskWithContext(c.Request.Context(), t.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create asynq task")
		InternalError(c, "internal error")
		return
	}

	info, err := h.asynqClient.Enqueue(asynqTask)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to enqueue task")
		// Task is created but not queued — set to pending still
		InternalError(c, "failed to enqueue task")
		return
	}

	// Update status to queued
	_ = h.repo.UpdateStatus(c.Request.Context(), t.ID, task.StatusQueued, nil, nil)
	_ = h.repo.AddEvent(c.Request.Context(), t.ID, task.EventQueued, map[string]any{
		"asynq_id": info.ID,
		"queue":    info.Queue,
	})
	t.Status = task.StatusQueued
	h.publishTaskEvent(c, "task_queued", t, map[string]any{
		"status":   task.StatusQueued,
		"asynq_id": info.ID,
		"queue":    info.Queue,
	})

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "create_task", ResourceType: "generation_task", ResourceID: &t.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"type": t.Type, "provider": t.Provider},
	})

	Created(c, t)
}

// ── Get ─────────────────────────────────────────────

// Get handles GET /api/generation/tasks/:id
func (h *TaskHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid task id")
		return
	}

	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			NotFound(c, "task not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get task")
		InternalError(c, "internal error")
		return
	}

	userID := middleware.GetUserID(c)
	if t.UserID != userID {
		NotFound(c, "task not found")
		return
	}

	OK(c, t)
}

// ── ListByProject ───────────────────────────────────

// ListByProject handles GET /api/projects/:id/tasks
func (h *TaskHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}

	page, pageSize := Pagination(c)
	tasks, total, err := h.repo.ListByProject(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list tasks")
		InternalError(c, "internal error")
		return
	}

	if tasks == nil {
		tasks = []task.GenerationTask{}
	}
	OKWithMeta(c, tasks, Meta{Page: page, PageSize: pageSize, Total: total})
}

// ── Cancel ──────────────────────────────────────────

// Cancel handles POST /api/generation/tasks/:id/cancel
func (h *TaskHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid task id")
		return
	}

	userID := middleware.GetUserID(c)

	// Check ownership
	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			NotFound(c, "task not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get task")
		InternalError(c, "internal error")
		return
	}
	if t.UserID != userID {
		NotFound(c, "task not found")
		return
	}

	if task.IsTerminal(t.Status) {
		BadRequest(c, fmt.Sprintf("task already in terminal state: %s", t.Status))
		return
	}

	// Cancel in DB — the worker checks task status before processing
	// For queued tasks, the Asynq job will be a no-op when dequeued

	if err := h.repo.Cancel(c.Request.Context(), id); err != nil {
		h.log.Error().Err(err).Msg("failed to cancel task")
		InternalError(c, "internal error")
		return
	}

	_ = h.repo.AddEvent(c.Request.Context(), id, task.EventCanceled, map[string]any{
		"canceled_by": userID,
	})
	if h.billing != nil {
		if _, err := h.billing.Refund(c.Request.Context(), userID, "generation_task", id, "Task canceled"); err != nil &&
			!errors.Is(err, billing.ErrReservationNotFound) &&
			!errors.Is(err, billing.ErrReservationConfirmed) {
			h.log.Warn().Err(err).Str("task_id", id.String()).Msg("failed to refund canceled task")
		}
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "cancel_task", ResourceType: "generation_task", ResourceID: &id,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	t.Status = task.StatusCanceled
	h.publishTaskEvent(c, "task_canceled", t, map[string]any{
		"status":      task.StatusCanceled,
		"canceled_by": userID,
	})
	OK(c, t)
}

// ── Events ──────────────────────────────────────────

// Events handles GET /api/generation/tasks/:id/events
func (h *TaskHandler) Events(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid task id")
		return
	}

	userID := middleware.GetUserID(c)

	// Check ownership
	t, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			NotFound(c, "task not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get task")
		InternalError(c, "internal error")
		return
	}
	if t.UserID != userID {
		NotFound(c, "task not found")
		return
	}

	events, err := h.repo.ListEvents(c.Request.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list events")
		InternalError(c, "internal error")
		return
	}

	if events == nil {
		events = []task.TaskEvent{}
	}
	OK(c, events)
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (h *TaskHandler) publishTaskEvent(c *gin.Context, eventType string, t *task.GenerationTask, payload map[string]any) {
	if h.outbox == nil {
		return
	}
	payload["task_type"] = t.Type
	payload["provider"] = t.Provider
	payload["project_id"] = t.ProjectID
	_ = h.outbox.Publish(c.Request.Context(), eventbus.TopicGenerationTaskEvents,
		eventbus.NewEvent(eventType, "generation_task", t.ID, payload).WithUser(t.UserID))
}
