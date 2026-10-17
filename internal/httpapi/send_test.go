package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"clopy-paste/internal/messaging"
)

type sent struct {
	calls      int
	user, text string
	deadline   time.Time
	hasDL      bool
}

func sendServer(pub error, logger *slog.Logger) (http.Handler, *sent) {
	rec := &sent{}
	fake := &messaging.FakePublisher{PublishFn: func(ctx context.Context, u, t string) error {
		rec.calls++
		rec.user, rec.text = u, t
		rec.deadline, rec.hasDL = ctx.Deadline()
		return pub
	}}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	}
	return NewServer(Deps{Logger: logger, Censor: messaging.NewCensor("$!~"), Publisher: fake}).Handler(), rec
}

func post(h http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(body)))
	return rec
}

func TestSendMessage(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		h, f := sendServer(nil, nil)
		rec := post(h, `{"user":"bob$","text":"hi!"}`)
		if rec.Code != 202 || strings.TrimSpace(rec.Body.String()) != "{}" {
			t.Fatalf("got %d %q", rec.Code, rec.Body.String())
		}
		if f.user != "bob_" || f.text != "hi_" {
			t.Fatalf("published %q %q", f.user, f.text)
		}
	})
	t.Run("invalid-json", func(t *testing.T) {
		for _, b := range []string{"nope", `{"user":1,"text":"x"}`, `{"user":"a","text":"b"} x`} {
			h, f := sendServer(nil, nil)
			wantJSON(t, post(h, b), 400, "error", "invalid JSON body")
			if f.calls != 0 {
				t.Fatalf("publisher called for %q", b)
			}
		}
	})
	t.Run("limits", func(t *testing.T) {
		emoji := "\U0001F600"
		cases := []struct {
			user, text string
			code       int
			msg        string
		}{
			{"", "x", 400, "user must be 1..128 characters"},
			{"a", "x", 202, ""},
			{strings.Repeat(emoji, 128), "x", 202, ""},
			{strings.Repeat(emoji, 129), "x", 400, "user must be 1..128 characters"},
			{"a", "", 400, "text must be 1..1000 characters"},
			{"a", strings.Repeat(emoji, 1000), 202, ""},
			{"a", strings.Repeat(emoji, 1001), 400, "text must be 1..1000 characters"},
		}
		for _, c := range cases {
			h, f := sendServer(nil, nil)
			rec := post(h, `{"user":"`+c.user+`","text":"`+c.text+`"}`)
			if rec.Code != c.code {
				t.Fatalf("status %d want %d (%q)", rec.Code, c.code, rec.Body.String())
			}
			if c.code == 400 {
				wantJSON(t, rec, 400, "error", c.msg)
				if f.calls != 0 {
					t.Fatal("published on 4xx")
				}
			}
		}
		h, _ := sendServer(nil, nil)
		wantJSON(t, post(h, `{"text":"x"}`), 400, "error", "user must be 1..128 characters")
	})
	t.Run("invalid-utf8", func(t *testing.T) {
		h, f := sendServer(nil, nil)
		wantJSON(t, post(h, "{\"user\":\"a\xff\",\"text\":\"x\"}"), 400, "error", "user must be 1..128 characters")
		wantJSON(t, post(h, "{\"user\":\"a\",\"text\":\"x\xc3\"}"), 400, "error", "text must be 1..1000 characters")
		if f.calls != 0 {
			t.Fatal("published invalid UTF-8")
		}
		if rec := post(h, "{\"extra\":\"ÿ\",\"user\":\"a\",\"text\":\"x\"}"); rec.Code != 202 {
			t.Fatalf("unknown field with bad bytes: %d", rec.Code)
		}
	})
	t.Run("body-too-large", func(t *testing.T) {
		h, f := sendServer(nil, nil)
		body := `{"user":"a","text":"` + strings.Repeat("x", 16385-22) + `"}`
		if len(body) != 16385 {
			t.Fatalf("len %d", len(body))
		}
		wantJSON(t, post(h, body), 400, "error", "request body too large")
		if f.calls != 0 {
			t.Fatal("published")
		}
	})
	t.Run("kafka-down", func(t *testing.T) {
		h, _ := sendServer(errors.New("boom"), nil)
		wantJSON(t, post(h, `{"user":"a","text":"b"}`), 503, "error", "kafka unavailable, message not sent")
	})
	t.Run("deadline", func(t *testing.T) {
		h, f := sendServer(nil, nil)
		start := time.Now()
		post(h, `{"user":"a","text":"b"}`)
		if !f.hasDL || f.deadline.After(start.Add(5*time.Second+time.Second)) || f.deadline.Before(start.Add(4*time.Second)) {
			t.Fatalf("deadline %v (start %v)", f.deadline, start)
		}
	})
	t.Run("method", func(t *testing.T) {
		h, _ := sendServer(nil, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/messages", nil))
		wantJSON(t, rec, 405, "error", "method PUT not allowed")
		if a := rec.Header().Get("Allow"); !strings.Contains(a, "GET") || !strings.Contains(a, "POST") {
			t.Fatalf("Allow = %q", a)
		}
	})
	t.Run("no-text-in-logs", func(t *testing.T) {
		var buf bytes.Buffer
		lg := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		for _, e := range []error{nil, errors.New("SECRET-MARKER-123")} {
			h, _ := sendServer(e, lg)
			post(h, `{"user":"u","text":"SECRET-MARKER-123"}`)
		}
		post(func() http.Handler { h, _ := sendServer(nil, lg); return h }(), `{"user":"u","text":""}`)
		if strings.Contains(buf.String(), "SECRET-MARKER-123") {
			t.Fatalf("text leaked: %s", buf.String())
		}
	})
}

func storeServer(store messaging.Store) http.Handler {
	pub := &messaging.FakePublisher{}
	return NewServer(Deps{Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), Censor: messaging.NewCensor("$!~"), Publisher: pub, Store: store}).Handler()
}

func TestSendNotStored(t *testing.T) {
	msg := messaging.Message{Text: "m"}
	t.Run("does not create user", func(t *testing.T) {
		var logs bytes.Buffer
		store := messaging.NewStoreWithLogger(slog.New(slog.NewTextHandler(&logs, nil)))
		h := storeServer(store)
		if rec := post(h, `{"user":"X","text":"hi"}`); rec.Code != 202 {
			t.Fatalf("status %d", rec.Code)
		}
		if got := store.Get("X"); len(got) != 0 {
			t.Fatalf("X stored: %v", got)
		}
		// Capacity is 100 users: if send had created X, the 100th add below would evict it and log.
		for i := 0; i < 100; i++ {
			store.Add("u"+strconv.Itoa(i), msg)
		}
		if strings.Contains(logs.String(), "user evicted") {
			t.Fatalf("send created a user: %s", logs.String())
		}
	})
	t.Run("does not touch user", func(t *testing.T) {
		store := messaging.NewStore()
		store.Add("X", msg)
		for i := 0; i < 99; i++ {
			store.Add("u"+strconv.Itoa(i), msg)
		}
		if rec := post(storeServer(store), `{"user":"X","text":"hi"}`); rec.Code != 202 {
			t.Fatalf("status %d", rec.Code)
		}
		store.Add("new", msg)
		if got := store.Get("X"); len(got) != 0 {
			t.Fatal("X survived eviction: send touched it")
		}
	})
}
