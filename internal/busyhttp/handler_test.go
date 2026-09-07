package busyhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHandler_BusySpinsForDuration(t *testing.T) {
	duration := 20 * time.Millisecond
	handler := NewHandler(duration)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler(rec, req)
	elapsed := time.Since(start)

	if elapsed < duration {
		t.Fatalf("handler returned after %s, want at least %s", elapsed, duration)
	}

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	want := "I've been busy for 0.02s.\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestNewHandler_WholeSecondFormatsWithoutDecimal(t *testing.T) {
	handler := NewHandler(time.Second)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	want := "I've been busy for 1s.\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Healthz(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "ok"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
