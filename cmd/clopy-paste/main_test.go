package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"clopy-paste/internal/config"
	"clopy-paste/internal/messaging"
)

// events records the order of calls across the fakes at the Kafka boundary.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, s)
}

func (e *events) get() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.list...)
}

// fakeClient stands in for *kgo.Client. blockFlush makes Flush wait for its deadline.
type fakeClient struct {
	ev         *events
	blockFlush bool
}

func (c *fakeClient) Flush(ctx context.Context) error {
	if c.blockFlush {
		<-ctx.Done()
		return ctx.Err()
	}
	c.ev.add("flush")
	return nil
}

func (c *fakeClient) CommitMarkedOffsets(context.Context) error { c.ev.add("commit"); return nil }
func (c *fakeClient) LeaveGroupContext(context.Context) error   { c.ev.add("leave"); return nil }
func (c *fakeClient) Close()                                    { c.ev.add("close") }

// fakeConsumer polls until its context is cancelled, like kafka.Consumer.Run.
type fakeConsumer struct{ ev *events }

func (f *fakeConsumer) Run(ctx context.Context) error {
	<-ctx.Done()
	f.ev.add("consumer stopped")
	return nil
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		out = append(out, m)
	}
	return out
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func findLog(lines []map[string]any, msg string) map[string]any {
	for _, l := range lines {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	env := map[string]string{
		"INSTANCE_NAME":           "alpha",
		"KAFKA_BOOTSTRAP_SERVERS": "127.0.0.1:1",
		"KAFKA_INPUT_TOPIC":       "in-topic",
		"KAFKA_OUTPUT_TOPIC":      "out-topic",
		"KAFKA_CONSUMER_GROUP":    "",
		"KAFKA_SECURITY_PROTOCOL": "PLAINTEXT",
		"KAFKA_SASL_USERNAME":     "",
		"KAFKA_SASL_PASSWORD":     "",
		"KAFKA_CA_FILE":           "",
		"CENSOR_CHARS":            "$!~",
		"LOG_LEVEL":               "INFO",
		"GOMEMLIMIT":              "100MiB",
	}
	for k, v := range overrides {
		env[k] = v
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

// harness runs app.run in the background with fakes at the Kafka boundary.
type harness struct {
	out       *syncBuffer
	ev        *events
	client    *fakeClient
	publisher *messaging.FakePublisher
	connected bool
	ready     chan net.Addr
	app       app
}

func newHarness() *harness {
	h := &harness{out: &syncBuffer{}, ev: &events{}, publisher: &messaging.FakePublisher{}, ready: make(chan net.Addr, 1)}
	h.client = &fakeClient{ev: h.ev}
	h.app = app{
		stdout: h.out,
		addr:   "127.0.0.1:0",
		connect: func(config.Config, messaging.Store, *slog.Logger) (kafkaSide, error) {
			h.connected = true
			return kafkaSide{publisher: h.publisher, consumer: &fakeConsumer{ev: h.ev}, client: h.client}, nil
		},
		shutdownTimeout: shutdownTimeout,
		onReady:         func(a net.Addr) { h.ready <- a },
	}
	return h
}

// start runs the app; the returned func cancels it (the "signal") and waits for the exit code.
func (h *harness) start(t *testing.T) (addr string, stop func() int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	code := make(chan int, 1)
	go func() { code <- h.app.run(ctx) }()
	select {
	case a := <-h.ready:
		addr = "http://" + a.String()
	case c := <-code:
		cancel()
		t.Fatalf("run exited early with %d: %s", c, h.out.String())
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("not ready within 5 s")
	}
	return addr, func() int {
		cancel()
		select {
		case c := <-code:
			return c
		case <-time.After(15 * time.Second):
			t.Fatal("run did not return within 15 s")
			return -1
		}
	}
}

func TestShutdownAnswersInFlightPublishInOrder(t *testing.T) {
	setEnv(t, nil)
	h := newHarness()
	started := make(chan struct{})
	h.publisher.PublishFn = func(ctx context.Context, user, text string) error {
		close(started)
		time.Sleep(2 * time.Second)
		h.ev.add("publish done")
		return nil
	}
	addr, stop := h.start(t)

	status := make(chan int, 1)
	go func() {
		resp, err := http.Post(addr+"/api/messages", "application/json", strings.NewReader(`{"user":"alice","text":"hi"}`))
		if err != nil {
			status <- -1
			return
		}
		resp.Body.Close()
		status <- resp.StatusCode
	}()
	<-started

	begin := time.Now()
	code := stop()
	elapsed := time.Since(begin)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; logs: %s", code, h.out.String())
	}
	if elapsed > shutdownTimeout {
		t.Fatalf("shutdown took %v, want <= %v", elapsed, shutdownTimeout)
	}
	if got := <-status; got != http.StatusAccepted {
		t.Fatalf("in-flight POST status = %d, want 202", got)
	}
	want := []string{"publish done", "flush", "consumer stopped", "commit", "leave", "close"}
	if got := h.ev.get(); !reflect.DeepEqual(got, want) {
		t.Fatalf("shutdown order = %v, want %v", got, want)
	}
	done := findLog(h.out.lines(t), "shutdown complete")
	if done == nil || done["level"] != "INFO" || done["duration_ms"] == nil {
		t.Fatalf("missing INFO shutdown complete with duration_ms: %s", h.out.String())
	}
}

func TestShutdownDeadlineLogsStepAndExitsNonZero(t *testing.T) {
	setEnv(t, nil)
	h := newHarness()
	h.client.blockFlush = true
	h.app.shutdownTimeout = 200 * time.Millisecond
	_, stop := h.start(t)

	if code := stop(); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	warn := findLog(h.out.lines(t), "shutdown deadline exceeded")
	if warn == nil || warn["level"] != "WARN" || warn["step"] != "flush" {
		t.Fatalf("want WARN shutdown deadline exceeded step=flush, logs: %s", h.out.String())
	}
	for _, e := range h.ev.get() {
		if e == "commit" || e == "leave" || e == "close" {
			t.Fatalf("step %q ran after the deadline: %v", e, h.ev.get())
		}
	}
}

// TestConnectKafkaDoesNotWaitForBroker builds the real client against an address where no
// broker listens: startup must not block on Kafka being reachable (ADR-016).
func TestConnectKafkaDoesNotWaitForBroker(t *testing.T) {
	setEnv(t, nil)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Now()
	side, err := connectKafka(cfg, messaging.NewStore(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("connectKafka: %v", err)
	}
	defer side.client.Close()
	if elapsed := time.Since(begin); elapsed > 3*time.Second {
		t.Fatalf("connectKafka took %v with no broker, want < 3 s", elapsed)
	}
	if side.publisher == nil || side.consumer == nil {
		t.Fatal("connectKafka returned a nil publisher or consumer")
	}
}

func TestReadyAndStartupLogWithoutPassword(t *testing.T) {
	const password = "pw-must-not-leak"
	setEnv(t, map[string]string{
		"KAFKA_SECURITY_PROTOCOL": "SASL_SSL",
		"KAFKA_SASL_USERNAME":     "svc-user",
		"KAFKA_SASL_PASSWORD":     password,
	})
	h := newHarness()
	addr, stop := h.start(t)
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(addr + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
	if code := stop(); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if strings.Contains(h.out.String(), password) {
		t.Fatalf("logs contain the SASL password: %s", h.out.String())
	}
	lines := h.out.lines(t)
	start := findLog(lines, "starting")
	if start == nil {
		t.Fatalf("no starting line: %s", h.out.String())
	}
	wantAttrs := map[string]any{
		"version": version, "instance": "alpha", "input_topic": "in-topic",
		"output_topic": "out-topic", "protocol": "SASL_SSL", "censor_chars": float64(3),
	}
	for k, v := range wantAttrs {
		if start[k] != v {
			t.Errorf("starting %s = %v, want %v", k, start[k], v)
		}
	}
	if findLog(lines, "ready") == nil {
		t.Fatalf("no ready line: %s", h.out.String())
	}
}

func TestInvalidConfigExitsBeforeListening(t *testing.T) {
	setEnv(t, map[string]string{"INSTANCE_NAME": ""})
	h := newHarness()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.app.addr = ln.Addr().String()
	ln.Close()

	if code := h.app.run(context.Background()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	lines := h.out.lines(t)
	if len(lines) != 1 || lines[0]["level"] != "ERROR" || lines[0]["msg"] != "invalid config" ||
		!strings.Contains(h.out.String(), "INSTANCE_NAME") {
		t.Fatalf("want exactly one ERROR invalid config naming INSTANCE_NAME, got: %s", h.out.String())
	}
	if h.connected || len(h.ready) != 0 {
		t.Fatal("kafka connected or server became ready despite invalid config")
	}
	if c, err := net.DialTimeout("tcp", h.app.addr, time.Second); err == nil {
		c.Close()
		t.Fatal("something is listening after invalid config")
	}
}

func TestConnectErrorExitsBeforeListening(t *testing.T) {
	setEnv(t, nil)
	h := newHarness()
	h.app.connect = func(config.Config, messaging.Store, *slog.Logger) (kafkaSide, error) {
		return kafkaSide{}, errors.New("bad CA file")
	}
	if code := h.app.run(context.Background()); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if l := findLog(h.out.lines(t), "kafka client setup failed"); l == nil || l["level"] != "ERROR" {
		t.Fatalf("want ERROR kafka client setup failed, got: %s", h.out.String())
	}
	if len(h.ready) != 0 {
		t.Fatal("server became ready despite connect error")
	}
}
