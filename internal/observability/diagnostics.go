package observability

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

func DiagnosticsMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func StartDiagnosticsServer(addr string, log zerolog.Logger) *http.Server {
	if addr == "" {
		return nil
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           DiagnosticsMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info().Str("addr", addr).Msg("observability diagnostics server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("observability diagnostics server failed")
		}
	}()
	return srv
}

func ShutdownDiagnostics(ctx context.Context, srv *http.Server) {
	if srv == nil {
		return
	}
	_ = srv.Shutdown(ctx)
}
