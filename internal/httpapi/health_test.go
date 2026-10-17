package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() *Server {
	return NewServer(testDeps(Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func wantJSON(t *testing.T, rec *httptest.ResponseRecorder, status int, key, val string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d", rec.Code, status)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not JSON: %v (%q)", err, rec.Body.String())
	}
	if got[key] != val {
		t.Fatalf("body = %v, want %s=%q", got, key, val)
	}
}

func TestReadyz(t *testing.T) {
	t.Run("healthz", func(t *testing.T) {
		wantJSON(t, do(newTestServer().Handler(), "GET", "/healthz"), 200, "status", "ok")
	})
	t.Run("ready", func(t *testing.T) {
		// ADR-016: readiness is independent of Kafka; no Publisher is consulted.
		wantJSON(t, do(newTestServer().Handler(), "GET", "/readyz"), 200, "status", "ok")
	})
	t.Run("shutting-down", func(t *testing.T) {
		s := newTestServer()
		s.SetReady(false)
		wantJSON(t, do(s.Handler(), "GET", "/readyz"), 503, "error", "shutting down")
	})
	t.Run("in-flight-finishes", func(t *testing.T) {
		s := newTestServer()
		release := make(chan struct{})
		started := make(chan struct{})
		inner := s.Handler()
		ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/slow" {
				close(started)
				<-release
				w.WriteHeader(http.StatusAccepted)
				return
			}
			inner.ServeHTTP(w, r)
		}))
		ts.Start()
		defer ts.Close()

		type result struct {
			code int
			err  error
		}
		resCh := make(chan result, 1)
		go func() {
			resp, err := http.Get(ts.URL + "/slow")
			if err != nil {
				resCh <- result{err: err}
				return
			}
			resp.Body.Close()
			resCh <- result{code: resp.StatusCode}
		}()
		<-started

		s.SetReady(false)
		shutDone := make(chan error, 1)
		begin := time.Now()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			shutDone <- ts.Config.Shutdown(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
		close(release)

		r := <-resCh
		if r.err != nil || r.code != http.StatusAccepted {
			t.Fatalf("in-flight = %d, %v; want 202", r.code, r.err)
		}
		if err := <-shutDone; err != nil {
			t.Fatalf("shutdown: %v", err)
		}
		if d := time.Since(begin); d > 10*time.Second {
			t.Fatalf("shutdown took %v", d)
		}
	})
	t.Run("not-found", func(t *testing.T) {
		wantJSON(t, do(newTestServer().Handler(), "GET", "/nope"), 404, "error", "not found")
	})
}

func TestErrorFormat(t *testing.T) {
	t.Run("405 keeps Allow and JSON body", func(t *testing.T) {
		rec := do(newTestServer().Handler(), "POST", "/healthz")
		wantJSON(t, rec, 405, "error", "method POST not allowed")
		if !strings.Contains(rec.Header().Get("Allow"), "GET") {
			t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
		}
	})
	t.Run("panic -> 500 internal error", func(t *testing.T) {
		wantJSON(t, do(panicHandler(newTestServer()), "POST", "/api/messages"), 500, "error", "internal error")
	})
}
