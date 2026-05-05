package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/project"
)

// ProjectHandler handles project CRUD endpoints.
type ProjectHandler struct {
	repo     project.Repository
	auditLog *audit.Logger
	log      zerolog.Logger
}

// NewProjectHandler creates a new ProjectHandler.
func NewProjectHandler(repo project.Repository, auditLog *audit.Logger, log zerolog.Logger) *ProjectHandler {
	return &ProjectHandler{repo: repo, auditLog: auditLog, log: log}
}

type createProjectRequest struct {
	Name        string `json:"name"        binding:"required,min=1,max=255"`
	Description string `json:"description" binding:"max=5000"`
}

// Create handles POST /api/projects.
func (h *ProjectHandler) Create(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	p := &project.Project{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.repo.CreateProject(c.Request.Context(), p); err != nil {
		h.log.Error().Err(err).Msg("failed to create project")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "create_project", ResourceType: "project", ResourceID: &p.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	Created(c, p)
}

// List handles GET /api/projects.
func (h *ProjectHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	page, pageSize := Pagination(c)

	projects, total, err := h.repo.ListProjects(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list projects")
		InternalError(c, "internal error")
		return
	}

	if projects == nil {
		projects = []project.Project{}
	}
	OKWithMeta(c, projects, Meta{Page: page, PageSize: pageSize, Total: total})
}

// Get handles GET /api/projects/:id.
func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}

	p, err := h.repo.GetProject(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			NotFound(c, "project not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get project")
		InternalError(c, "internal error")
		return
	}

	// Only the owner can see their project
	userID := middleware.GetUserID(c)
	if p.UserID != userID {
		NotFound(c, "project not found")
		return
	}

	OK(c, p)
}

type updateProjectRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	Status      *string `json:"status"      binding:"omitempty,oneof=draft active archived"`
}

// Update handles PATCH /api/projects/:id.
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}

	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.repo.UpdateProject(c.Request.Context(), id, userID, req.Name, req.Description, req.Status); err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			NotFound(c, "project not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to update project")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "update_project", ResourceType: "project", ResourceID: &id,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: req,
	})

	p, _ := h.repo.GetProject(c.Request.Context(), id)
	OK(c, p)
}

// PublishWork handles POST /api/works/:id/publish.
func (h *ProjectHandler) PublishWork(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid work id")
		return
	}

	userID := middleware.GetUserID(c)
	if err := h.repo.PublishWork(c.Request.Context(), id, userID); err != nil {
		if errors.Is(err, project.ErrWorkNotFound) {
			NotFound(c, "work not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to publish work")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "publish_work", ResourceType: "work", ResourceID: &id,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	work, _ := h.repo.GetWork(c.Request.Context(), id)
	OK(c, work)
}

// Feed handles GET /api/feed (public, no auth required).
func (h *ProjectHandler) Feed(c *gin.Context) {
	page, pageSize := Pagination(c)

	works, total, err := h.repo.ListPublishedWorks(c.Request.Context(), page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list feed")
		InternalError(c, "internal error")
		return
	}

	if works == nil {
		works = []project.Work{}
	}
	OKWithMeta(c, works, Meta{Page: page, PageSize: pageSize, Total: total})
}
