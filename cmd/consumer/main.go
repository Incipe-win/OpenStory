package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/consumer"
	"github.com/Incipe-win/OpenStory/internal/db"
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
	name := os.Getenv("CONSUMER_NAME")
	if name == "" {
		name = consumer.NameAnalytics
	}
	metricsAddr := os.Getenv("CONSUMER_METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = cfg.Observability.DiagnosticsAddr
	}
	traceShutdown, err := observability.InitTracer(context.Background(), observability.TraceConfig{
		ServiceName:  cfg.Observability.ServiceName + "-" + name,
		Enabled:      cfg.Observability.TracingEnabled,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		OTLPInsecure: cfg.Observability.OTLPInsecure,
	})
	if err != nil {
		log.Warn().Err(err).Msg("OpenTelemetry tracing disabled")
		traceShutdown = func(context.Context) error { return nil }
	}
	defer traceShutdown(context.Background()) //nolint:errcheck

	def, ok := consumer.DefinitionByName(name)
	if !ok {
		log.Fatal().Str("consumer", name).Msg("unknown consumer")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	outbox.EnsureTopics(cfg.Kafka.Brokers, log)
	store := consumer.NewPgStore(pool)
	dlq := consumer.NewDLQPublisher(cfg.Kafka.Brokers)
	defer dlq.Close()

	diagSrv := observability.StartDiagnosticsServer(metricsAddr, log)

	runner := consumer.NewRunner(def, cfg.Kafka.Brokers, store, dlq, log)
	defer runner.Close()

	go func() {
		if err := runner.Run(ctx); err != nil {
			log.Error().Err(err).Msg("consumer stopped with error")
		}
	}()
	log.Info().Str("consumer", def.Name).Str("topic", def.Topic).Str("group", def.GroupID).Msg("consumer started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	observability.ShutdownDiagnostics(context.Background(), diagSrv)
	log.Info().Msg("consumer stopped")
}
