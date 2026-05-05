package handler

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/audit"
	"github.com/Incipe-win/OpenStory/internal/auth"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	repo     auth.Repository
	jwt      *auth.JWTService
	auditLog *audit.Logger
	log      zerolog.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(repo auth.Repository, jwt *auth.JWTService, auditLog *audit.Logger, log zerolog.Logger) *AuthHandler {
	return &AuthHandler{repo: repo, jwt: jwt, auditLog: auditLog, log: log}
}

type registerRequest struct {
	Email       string `json:"email"        binding:"required,email"`
	Username    string `json:"username"     binding:"required,min=3,max=50"`
	Password    string `json:"password"     binding:"required,min=8,max=72"`
	DisplayName string `json:"display_name" binding:"max=100"`
}

// Register creates a new user account.
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to hash password")
		InternalError(c, "internal error")
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user := &auth.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: hash,
		DisplayName:  displayName,
		Role:         "user",
	}

	if err := h.repo.CreateUser(c.Request.Context(), user); err != nil {
		if errors.Is(err, auth.ErrEmailExists) {
			Conflict(c, "email already registered")
			return
		}
		if errors.Is(err, auth.ErrUsernameExists) {
			Conflict(c, "username already taken")
			return
		}
		h.log.Error().Err(err).Msg("failed to create user")
		InternalError(c, "internal error")
		return
	}

	// Audit log
	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &user.ID, Action: "register", ResourceType: "user", ResourceID: &user.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	Created(c, user)
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a token pair.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	user, err := h.repo.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			Unauthorized(c, "invalid credentials")
			return
		}
		h.log.Error().Err(err).Msg("failed to get user")
		InternalError(c, "internal error")
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		Unauthorized(c, "invalid credentials")
		return
	}

	if user.Status != "active" {
		Forbidden(c, "account is not active")
		return
	}

	tokenPair, refreshHash, err := h.jwt.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate tokens")
		InternalError(c, "internal error")
		return
	}

	// Store refresh token
	rt := &auth.RefreshToken{
		UserID:     user.ID,
		TokenHash:  refreshHash,
		DeviceInfo: c.GetHeader("User-Agent"),
		IPAddress:  c.ClientIP(),
		ExpiresAt:  time.Now().Add(h.jwt.RefreshTokenTTL()),
	}
	if err := h.repo.CreateRefreshToken(c.Request.Context(), rt); err != nil {
		h.log.Error().Err(err).Msg("failed to store refresh token")
		InternalError(c, "internal error")
		return
	}

	// Audit log
	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &user.ID, Action: "login", ResourceType: "user", ResourceID: &user.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	OK(c, tokenPair)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh exchanges a valid refresh token for a new token pair.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	hash := auth.HashToken(req.RefreshToken)
	rt, err := h.repo.GetRefreshTokenByHash(c.Request.Context(), hash)
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) || errors.Is(err, auth.ErrTokenExpired) || errors.Is(err, auth.ErrTokenRevoked) {
			Unauthorized(c, "invalid or expired refresh token")
			return
		}
		h.log.Error().Err(err).Msg("failed to get refresh token")
		InternalError(c, "internal error")
		return
	}

	// Revoke old token
	_ = h.repo.RevokeRefreshToken(c.Request.Context(), rt.ID)

	// Get user for new claims
	user, err := h.repo.GetUserByID(c.Request.Context(), rt.UserID)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to get user for refresh")
		InternalError(c, "internal error")
		return
	}

	tokenPair, newRefreshHash, err := h.jwt.GenerateTokenPair(user.ID, user.Username, user.Role)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate tokens")
		InternalError(c, "internal error")
		return
	}

	// Store new refresh token
	newRT := &auth.RefreshToken{
		UserID:     user.ID,
		TokenHash:  newRefreshHash,
		DeviceInfo: c.GetHeader("User-Agent"),
		IPAddress:  c.ClientIP(),
		ExpiresAt:  time.Now().Add(h.jwt.RefreshTokenTTL()),
	}
	if err := h.repo.CreateRefreshToken(c.Request.Context(), newRT); err != nil {
		h.log.Error().Err(err).Msg("failed to store new refresh token")
		InternalError(c, "internal error")
		return
	}

	_ = h.auditLog.Log(c.Request.Context(), audit.Entry{
		UserID: &user.ID, Action: "refresh_token", ResourceType: "user", ResourceID: &user.ID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})

	OK(c, tokenPair)
}

// Me returns the authenticated user's profile.
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			NotFound(c, "user not found")
			return
		}
		h.log.Error().Err(err).Msg("failed to get user")
		InternalError(c, "internal error")
		return
	}
	OK(c, user)
}
