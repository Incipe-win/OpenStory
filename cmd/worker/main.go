package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/db"
	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/Incipe-win/OpenStory/internal/provider"
	"github.com/Incipe-win/OpenStory/internal/task"
)

func main() {
	// ── Load config ──────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ── Logger ───────────────────────────────────────
	log := observability.NewLogger(cfg.Server.Env)
	log.Info().Msg("starting OpenStory worker")

	// ── Database ─────────────────────────────────────
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	// ── Provider Registry ────────────────────────────
	registry := provider.NewRegistry()
	registry.Register(provider.NewMockProvider())
	log.Info().Msg("mock provider registered")

	// ── Task Processor ───────────────────────────────
	taskRepo := task.NewPgRepository(pool)
	processor := task.NewProcessor(taskRepo, registry, log)

	// ── Asynq Server ─────────────────────────────────
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"generation": 6,
				"default":    3,
				"low":        1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
				log.Error().
					Str("type", t.Type()).
					Err(err).
					Msg("task processing error")
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(task.AsynqTaskType, processor.ProcessTask)

	// ── Start worker ─────────────────────────────────
	go func() {
		log.Info().Msg("asynq worker starting...")
		if err := srv.Run(mux); err != nil {
			log.Fatal().Err(err).Msg("asynq server failed")
		}
	}()

	// ── Graceful Shutdown ────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down worker...")
	srv.Shutdown()
	log.Info().Msg("worker stopped")
}
