package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/db"
	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
	"github.com/Incipe-win/OpenStory/internal/outbox"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := observability.NewLogger(cfg.Server.Env)
	log.Info().Msg("starting OpenStory outbox-relay")
	traceShutdown, err := observability.InitTracer(context.Background(), observability.TraceConfig{
		ServiceName:  cfg.Observability.ServiceName + "-outbox-relay",
		Enabled:      cfg.Observability.TracingEnabled,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		OTLPInsecure: cfg.Observability.OTLPInsecure,
	})
	if err != nil {
		log.Warn().Err(err).Msg("OpenTelemetry tracing disabled")
		traceShutdown = func(context.Context) error { return nil }
	}
	defer traceShutdown(context.Background()) //nolint:errcheck

	// ── Database ─────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	// ── Kafka ────────────────────────────────────────
	publisher := eventbus.NewKafkaPublisher(cfg.Kafka.Brokers)
	defer publisher.Close()
	log.Info().Strs("brokers", cfg.Kafka.Brokers).Msg("kafka publisher initialized")

	// Ensure topics exist
	outbox.EnsureTopics(cfg.Kafka.Brokers, log)

	diagSrv := observability.StartDiagnosticsServer(cfg.Observability.DiagnosticsAddr, log)

	// ── Relay ────────────────────────────────────────
	relay := outbox.NewRelay(pool, publisher, log)

	go func() {
		if err := relay.Run(ctx); err != nil {
			log.Error().Err(err).Msg("relay error")
		}
	}()

	// ── Graceful Shutdown ────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down outbox-relay...")
	cancel()
	observability.ShutdownDiagnostics(context.Background(), diagSrv)
	time.Sleep(500 * time.Millisecond) // allow in-flight to complete
	log.Info().Msg("outbox-relay stopped")
}
