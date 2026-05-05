package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

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

	// ── Prometheus metrics ───────────────────────────
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		srv := &http.Server{
			Addr:    ":9090",
			Handler: mux,
		}
		log.Info().Msg("prometheus metrics on :9090/metrics")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("metrics server error")
		}
	}()

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
	time.Sleep(500 * time.Millisecond) // allow in-flight to complete
	log.Info().Msg("outbox-relay stopped")
}
