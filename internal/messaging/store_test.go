package messaging

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestStore() Store {
	return NewStoreWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func msg(text string) Message { return Message{Text: text, Time: time.Unix(0, 0)} }

func uname(i int) string { return fmt.Sprintf("u%03d", i) }

func TestStore(t *testing.T) {
	t.Run("fifo-101", func(t *testing.T) {
		s := newTestStore()
		for i := 1; i <= 101; i++ {
			s.Add("bob", msg(fmt.Sprintf("m%d", i)))
		}
		got := s.Get("bob")
		if len(got) != 100 || got[0].Text != "m101" || got[99].Text != "m2" {
			t.Fatalf("len=%d first=%q last=%q", len(got), got[0].Text, got[len(got)-1].Text)
		}
	})
	t.Run("lru-evict", func(t *testing.T) {
		s := newTestStore()
		for i := 1; i <= 100; i++ {
			s.Add(uname(i), msg("x"))
		}
		s.Add(uname(101), msg("x"))
		if g := s.Get(uname(1)); len(g) != 0 {
			t.Fatalf("u001 not evicted")
		}
		if len(s.Get(uname(2))) != 1 || len(s.Get(uname(101))) != 1 {
			t.Fatal("u002/u101 missing")
		}
	})
	t.Run("get-touches", func(t *testing.T) {
		s := newTestStore()
		for i := 1; i <= 100; i++ {
			s.Add(uname(i), msg("x"))
		}
		s.Get(uname(1))
		s.Add(uname(101), msg("x"))
		if len(s.Get(uname(2))) != 0 {
			t.Fatal("u002 not evicted")
		}
		if len(s.Get(uname(1))) != 1 {
			t.Fatal("u001 evicted")
		}
	})
	t.Run("unknown-not-created", func(t *testing.T) {
		s := newTestStore()
		for i := 1; i <= 100; i++ {
			s.Add(uname(i), msg("x"))
		}
		for i := 1; i <= 50; i++ {
			g := s.Get(fmt.Sprintf("x%d", i))
			if g == nil || len(g) != 0 {
				t.Fatalf("x%d: got %#v", i, g)
			}
		}
		for i := 1; i <= 100; i++ {
			if len(s.Get(uname(i))) != 1 {
				t.Fatalf("%s missing", uname(i))
			}
		}
	})
	t.Run("copy-returned", func(t *testing.T) {
		s := newTestStore()
		s.Add("bob", msg("hi"))
		s.Get("bob")[0].Text = "changed"
		if s.Get("bob")[0].Text != "hi" {
			t.Fatal("store mutated via returned slice")
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		s := newTestStore()
		var wg sync.WaitGroup
		for g := 0; g < 50; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 200; i++ {
					u := uname((g*7 + i) % 150)
					s.Add(u, msg("x"))
					s.Get(u)
				}
			}()
		}
		wg.Wait()
		n := 0
		for i := 0; i < 150; i++ {
			if len(s.Get(uname(i))) > 0 {
				n++
			}
		}
		if n > 100 {
			t.Fatalf("users=%d", n)
		}
	})
	t.Run("memory-worst-case", func(t *testing.T) {
		s := newTestStore()
		text := strings.Repeat("\U0001F600", 1000)
		for u := 0; u < 100; u++ {
			for m := 0; m < 100; m++ {
				// distinct string per message, as real input would be
				s.Add(uname(u), msg(text+fmt.Sprint(m)))
			}
		}
		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > 100<<20 {
			t.Fatalf("HeapAlloc=%d MiB", ms.HeapAlloc>>20)
		}
		runtime.KeepAlive(s)
	})
	t.Run("add-touches-and-logs-eviction", func(t *testing.T) {
		var buf bytes.Buffer
		s := NewStoreWithLogger(slog.New(slog.NewTextHandler(&buf, nil)))
		for i := 1; i <= 100; i++ {
			s.Add(uname(i), msg("x"))
		}
		if buf.Len() != 0 {
			t.Fatalf("unexpected log: %s", buf.String())
		}
		s.Add(uname(1), msg("y"))
		s.Add(uname(101), msg("x"))
		if len(s.Get(uname(1))) != 2 {
			t.Fatal("u001 evicted despite Add touch")
		}
		out := buf.String()
		if !strings.Contains(out, "level=INFO") || !strings.Contains(out, "user evicted") || !strings.Contains(out, "user=u002") {
			t.Fatalf("log=%q", out)
		}
	})
}
