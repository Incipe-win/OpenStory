package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/moderation"
)

type ModerationHandler struct {
	service *moderation.Service
	log     zerolog.Logger
}

func NewModerationHandler(service *moderation.Service, log zerolog.Logger) *ModerationHandler {
	return &ModerationHandler{service: service, log: log}
}

func (h *ModerationHandler) List(c *gin.Context) {
	page, pageSize := Pagination(c)
	status := c.DefaultQuery("status", moderation.StatusPending)
	records, total, err := h.service.List(c.Request.Context(), status, page, pageSize)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to list moderation records")
		InternalError(c, "internal error")
		return
	}
	if records == nil {
		records = []moderation.Record{}
	}
	OKWithMeta(c, records, Meta{Page: page, PageSize: pageSize, Total: total})
}

type reviewWorkRequest struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
	Reason string `json:"reason" binding:"omitempty,max=2000"`
}

func (h *ModerationHandler) ReviewWork(c *gin.Context) {
	workID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		BadRequest(c, "invalid work id")
		return
	}
	var req reviewWorkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	record, err := h.service.ReviewWork(c.Request.Context(), workID, middleware.GetUserID(c), req.Status, req.Reason, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		if errors.Is(err, moderation.ErrRecordNotFound) {
			NotFound(c, "work not found")
			return
		}
		if errors.Is(err, moderation.ErrInvalidStatus) {
			BadRequest(c, err.Error())
			return
		}
		h.log.Error().Err(err).Msg("failed to review work")
		InternalError(c, "internal error")
		return
	}
	OK(c, record)
}
