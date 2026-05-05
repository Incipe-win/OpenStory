// Package observability provides structured logging and telemetry utilities.
package observability

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// NewLogger creates a new zerolog.Logger configured for the given environment.
// In "development" mode it outputs human-readable console logs;
// otherwise it outputs structured JSON.
func NewLogger(appEnv string) zerolog.Logger {
	var w io.Writer
	if appEnv == "development" {
		w = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	} else {
		w = os.Stdout
	}

	return zerolog.New(w).
		With().
		Timestamp().
		Caller().
		Logger()
}
