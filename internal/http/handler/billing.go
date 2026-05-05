package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
)

type BillingHandler struct {
	billing billing.Service
	log     zerolog.Logger
}

func NewBillingHandler(billingSvc billing.Service, log zerolog.Logger) *BillingHandler {
	return &BillingHandler{billing: billingSvc, log: log}
}

func (h *BillingHandler) Balance(c *gin.Context) {
	account, err := h.billing.GetAccount(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, billing.ErrAccountNotFound) {
			NotFound(c, "credit account not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get credit account")
		InternalError(c, "internal error")
		return
	}
	OK(c, account)
}
