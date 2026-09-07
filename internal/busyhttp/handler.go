// Package busyhttp serves an HTTP handler that burns CPU for a configurable
// duration on every request — a demo load generator for Kubernetes
// HPA/autoscaling exercises.
package busyhttp

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// NewHandler returns a handler that busy-spins the CPU for duration before
// responding. The spin is a literal deadline-polling loop, not time.Sleep:
// the whole point of this tool is that a request shows up as real CPU load,
// which an HPA can react to and time.Sleep would not produce.
func NewHandler(duration time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deadline := time.Now().Add(duration)
		for time.Now().Before(deadline) {
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "I've been busy for %ss.\n", formatSeconds(duration))

		slog.Info("request", "remote_addr", r.RemoteAddr, "method", r.Method, "path", r.URL.String(), "busy_seconds", duration.Seconds())
	}
}

// formatSeconds renders a duration as a plain seconds value, trimming a
// trailing ".0" so "1s" prints as "1" rather than "1.0".
func formatSeconds(d time.Duration) string {
	seconds := d.Seconds()
	if seconds == float64(int64(seconds)) {
		return fmt.Sprintf("%d", int64(seconds))
	}
	return fmt.Sprintf("%g", seconds)
}

// Healthz reports liveness/readiness for Kubernetes probes.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
