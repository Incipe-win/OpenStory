// Package handler contains HTTP request handlers.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler provides health check endpoints.
type HealthHandler struct {
	pool    *pgxpool.Pool
	rdb     *redis.Client
	version string
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, rdb *redis.Client, version string) *HealthHandler {
	return &HealthHandler{pool: pool, rdb: rdb, version: version}
}

// Healthz is a lightweight liveness probe.
func (h *HealthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": h.version,
	})
}

// Readyz is a deep readiness probe that checks all dependencies.
func (h *HealthHandler) Readyz(c *gin.Context) {
	ctx := c.Request.Context()
	checks := gin.H{}

	// Check PostgreSQL
	if h.pool != nil {
		if err := h.pool.Ping(ctx); err != nil {
			checks["postgres"] = err.Error()
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"checks": checks,
			})
			return
		}
		checks["postgres"] = "ok"
	}

	// Check Redis
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = err.Error()
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "error",
				"checks": checks,
			})
			return
		}
		checks["redis"] = "ok"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": h.version,
		"checks":  checks,
	})
}
