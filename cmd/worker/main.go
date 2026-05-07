package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"github.com/Incipe-win/OpenStory/internal/asset"
	"github.com/Incipe-win/OpenStory/internal/billing"
	composepkg "github.com/Incipe-win/OpenStory/internal/compose"
	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/db"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
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
	traceShutdown, err := observability.InitTracer(context.Background(), observability.TraceConfig{
		ServiceName:  cfg.Observability.ServiceName + "-worker",
		Enabled:      cfg.Observability.TracingEnabled,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		OTLPInsecure: cfg.Observability.OTLPInsecure,
	})
	if err != nil {
		log.Warn().Err(err).Msg("OpenTelemetry tracing disabled")
		traceShutdown = func(context.Context) error { return nil }
	}
	defer traceShutdown(context.Background()) //nolint:errcheck
	diagSrv := observability.StartDiagnosticsServer(cfg.Observability.DiagnosticsAddr, log)

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
	secretResolver := provider.NewSecretResolver(pool, cfg.Provider.ConfigEncryptionKey)

	openAIKey := cfg.Provider.OpenAICompatibleAPIKey
	if openAIKey == "" {
		if key, err := secretResolver.Resolve(ctx, "openai-compatible", "api_key", "OPENAI_COMPATIBLE_API_KEY"); err != nil {
			log.Warn().Err(err).Msg("failed to resolve openai-compatible api key")
		} else {
			openAIKey = key
		}
	}
	registry.Register(provider.NewOpenAIProvider(
		cfg.Provider.OpenAICompatibleBaseURL,
		openAIKey,
		cfg.Provider.OpenAICompatibleModel,
		cfg.Provider.DisableJSONSchema,
		cfg.Provider.OpenAIMaxTokens,
		cfg.Provider.OpenAIImageModel,
		cfg.Provider.OpenAIImageSize,
		cfg.Provider.OpenAIImageQuality,
	))

	comfyKey := cfg.Provider.ComfyUIAPIKey
	if comfyKey == "" {
		if key, err := secretResolver.Resolve(ctx, "comfyui", "api_key", "COMFYUI_API_KEY"); err != nil {
			log.Warn().Err(err).Msg("failed to resolve comfyui api key")
		} else {
			comfyKey = key
		}
	}
	registry.Register(provider.NewComfyUIProvider(cfg.Provider.ComfyUIBaseURL, comfyKey))

	replicateKey := cfg.Provider.ReplicateAPIToken
	if replicateKey == "" {
		if key, err := secretResolver.Resolve(ctx, "replicate", "api_token", "REPLICATE_API_TOKEN"); err != nil {
			log.Warn().Err(err).Msg("failed to resolve replicate api token")
		} else {
			replicateKey = key
		}
	}
	registry.Register(provider.NewReplicateProvider(cfg.Provider.ReplicateBaseURL, replicateKey))
	log.Info().Msg("providers registered")

	// ── Core Dependencies ───────────────────────────────
	outboxWriter := eventbus.NewOutboxWriter(pool)

	// ── Asset & Storage ───────────────────────────────
	assetRepo := asset.NewCreator(pool, outboxWriter)
	storage, err := asset.NewStorage(cfg.MinIO)
	if err != nil {
		log.Warn().Err(err).Msg("minio storage unavailable; compose tasks and asset creation will fail")
	}

	// ── Task Processor ───────────────────────────────
	taskRepo := task.NewPgRepository(pool)
	billingSvc := billing.NewPgService(pool, outboxWriter)
	callRecorder := provider.NewPgCallRecorder(pool)
	processor := task.NewProcessor(taskRepo, registry, outboxWriter, billingSvc, callRecorder, cfg.Provider.MaxAttempts, log)
	processor.SetAssetCreator(assetRepo, storage)

	if storage != nil {
		processor.SetComposer(composepkg.NewService(assetRepo, storage))
		log.Info().Msg("ffmpeg composer initialized")
	}

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
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer inspector.Close()

	// Asynq client for enqueuing child tasks (cascade)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer asynqClient.Close()
	processor.SetAsynqClient(asynqClient)

	metricsCtx, metricsCancel := context.WithCancel(ctx)
	observability.StartQueueMetrics(metricsCtx, inspector, []string{"generation", "default", "low"}, 5*time.Second, log)

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
	metricsCancel()
	srv.Shutdown()
	observability.ShutdownDiagnostics(context.Background(), diagSrv)
	log.Info().Msg("worker stopped")
}
