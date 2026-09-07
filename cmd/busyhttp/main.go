package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/platformfix/busyhttp/internal/busyhttp"
)

// busySeconds reads BUSY_SECONDS as a float number of seconds, defaulting
// to 1s. An unparseable value is a fatal startup error rather than a
// silent fallback: it's an external input worth validating at the
// boundary, and a demo tool that silently ignores its own configuration
// knob is worse than one that refuses to start.
func busySeconds() time.Duration {
	raw := os.Getenv("BUSY_SECONDS")
	if raw == "" {
		return time.Second
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		slog.Error("invalid BUSY_SECONDS", "value", raw, "error", err)
		os.Exit(1)
	}
	return time.Duration(seconds * float64(time.Second))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	duration := busySeconds()

	mux := http.NewServeMux()
	mux.HandleFunc("/", busyhttp.NewHandler(duration))
	mux.HandleFunc("/healthz", busyhttp.Healthz)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "port", port, "busy_seconds", duration.Seconds())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
