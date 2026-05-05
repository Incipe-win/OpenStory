// Package router sets up the Gin HTTP engine and registers all routes.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/audit"
	authpkg "github.com/Incipe-win/OpenStory/internal/auth"
	"github.com/Incipe-win/OpenStory/internal/billing"
	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/http/handler"
	"github.com/Incipe-win/OpenStory/internal/http/middleware"
	"github.com/Incipe-win/OpenStory/internal/moderation"
	"github.com/Incipe-win/OpenStory/internal/project"
	"github.com/Incipe-win/OpenStory/internal/task"
	"github.com/Incipe-win/OpenStory/internal/workflow"
)

// Deps holds all dependencies needed to build the router.
type Deps struct {
	Log         zerolog.Logger
	Pool        *pgxpool.Pool
	RDB         *redis.Client
	Config      *config.Config
	AsynqClient *asynq.Client
}

// New creates and configures a new Gin engine with all routes registered.
func New(deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.RequestContext(deps.Config.Observability.ServiceName))
	r.Use(middleware.Logger(deps.Log))

	// ── Health endpoints ─────────────────────────────
	health := handler.NewHealthHandler(deps.Pool, deps.RDB, deps.Config.Server.Version)
	r.GET("/healthz", health.Healthz)
	r.GET("/readyz", health.Readyz)

	// ── Dependencies ─────────────────────────────────
	jwtSvc := authpkg.NewJWTService(
		deps.Config.JWT.Secret,
		deps.Config.JWT.AccessTokenTTL,
		deps.Config.JWT.RefreshTokenTTL,
	)
	auditLog := audit.NewLogger(deps.Pool)
	outboxWriter := eventbus.NewOutboxWriter(deps.Pool)
	authRepo := authpkg.NewPgRepository(deps.Pool)
	projRepo := project.NewPgRepository(deps.Pool)
	wfRepo := workflow.NewPgRepository(deps.Pool)
	taskRepo := task.NewPgRepository(deps.Pool)
	billingSvc := billing.NewPgService(deps.Pool, outboxWriter)
	moderationSvc := moderation.NewService(deps.Pool, outboxWriter)
	assetRepo := asset.NewCreator(deps.Pool, outboxWriter)
	assetStorage, err := asset.NewStorage(deps.Config.MinIO)
	if err != nil {
		deps.Log.Warn().Err(err).Msg("minio storage unavailable")
	}

	// ── Handlers ─────────────────────────────────────
	authH := handler.NewAuthHandler(authRepo, jwtSvc, auditLog, deps.Log)
	projH := handler.NewProjectHandler(projRepo, auditLog, outboxWriter, deps.Log)
	wfH := handler.NewWorkflowHandler(wfRepo, auditLog, deps.Log)
	taskH := handler.NewTaskHandler(taskRepo, deps.AsynqClient, auditLog, billingSvc, outboxWriter, deps.Config.Limits.TaskConcurrentLimit, deps.Log)
	assetH := handler.NewAssetHandler(assetRepo, assetStorage, projRepo, taskRepo, deps.AsynqClient, auditLog, deps.Config.Limits.TaskConcurrentLimit, deps.Log)
	billingH := handler.NewBillingHandler(billingSvc, deps.Log)
	moderationH := handler.NewModerationHandler(moderationSvc, deps.Log)

	// ── Public routes ────────────────────────────────
	api := r.Group("/api")
	api.Use(middleware.IPRateLimit(deps.RDB, deps.Config.Limits.IPRequestsPerMinute, deps.Log))
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authH.Register)
			auth.POST("/login", authH.Login)
			auth.POST("/refresh", authH.Refresh)
		}

		// Public feed
		api.GET("/feed", projH.Feed)
	}

	// ── Authenticated routes ─────────────────────────
	authed := api.Group("")
	authed.Use(middleware.Auth(jwtSvc))
	authed.Use(middleware.UserRateLimit(deps.RDB, deps.Config.Limits.UserRequestsPerMinute, deps.Log))
	{
		authed.GET("/me", authH.Me)
		authed.GET("/credits/balance", billingH.Balance)

		projects := authed.Group("/projects")
		{
			projects.POST("", projH.Create)
			projects.GET("", projH.List)
			projects.GET("/:id", projH.Get)
			projects.PATCH("/:id", projH.Update)
			projects.POST("/:id/workflows", wfH.Create)
			projects.GET("/:id/tasks", taskH.ListByProject)
			projects.GET("/:id/assets", assetH.ListByProject)
			projects.POST("/:id/compose", assetH.Compose)
		}

		assets := authed.Group("/assets")
		{
			assets.POST("/upload-url", assetH.UploadURL)
			assets.GET("/:id", assetH.Get)
		}

		compose := authed.Group("/compose")
		{
			compose.GET("/:taskId", assetH.ComposeStatus)
		}

		workflows := authed.Group("/workflows")
		{
			workflows.GET("/:id", wfH.Get)
			workflows.PUT("/:id", wfH.Update)
			workflows.POST("/:id/validate", wfH.Validate)
			workflows.POST("/:id/snapshot", wfH.Snapshot)
		}

		generation := authed.Group("/generation/tasks")
		{
			generation.POST("", taskH.Create)
			generation.GET("/:id", taskH.Get)
			generation.POST("/:id/cancel", taskH.Cancel)
			generation.GET("/:id/events", taskH.Events)
		}

		works := authed.Group("/works")
		{
			works.POST("/:id/publish", projH.PublishWork)
		}

		admin := authed.Group("/admin")
		admin.Use(middleware.RequireRole("admin"))
		{
			admin.GET("/moderation", moderationH.List)
			admin.POST("/works/:id/review", moderationH.ReviewWork)
		}
	}

	return r
}
