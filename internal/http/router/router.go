// Package router sets up the Gin HTTP engine and registers all routes.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/http/handler"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
)

// New creates and configures a new Gin engine with all routes registered.
func New(log zerolog.Logger, pool *pgxpool.Pool, rdb *redis.Client, version string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))

	// Health endpoints (outside API versioning)
	health := handler.NewHealthHandler(pool, rdb, version)
	r.GET("/healthz", health.Healthz)
	r.GET("/readyz", health.Readyz)

	// API v1 group — future routes go here
	// v1 := r.Group("/api/v1")
	// {
	// }

	return r
}
