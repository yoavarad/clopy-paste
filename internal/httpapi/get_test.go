package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"clopy-paste/internal/messaging"
)

func newGetServer(store messaging.Store) *Server {
	return NewServer(Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Censor: messaging.NewCensor("$!~"),
		Store:  store,
	})
}

func TestGetMessages(t *testing.T) {
	t.Run("newest-first", func(t *testing.T) {
		st := messaging.NewStore()
		t1 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
		t2 := t1.Add(time.Minute)
		st.Add("bob", messaging.Message{Text: "m1", Time: t1})
		st.Add("bob", messaging.Message{Text: "m2", Time: t2})
		rec := do(newGetServer(st).Handler(), http.MethodGet, "/api/messages?user=bob")
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
		var got []struct{ Text, Time string }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Text != "m2" || got[1].Text != "m1" {
			t.Fatalf("got %+v", got)
		}
		if got[0].Time != t2.UTC().Format(time.RFC3339) {
			t.Fatalf("time = %q", got[0].Time)
		}
		if got[1].Time != t1.UTC().Format(time.RFC3339) {
			t.Fatalf("time = %q", got[1].Time)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		st := messaging.NewStore()
		rec := do(newGetServer(st).Handler(), http.MethodGet, "/api/messages?user=nobody")
		if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
			t.Fatalf("got %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("censored-lookup", func(t *testing.T) {
		st := messaging.NewStore()
		st.Add("bob_", messaging.Message{Text: "hi", Time: time.Now()})
		rec := do(newGetServer(st).Handler(), http.MethodGet, "/api/messages?user="+url.QueryEscape("bob$"))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"hi"`) {
			t.Fatalf("got %d %q", rec.Code, rec.Body.String())
		}
	})
	t.Run("max-length-multibyte", func(t *testing.T) {
		rec := do(newGetServer(messaging.NewStore()).Handler(), http.MethodGet, "/api/messages?user="+url.QueryEscape(strings.Repeat("é", 128)))
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("bad-param", func(t *testing.T) {
		h := newGetServer(messaging.NewStore()).Handler()
		for _, q := range []string{"", "?user=", "?user=" + strings.Repeat("a", 129), "?user=%ff"} {
			wantJSON(t, do(h, http.MethodGet, "/api/messages"+q), 400, "error", "user must be 1..128 characters")
		}
	})
}
