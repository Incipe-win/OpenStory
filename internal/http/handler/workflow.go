package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/project"
	"github.com/Incipe-win/OpenStory/internal/task"
	"github.com/Incipe-win/OpenStory/internal/workflow"
)

// WorkflowHandler handles workflow API endpoints.
type WorkflowHandler struct {
	repo        workflow.Repository
	projects    project.Repository
	tasks       task.Repository
	asynqClient *asynq.Client
	auditLog    *audit.Logger
	outbox      eventbus.EventBus
	taskLimit   int
	log         zerolog.Logger
}

func NewWorkflowHandler(repo workflow.Repository, projects project.Repository, tasks task.Repository, asynqClient *asynq.Client, auditLog *audit.Logger, outbox eventbus.EventBus, taskLimit int, log zerolog.Logger) *WorkflowHandler {
	return &WorkflowHandler{repo: repo, projects: projects, tasks: tasks, asynqClient: asynqClient, auditLog: auditLog, outbox: outbox, taskLimit: taskLimit, log: log}
}

// ── Create ──────────────────────────────────────────

type createWorkflowRequest struct {
	Name        string `json:"name"        binding:"required,min=1,max=255"`
	Description string `json:"description" binding:"max=5000"`
}

// Create handles POST /api/projects/:id/workflows
func (h *WorkflowHandler) Create(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}

	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if !h.canAccessProject(c, projectID, userID) {
		return
	}

	wf := &workflow.Workflow{
		ProjectID:   projectID,
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.repo.Create(c.Request.Context(), wf); err != nil {
		h.log.Error().Err(err).Msg("failed to create workflow")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "create_workflow", ResourceType: "workflow", ResourceID: &wf.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	Created(c, wf)
}

// ── ListByProject ───────────────────────────────────

// ListByProject handles GET /api/projects/:id/workflows
func (h *WorkflowHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}

	userID := middleware.GetUserID(c)
	if !h.canAccessProject(c, projectID, userID) {
		return
	}

	page, pageSize := Pagination(c)
	workflows, total, err := h.repo.ListByProject(c.Request.Context(), projectID, userID, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list workflows")
		InternalError(c, "internal error")
		return
	}

	if workflows == nil {
		workflows = []workflow.Workflow{}
	}
	OKWithMeta(c, workflows, Meta{Page: page, PageSize: pageSize, Total: total})
}

// ── Get ─────────────────────────────────────────────

// Get handles GET /api/workflows/:id
func (h *WorkflowHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	detail, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow")
		InternalError(c, "internal error")
		return
	}

	// Check ownership
	userID := middleware.GetUserID(c)
	if detail.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	OK(c, detail)
}

// ── Update ──────────────────────────────────────────

type nodeInput struct {
	ID        uuid.UUID `json:"id"   binding:"required"`
	Type      string    `json:"type" binding:"required"`
	Name      string    `json:"name"`
	Config    any       `json:"config"`
	PositionX float64   `json:"position_x"`
	PositionY float64   `json:"position_y"`
}

type edgeInput struct {
	ID           uuid.UUID `json:"id"`
	SourceNodeID uuid.UUID `json:"source_node_id" binding:"required"`
	TargetNodeID uuid.UUID `json:"target_node_id" binding:"required"`
	SourceHandle string    `json:"source_handle"`
	TargetHandle string    `json:"target_handle"`
}

type updateWorkflowRequest struct {
	Name        string      `json:"name"        binding:"required,min=1,max=255"`
	Description string      `json:"description" binding:"max=5000"`
	Status      string      `json:"status"      binding:"omitempty,oneof=draft validated published"`
	Nodes       []nodeInput `json:"nodes"       binding:"required"`
	Edges       []edgeInput `json:"edges"`
}

// Update handles PUT /api/workflows/:id
func (h *WorkflowHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	var req updateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Convert to domain models
	nodes := make([]workflow.Node, len(req.Nodes))
	for i, n := range req.Nodes {
		if !workflow.ValidNodeType(n.Type) {
			BadRequest(c, "invalid node type: "+n.Type)
			return
		}
		nodes[i] = workflow.Node{
			ID:        n.ID,
			Type:      workflow.NodeType(n.Type),
			Name:      n.Name,
			PositionX: n.PositionX,
			PositionY: n.PositionY,
		}
		if n.Config != nil {
			configJSON, err := json.Marshal(n.Config)
			if err != nil {
				BadRequest(c, "invalid node config")
				return
			}
			nodes[i].ConfigJSON = configJSON
		}
	}

	edges := make([]workflow.Edge, len(req.Edges))
	for i, e := range req.Edges {
		edges[i] = workflow.Edge{
			ID:           e.ID,
			SourceNodeID: e.SourceNodeID,
			TargetNodeID: e.TargetNodeID,
			SourceHandle: e.SourceHandle,
			TargetHandle: e.TargetHandle,
		}
	}

	// Validate DAG before persisting
	if err := workflow.ValidateDAG(nodes, edges); err != nil {
		BadRequest(c, "invalid DAG: "+err.Error())
		return
	}

	// Check ownership
	userID := middleware.GetUserID(c)
	existing, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow for update")
		InternalError(c, "internal error")
		return
	}
	if existing.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	wf := &workflow.Workflow{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      workflow.StatusDraft,
	}

	if err := h.repo.Update(c.Request.Context(), wf, nodes, edges); err != nil {
		h.log.Error().Err(err).Msg("failed to update workflow")
		InternalError(c, "internal error")
		return
	}
	h.refreshProjectStatus(c, existing.ProjectID)

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "update_workflow", ResourceType: "workflow", ResourceID: &id,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"nodes": len(nodes), "edges": len(edges)},
	})

	// Return updated detail
	detail, _ := h.repo.GetDetail(c.Request.Context(), id)
	OK(c, detail)
}

// ── Validate ────────────────────────────────────────

// Validate handles POST /api/workflows/:id/validate
func (h *WorkflowHandler) Validate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	userID := middleware.GetUserID(c)
	detail, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow for validation")
		InternalError(c, "internal error")
		return
	}
	if detail.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	if err := workflow.ValidateDAG(detail.Nodes, detail.Edges); err != nil {
		if statusErr := h.repo.UpdateStatus(c.Request.Context(), id, workflow.StatusDraft); statusErr != nil {
			h.log.Warn().Err(statusErr).Str("workflow_id", id.String()).Msg("failed to reset invalid workflow status")
		}
		h.refreshProjectStatus(c, detail.ProjectID)
		OK(c, gin.H{
			"valid":      false,
			"error":      err.Error(),
			"project_id": detail.ProjectID,
			"nodes":      len(detail.Nodes),
			"edges":      len(detail.Edges),
		})
		return
	}

	sorted, _ := workflow.TopologicalSort(detail.Nodes, detail.Edges)
	order := make([]gin.H, len(sorted))
	for i, n := range sorted {
		order[i] = gin.H{"id": n.ID, "type": n.Type, "name": n.Name}
	}

	nextStatus := workflow.StatusValidated
	if detail.Status == workflow.StatusPublished {
		nextStatus = workflow.StatusPublished
	}
	if err := h.repo.UpdateStatus(c.Request.Context(), id, nextStatus); err != nil {
		h.log.Error().Err(err).Msg("failed to mark workflow validated")
		InternalError(c, "internal error")
		return
	}
	h.refreshProjectStatus(c, detail.ProjectID)

	OK(c, gin.H{
		"valid":           true,
		"status":          nextStatus,
		"project_id":      detail.ProjectID,
		"nodes":           len(detail.Nodes),
		"edges":           len(detail.Edges),
		"execution_order": order,
		"node_schemas":    workflow.NodeSchemas,
	})
}

// ── Publish ─────────────────────────────────────────

// Publish handles POST /api/workflows/:id/publish
func (h *WorkflowHandler) Publish(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	userID := middleware.GetUserID(c)
	detail, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow for publish")
		InternalError(c, "internal error")
		return
	}
	if detail.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	if detail.Status != workflow.StatusValidated && detail.Status != workflow.StatusPublished {
		BadRequest(c, "Please validate the workflow first")
		return
	}

	if err := workflow.ValidateDAG(detail.Nodes, detail.Edges); err != nil {
		if statusErr := h.repo.UpdateStatus(c.Request.Context(), id, workflow.StatusDraft); statusErr != nil {
			h.log.Warn().Err(statusErr).Str("workflow_id", id.String()).Msg("failed to reset invalid workflow status")
		}
		h.refreshProjectStatus(c, detail.ProjectID)
		BadRequest(c, "invalid DAG: "+err.Error())
		return
	}

	if err := h.repo.UpdateStatus(c.Request.Context(), id, workflow.StatusPublished); err != nil {
		h.log.Error().Err(err).Msg("failed to publish workflow")
		InternalError(c, "internal error")
		return
	}
	h.refreshProjectStatus(c, detail.ProjectID)

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "publish_workflow", ResourceType: "workflow", ResourceID: &id,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"status": workflow.StatusPublished},
	})

	detail.Status = workflow.StatusPublished
	OK(c, detail)
}

// ── Run ─────────────────────────────────────────────

type runWorkflowRequest struct {
	Provider       string `json:"provider"`
	IdempotencyKey string `json:"idempotency_key"`
	Input          any    `json:"input"`
}

// Run handles POST /api/workflows/:id/run
func (h *WorkflowHandler) Run(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	var req runWorkflowRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			BadRequest(c, err.Error())
			return
		}
	}

	userID := middleware.GetUserID(c)
	detail, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow for run")
		InternalError(c, "internal error")
		return
	}
	if detail.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	if detail.Status != workflow.StatusPublished {
		BadRequest(c, "Please publish the workflow first")
		return
	}

	if err := workflow.ValidateDAG(detail.Nodes, detail.Edges); err != nil {
		if statusErr := h.repo.UpdateStatus(c.Request.Context(), id, workflow.StatusDraft); statusErr != nil {
			h.log.Warn().Err(statusErr).Str("workflow_id", id.String()).Msg("failed to reset invalid workflow status")
		}
		h.refreshProjectStatus(c, detail.ProjectID)
		BadRequest(c, "invalid DAG: "+err.Error())
		return
	}

	if h.tasks == nil || h.asynqClient == nil {
		InternalError(c, "workflow runner is not configured")
		return
	}

	if h.taskLimit > 0 {
		active, err := h.tasks.CountActiveByUser(c.Request.Context(), userID)
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

	sorted, _ := workflow.TopologicalSort(detail.Nodes, detail.Edges)
	inputJSON, err := json.Marshal(buildWorkflowRunInput(detail, sorted, req.Input))
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal workflow run input")
		InternalError(c, "internal error")
		return
	}

	providerName := strings.TrimSpace(req.Provider)
	if providerName == "" {
		providerName = "mock"
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("workflow-run:%s:%s", id, uuid.NewString())
	}

	runTask := &task.GenerationTask{
		UserID:         userID,
		ProjectID:      detail.ProjectID,
		WorkflowID:     &detail.ID,
		Type:           task.TypeCreativePipeline,
		Provider:       providerName,
		IdempotencyKey: idempotencyKey,
		InputJSON:      inputJSON,
	}

	if err := h.tasks.Create(c.Request.Context(), runTask); err != nil {
		if errors.Is(err, task.ErrIdempotencyConflict) {
			existing, _ := h.tasks.GetByIdempotencyKey(c.Request.Context(), idempotencyKey)
			if existing != nil && existing.UserID == userID {
				OK(c, gin.H{"task_id": existing.ID, "task": existing})
				return
			}
			Conflict(c, "task with this idempotency key already exists")
			return
		}
		h.log.Error().Err(err).Msg("failed to create workflow run task")
		InternalError(c, "internal error")
		return
	}

	_ = h.tasks.AddEvent(c.Request.Context(), runTask.ID, task.EventCreated, map[string]any{
		"type":        runTask.Type,
		"provider":    runTask.Provider,
		"workflow_id": detail.ID,
	})
	h.publishTaskEvent(c, "task_created", runTask, map[string]any{
		"status":      task.StatusPending,
		"workflow_id": detail.ID,
	})

	asynqTask, err := task.NewAsynqTaskWithContext(c.Request.Context(), runTask.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create workflow run asynq task")
		InternalError(c, "internal error")
		return
	}

	info, err := h.asynqClient.Enqueue(asynqTask)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to enqueue workflow run task")
		InternalError(c, "failed to enqueue workflow run task")
		return
	}

	_ = h.tasks.UpdateStatus(c.Request.Context(), runTask.ID, task.StatusQueued, nil, nil)
	_ = h.tasks.AddEvent(c.Request.Context(), runTask.ID, task.EventQueued, map[string]any{
		"asynq_id": info.ID,
		"queue":    info.Queue,
	})
	runTask.Status = task.StatusQueued
	h.publishTaskEvent(c, "task_queued", runTask, map[string]any{
		"status":   task.StatusQueued,
		"asynq_id": info.ID,
		"queue":    info.Queue,
	})

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "run_workflow", ResourceType: "generation_task", ResourceID: &runTask.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"workflow_id": detail.ID, "provider": runTask.Provider},
	})

	Created(c, gin.H{"task_id": runTask.ID, "task": runTask})
}

// ── Snapshot ────────────────────────────────────────

// Snapshot handles POST /api/workflows/:id/snapshot
func (h *WorkflowHandler) Snapshot(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid workflow id")
		return
	}

	userID := middleware.GetUserID(c)

	// Check ownership
	detail, err := h.repo.GetDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowNotFound) {
			NotFound(c, "workflow not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get workflow for snapshot")
		InternalError(c, "internal error")
		return
	}
	if detail.UserID != userID {
		NotFound(c, "workflow not found")
		return
	}

	version, err := h.repo.CreateSnapshot(c.Request.Context(), id, userID)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create snapshot")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "snapshot_workflow", ResourceType: "workflow_version", ResourceID: &version.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"version": version.VersionNum},
	})

	Created(c, version)
}

func (h *WorkflowHandler) canAccessProject(c *gin.Context, projectID, userID uuid.UUID) bool {
	p, err := h.projects.GetProject(c.Request.Context(), projectID)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			NotFound(c, "project not found")
			return false
		}
		h.log.Error().Err(err).Msg("failed to get project")
		InternalError(c, "internal error")
		return false
	}
	if p.UserID != userID {
		NotFound(c, "project not found")
		return false
	}
	return true
}

func (h *WorkflowHandler) refreshProjectStatus(c *gin.Context, projectID uuid.UUID) {
	if h.projects == nil {
		return
	}
	if err := h.projects.RefreshStatusFromPublishedWorkflows(c.Request.Context(), projectID); err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			return
		}
		h.log.Warn().Err(err).Str("project_id", projectID.String()).Msg("failed to refresh project status")
	}
}

func buildWorkflowRunInput(detail *workflow.WorkflowDetail, sorted []workflow.Node, userInput any) map[string]any {
	nodes := make([]map[string]any, len(detail.Nodes))
	for i, n := range detail.Nodes {
		nodes[i] = serializeWorkflowNode(n)
	}

	edges := make([]map[string]any, len(detail.Edges))
	for i, e := range detail.Edges {
		edges[i] = map[string]any{
			"id":             e.ID,
			"source_node_id": e.SourceNodeID,
			"target_node_id": e.TargetNodeID,
			"source_handle":  e.SourceHandle,
			"target_handle":  e.TargetHandle,
		}
	}

	executionOrder := make([]map[string]any, len(sorted))
	for i, n := range sorted {
		executionOrder[i] = map[string]any{
			"id":   n.ID,
			"type": n.Type,
			"name": n.Name,
		}
	}

	if userInput == nil {
		userInput = map[string]any{}
	}

	return map[string]any{
		"prompt":          workflowRunPrompt(detail, sorted, userInput),
		"input":           userInput,
		"workflow_id":     detail.ID,
		"workflow_name":   detail.Name,
		"description":     detail.Description,
		"nodes":           nodes,
		"edges":           edges,
		"execution_order": executionOrder,
	}
}

func serializeWorkflowNode(n workflow.Node) map[string]any {
	var config any = map[string]any{}
	if len(n.ConfigJSON) > 0 {
		if err := json.Unmarshal(n.ConfigJSON, &config); err != nil {
			config = map[string]any{}
		}
	}

	return map[string]any{
		"id":         n.ID,
		"type":       n.Type,
		"name":       n.Name,
		"config":     config,
		"position_x": n.PositionX,
		"position_y": n.PositionY,
	}
}

func workflowRunPrompt(detail *workflow.WorkflowDetail, sorted []workflow.Node, userInput any) string {
	if prompt := stringField(userInput, "prompt", "idea", "text", "description", "brief", "topic"); prompt != "" {
		return prompt
	}

	for _, n := range sorted {
		var config any
		if len(n.ConfigJSON) == 0 {
			continue
		}
		if err := json.Unmarshal(n.ConfigJSON, &config); err != nil {
			continue
		}
		if prompt := stringField(config, "prompt", "idea", "text", "description", "brief", "topic"); prompt != "" {
			return prompt
		}
	}

	if strings.TrimSpace(detail.Description) != "" {
		return detail.Description
	}
	if strings.TrimSpace(detail.Name) != "" {
		return detail.Name
	}
	return "Run this OpenStory workflow and generate a concise AI short-video plan."
}

func stringField(value any, keys ...string) string {
	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if s, ok := obj[key].(string); ok {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func (h *WorkflowHandler) publishTaskEvent(c *gin.Context, eventType string, t *task.GenerationTask, payload map[string]any) {
	if h.outbox == nil {
		return
	}
	payload["task_type"] = t.Type
	payload["provider"] = t.Provider
	payload["project_id"] = t.ProjectID
	_ = h.outbox.Publish(c.Request.Context(), eventbus.TopicGenerationTaskEvents,
		eventbus.NewEvent(eventType, "generation_task", t.ID, payload).WithUser(t.UserID))
}
