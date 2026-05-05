package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := observability.NewLogger(cfg.Server.Env)
	log.Info().Msg("starting OpenStory outbox-relay (placeholder)")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("outbox-relay stopped")
}
