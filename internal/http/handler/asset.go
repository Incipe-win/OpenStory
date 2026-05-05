package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/project"
	"github.com/Incipe-win/OpenStory/internal/task"
)

type AssetHandler struct {
	assets      *asset.Creator
	storage     *asset.Storage
	projects    project.Repository
	tasks       task.Repository
	asynqClient *asynq.Client
	auditLog    *audit.Logger
	taskLimit   int
	log         zerolog.Logger
}

func NewAssetHandler(assets *asset.Creator, storage *asset.Storage, projects project.Repository, tasks task.Repository, asynqClient *asynq.Client, auditLog *audit.Logger, taskLimit int, log zerolog.Logger) *AssetHandler {
	return &AssetHandler{assets: assets, storage: storage, projects: projects, tasks: tasks, asynqClient: asynqClient, auditLog: auditLog, taskLimit: taskLimit, log: log}
}

type uploadURLRequest struct {
	ProjectID  uuid.UUID `json:"project_id" binding:"required"`
	Type       string    `json:"type" binding:"required"`
	Name       string    `json:"name" binding:"required,max=255"`
	MimeType   string    `json:"mime_type" binding:"required"`
	SizeBytes  int64     `json:"size_bytes" binding:"omitempty,min=0"`
	DurationMs *int      `json:"duration_ms"`
	Width      *int      `json:"width"`
	Height     *int      `json:"height"`
	Checksum   string    `json:"checksum" binding:"omitempty,max=128"`
}

func (h *AssetHandler) UploadURL(c *gin.Context) {
	if h.storage == nil {
		InternalError(c, "object storage is not configured")
		return
	}
	var req uploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	if !h.canAccessProject(c, req.ProjectID, userID) {
		return
	}

	a := &asset.Asset{
		ID:            uuid.New(),
		UserID:        userID,
		ProjectID:     &req.ProjectID,
		Type:          req.Type,
		Name:          req.Name,
		MimeType:      req.MimeType,
		SizeBytes:     req.SizeBytes,
		DurationMs:    req.DurationMs,
		Width:         req.Width,
		Height:        req.Height,
		Checksum:      req.Checksum,
		StorageBucket: h.storage.Bucket(),
		MetadataJSON:  json.RawMessage(`{"upload_status":"pending"}`),
	}
	a.StorageKey = asset.BuildStorageKey(req.ProjectID.String(), a.ID.String(), req.Name)
	a.URL = asset.ObjectURL(a.StorageBucket, a.StorageKey)

	url, err := h.storage.PresignedPutURL(c.Request.Context(), a.StorageKey, req.MimeType, 15*time.Minute)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create presigned upload url")
		InternalError(c, "internal error")
		return
	}
	if err := h.assets.Create(c.Request.Context(), a); err != nil {
		h.log.Error().Err(err).Msg("failed to create asset upload placeholder")
		InternalError(c, "internal error")
		return
	}
	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "create_asset_upload_url", ResourceType: "asset", ResourceID: &a.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"project_id": req.ProjectID, "type": req.Type, "mime_type": req.MimeType, "size_bytes": req.SizeBytes},
	})

	Created(c, gin.H{
		"asset":        a,
		"upload_url":   url.String(),
		"expires_in":   900,
		"method":       "PUT",
		"content_type": req.MimeType,
	})
}

func (h *AssetHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid asset id")
		return
	}
	a, err := h.assets.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, asset.ErrAssetNotFound) {
			NotFound(c, "asset not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get asset")
		InternalError(c, "internal error")
		return
	}
	if a.UserID != middleware.GetUserID(c) {
		NotFound(c, "asset not found")
		return
	}
	OK(c, a)
}

func (h *AssetHandler) ListByProject(c *gin.Context) {
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
	assets, total, err := h.assets.ListByProject(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list assets")
		InternalError(c, "internal error")
		return
	}
	if assets == nil {
		assets = []asset.Asset{}
	}
	OKWithMeta(c, assets, Meta{Page: page, PageSize: pageSize, Total: total})
}

type composeRequest struct {
	ImageAssetIDs      []uuid.UUID `json:"image_asset_ids" binding:"required,min=1"`
	Subtitle           string      `json:"subtitle"`
	SubtitleFormat     string      `json:"subtitle_format" binding:"omitempty,oneof=srt vtt"`
	DurationPerImageMs int         `json:"duration_per_image_ms" binding:"omitempty,min=250"`
	Width              int         `json:"width" binding:"omitempty,min=16"`
	Height             int         `json:"height" binding:"omitempty,min=16"`
	FPS                int         `json:"fps" binding:"omitempty,min=1,max=120"`
	Title              string      `json:"title" binding:"omitempty,max=255"`
	IdempotencyKey     string      `json:"idempotency_key" binding:"omitempty,max=255"`
}

func (h *AssetHandler) Compose(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid project id")
		return
	}
	userID := middleware.GetUserID(c)
	if !h.canAccessProject(c, projectID, userID) {
		return
	}

	var req composeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
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
	inputJSON, _ := json.Marshal(req)
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("compose:%s:%d", projectID, time.Now().UnixNano())
	}
	t := &task.GenerationTask{
		UserID:         userID,
		ProjectID:      projectID,
		Type:           task.TypeCompose,
		Provider:       "ffmpeg",
		IdempotencyKey: idempotencyKey,
		InputJSON:      inputJSON,
	}
	if err := h.tasks.Create(c.Request.Context(), t); err != nil {
		if errors.Is(err, task.ErrIdempotencyConflict) {
			existing, _ := h.tasks.GetByIdempotencyKey(c.Request.Context(), idempotencyKey)
			if existing != nil {
				OK(c, existing)
				return
			}
			Conflict(c, "task with this idempotency key already exists")
			return
		}
		h.log.Error().Err(err).Msg("failed to create compose task")
		InternalError(c, "internal error")
		return
	}
	_ = h.tasks.AddEvent(c.Request.Context(), t.ID, task.EventCreated, map[string]any{"type": t.Type, "provider": t.Provider})
	asynqTask, err := task.NewAsynqTaskWithContext(c.Request.Context(), t.ID)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create compose asynq task")
		InternalError(c, "internal error")
		return
	}
	info, err := h.asynqClient.Enqueue(asynqTask)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to enqueue compose task")
		InternalError(c, "failed to enqueue compose task")
		return
	}
	_ = h.tasks.UpdateStatus(c.Request.Context(), t.ID, task.StatusQueued, nil, nil)
	_ = h.tasks.AddEvent(c.Request.Context(), t.ID, task.EventQueued, map[string]any{"asynq_id": info.ID, "queue": info.Queue})
	t.Status = task.StatusQueued
	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &userID, Action: "create_compose_task", ResourceType: "generation_task", ResourceID: &t.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		NewValues: map[string]any{"project_id": projectID, "image_asset_ids": req.ImageAssetIDs},
	})
	Created(c, t)
}

func (h *AssetHandler) ComposeStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		BadRequest(c, "invalid compose task id")
		return
	}
	t, err := h.tasks.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			NotFound(c, "compose task not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get compose task")
		InternalError(c, "internal error")
		return
	}
	if t.UserID != middleware.GetUserID(c) || t.Type != task.TypeCompose {
		NotFound(c, "compose task not found")
		return
	}
	OK(c, t)
}

func (h *AssetHandler) canAccessProject(c *gin.Context, projectID, userID uuid.UUID) bool {
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
