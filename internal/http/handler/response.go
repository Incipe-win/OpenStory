package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── Unified Error Codes ─────────────────────────────

const (
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeConflict     = "CONFLICT"
	CodeInternal     = "INTERNAL_ERROR"
)

// errorBody represents a unified error response.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ── Error Response Helpers ──────────────────────────

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": errorBody{Code: code, Message: message}})
}

// BadRequest sends a 400 validation error.
func BadRequest(c *gin.Context, msg string) {
	respondError(c, http.StatusBadRequest, CodeValidation, msg)
}

// Unauthorized sends a 401 error.
func Unauthorized(c *gin.Context, msg string) {
	respondError(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}

// Forbidden sends a 403 error.
func Forbidden(c *gin.Context, msg string) {
	respondError(c, http.StatusForbidden, CodeForbidden, msg)
}

// NotFound sends a 404 error.
func NotFound(c *gin.Context, msg string) {
	respondError(c, http.StatusNotFound, CodeNotFound, msg)
}

// Conflict sends a 409 error.
func Conflict(c *gin.Context, msg string) {
	respondError(c, http.StatusConflict, CodeConflict, msg)
}

// InternalError sends a 500 error.
func InternalError(c *gin.Context, msg string) {
	respondError(c, http.StatusInternalServerError, CodeInternal, msg)
}

// ── Success Response Helpers ────────────────────────

// Meta represents pagination metadata.
type Meta struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// OK sends a 200 response with data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// OKWithMeta sends a 200 response with data and pagination.
func OKWithMeta(c *gin.Context, data any, meta Meta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

// Created sends a 201 response with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

// ── Pagination Helper ───────────────────────────────

// Pagination extracts page and page_size from query params with defaults.
func Pagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}
