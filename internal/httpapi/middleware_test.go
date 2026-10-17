package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func logServer(buf *bytes.Buffer) *Server {
	return NewServer(Deps{Logger: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))})
}

func TestRequestLogging(t *testing.T) {
	t.Run("debug line with method, pattern, status, duration_ms", func(t *testing.T) {
		var buf bytes.Buffer
		h := logServer(&buf).Handler()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		out := buf.String()
		for _, want := range []string{"level=DEBUG", "method=GET", "route=\"GET /{$}\"", "status=200", "duration_ms="} {
			if !strings.Contains(out, want) {
				t.Errorf("log missing %q: %s", want, out)
			}
		}
	})
	t.Run("health routes excluded", func(t *testing.T) {
		var buf bytes.Buffer
		h := logServer(&buf).Handler()
		for _, p := range []string{"/healthz", "/readyz"} {
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
		}
		if buf.Len() != 0 {
			t.Fatalf("unexpected log: %s", buf.String())
		}
	})
	t.Run("unmatched route logged with status 404", func(t *testing.T) {
		var buf bytes.Buffer
		logServer(&buf).Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nope", nil))
		if !strings.Contains(buf.String(), "status=404") {
			t.Fatalf("log: %s", buf.String())
		}
	})
	t.Run("panic logged at ERROR internal error with route, method, error", func(t *testing.T) {
		var buf bytes.Buffer
		logServer(&buf).Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/messages", nil))
		out := buf.String()
		for _, want := range []string{"level=ERROR", "msg=\"internal error\"", "method=POST", "route=\"POST /api/messages\"", "error=", "status=500"} {
			if !strings.Contains(out, want) {
				t.Errorf("log missing %q: %s", want, out)
			}
		}
	})
	t.Run("statusRecorder defaults to 200", func(t *testing.T) {
		rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
		_, _ = rec.Write([]byte("x"))
		if rec.status != http.StatusOK {
			t.Fatalf("status = %d", rec.status)
		}
	})
}
