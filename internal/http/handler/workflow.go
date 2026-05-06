package handler

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/project"
	"github.com/Incipe-win/OpenStory/internal/workflow"
)

// WorkflowHandler handles workflow API endpoints.
type WorkflowHandler struct {
	repo     workflow.Repository
	projects project.Repository
	auditLog *audit.Logger
	log      zerolog.Logger
}

func NewWorkflowHandler(repo workflow.Repository, projects project.Repository, auditLog *audit.Logger, log zerolog.Logger) *WorkflowHandler {
	return &WorkflowHandler{repo: repo, projects: projects, auditLog: auditLog, log: log}
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
	Status      string      `json:"status"      binding:"omitempty,oneof=draft running completed failed cancelled"`
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

	status := req.Status
	if status == "" {
		status = existing.Status
	}

	wf := &workflow.Workflow{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      status,
	}

	if err := h.repo.Update(c.Request.Context(), wf, nodes, edges); err != nil {
		h.log.Error().Err(err).Msg("failed to update workflow")
		InternalError(c, "internal error")
		return
	}

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
		OK(c, gin.H{
			"valid": false,
			"error": err.Error(),
			"nodes": len(detail.Nodes),
			"edges": len(detail.Edges),
		})
		return
	}

	sorted, _ := workflow.TopologicalSort(detail.Nodes, detail.Edges)
	order := make([]gin.H, len(sorted))
	for i, n := range sorted {
		order[i] = gin.H{"id": n.ID, "type": n.Type, "name": n.Name}
	}

	OK(c, gin.H{
		"valid":           true,
		"nodes":           len(detail.Nodes),
		"edges":           len(detail.Edges),
		"execution_order": order,
		"node_schemas":    workflow.NodeSchemas,
	})
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
