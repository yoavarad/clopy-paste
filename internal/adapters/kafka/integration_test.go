//go:build integration

// Integration tests for the Kafka adapter against the Strimzi cluster from deploy/local
// (ydk:test:messaging/kafka-adapter.integration). Real broker, real franz-go client, real Store;
// no mocks. Run through scripts/test-integration.sh, which exports the KAFKA_IT_* variables.
package kafka_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"clopy-paste/internal/adapters/kafka"
	"clopy-paste/internal/config"
	"clopy-paste/internal/httpapi"
	"clopy-paste/internal/messaging"
)

// itEnv holds the broker coordinates scripts/test-integration.sh exports.
type itEnv struct {
	plain, sasl, user, pass, ca string
}

func loadEnv(t *testing.T) itEnv {
	t.Helper()
	e := itEnv{
		plain: os.Getenv("KAFKA_IT_PLAINTEXT_BOOTSTRAP"),
		sasl:  os.Getenv("KAFKA_IT_SASL_BOOTSTRAP"),
		user:  os.Getenv("KAFKA_IT_USERNAME"),
		pass:  os.Getenv("KAFKA_IT_PASSWORD"),
		ca:    os.Getenv("KAFKA_IT_CA_FILE"),
	}
	for name, v := range map[string]string{
		"KAFKA_IT_PLAINTEXT_BOOTSTRAP": e.plain, "KAFKA_IT_SASL_BOOTSTRAP": e.sasl,
		"KAFKA_IT_USERNAME": e.user, "KAFKA_IT_PASSWORD": e.pass, "KAFKA_IT_CA_FILE": e.ca,
	} {
		if v == "" {
			t.Fatalf("%s is not set; run scripts/test-integration.sh", name)
		}
	}
	return e
}

// uniq returns a name that no earlier run of this suite used.
func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// cfgFor builds the adapter config for the given security mode.
func (e itEnv) cfgFor(mode, in, out, group string) config.Config {
	cfg := config.Config{
		KafkaInputTopic:       in,
		KafkaOutputTopic:      out,
		KafkaConsumerGroup:    group,
		KafkaSecurityProtocol: mode,
	}
	if mode == "SASL_SSL" {
		cfg.KafkaBootstrapServers = []string{e.sasl}
		cfg.KafkaSaslUsername, cfg.KafkaSaslPassword, cfg.KafkaCaFile = e.user, e.pass, e.ca
	} else {
		cfg.KafkaBootstrapServers = []string{e.plain}
	}
	return cfg
}

func (e itEnv) connect(t *testing.T, cfg config.Config) *kgo.Client {
	t.Helper()
	cl, err := kafka.Connect(cfg)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(cl.Close)
	return cl
}

// rawClient is a plain PLAINTEXT client with no group, for producing test input and reading topics back.
func (e itEnv) rawClient(t *testing.T, opts ...kgo.Opt) *kgo.Client {
	t.Helper()
	cl, err := kafka.NewClient([]string{e.plain}, opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(cl.Close)
	return cl
}

func (e itEnv) createTopic(t *testing.T, prefix string) string {
	t.Helper()
	name := uniq(prefix)
	req := kmsg.NewPtrCreateTopicsRequest()
	rt := kmsg.NewCreateTopicsRequestTopic()
	rt.Topic, rt.NumPartitions, rt.ReplicationFactor = name, 1, 1
	req.Topics = append(req.Topics, rt)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := req.RequestWith(ctx, e.rawClient(t))
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if len(resp.Topics) != 1 {
		t.Fatalf("create topic %s: %d results", name, len(resp.Topics))
	}
	if ec := resp.Topics[0].ErrorCode; ec != 0 {
		t.Fatalf("create topic %s: error code %d", name, ec)
	}
	return name
}

func produce(t *testing.T, cl *kgo.Client, topic string, recs ...*kgo.Record) {
	t.Helper()
	for _, r := range recs {
		r.Topic = topic
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cl.ProduceSync(ctx, recs...).FirstErr(); err != nil {
		t.Fatalf("produce: %v", err)
	}
}

func valid(user, text string) *kgo.Record {
	return &kgo.Record{Key: []byte(user), Value: messaging.EncodeRecordValue(user, text)}
}

// syncBuf is a goroutine-safe log sink.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) lines() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(s.b.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(ln), &m) == nil {
			out = append(out, m)
		}
	}
	return out
}

// sentinelUser marks readiness probes that startConsumer produces; tests ignore this user.
const sentinelUser = "__sentinel__"

// runningConsumer is a Consumer polling in the background.
type runningConsumer struct {
	cl     *kgo.Client
	cancel context.CancelFunc
	done   chan error
}

// startConsumer joins group on in, then waits until the group has a stable assignment so that
// records produced afterwards are seen even from a new group starting at the end (ADR-017).
func (e itEnv) startConsumer(t *testing.T, mode, in, group string, store messaging.Store, logger *slog.Logger) *runningConsumer {
	t.Helper()
	cl := e.connect(t, e.cfgFor(mode, in, uniq("unused-out"), group))
	ctx, cancel := context.WithCancel(context.Background())
	rc := &runningConsumer{cl: cl, cancel: cancel, done: make(chan error, 1)}
	c := kafka.NewConsumer(cl, in, store, logger)
	go func() { rc.done <- c.Run(ctx) }()
	t.Cleanup(cancel)

	admin := e.rawClient(t)
	deadline := time.Now().Add(60 * time.Second)
	for {
		req := kmsg.NewPtrDescribeGroupsRequest()
		req.Groups = []string{group}
		resp, err := req.RequestWith(context.Background(), admin)
		if err == nil && len(resp.Groups) == 1 && resp.Groups[0].State == "Stable" && len(resp.Groups[0].Members) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("group %s did not become stable", group)
		}
		time.Sleep(200 * time.Millisecond)
	}
	// A stable group does not mean the end-offset lookup is done: produce sentinels until the
	// consumer has stored one, which proves it is fetching.
	raw := e.rawClient(t)
	deadline = time.Now().Add(60 * time.Second)
	for len(store.Get(sentinelUser)) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("consumer on %s never saw a sentinel record", in)
		}
		produce(t, raw, in, valid(sentinelUser, "ready"))
		time.Sleep(300 * time.Millisecond)
	}
	return rc
}

// stop shuts down gracefully: stop polling, commit marked offsets, leave the group.
func (rc *runningConsumer) stop(t *testing.T) {
	t.Helper()
	rc.cancel()
	select {
	case err := <-rc.done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rc.cl.CommitMarkedOffsets(ctx); err != nil {
		t.Fatalf("CommitMarkedOffsets: %v", err)
	}
	rc.cl.Close()
}

// waitTexts polls the store until user holds want messages (any order) or the deadline passes.
func waitTexts(t *testing.T, store messaging.Store, user string, want int, within time.Duration) []messaging.Message {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		msgs := store.Get(user)
		if len(msgs) >= want || time.Now().After(deadline) {
			return msgs
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func texts(msgs []messaging.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Text
	}
	sort.Strings(out)
	return out
}

func TestClusterIntegration(t *testing.T) {
	env := loadEnv(t)

	t.Run("publish-record-shape", func(t *testing.T) {
		topic := env.createTopic(t, "shape")
		reader := env.rawClient(t, kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		pubCl := env.connect(t, env.cfgFor("SASL_SSL", "unused-in", topic, uniq("g")))
		pub := kafka.NewPublisher(pubCl, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := pub.Publish(ctx, "bob_", "hi_"); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		fetches := reader.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			t.Fatalf("read back: %v", err)
		}
		recs := fetches.Records()
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		rec := recs[0]
		if string(rec.Key) != "bob_" {
			t.Errorf("key = %q, want bob_", rec.Key)
		}
		if want := `{"user":"bob_","text":"hi_"}`; string(rec.Value) != want {
			t.Errorf("value = %q, want %q", rec.Value, want)
		}
		if len(rec.Headers) != 0 {
			t.Errorf("headers = %v, want none", rec.Headers)
		}
	})

	t.Run("consume-to-store", func(t *testing.T) {
		in := env.createTopic(t, "c2s")
		store := messaging.NewStore()
		env.startConsumer(t, "SASL_SSL", in, uniq("g"), store, nil)

		ts := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
		rec := valid("carol", "hello")
		rec.Timestamp = ts
		start := time.Now()
		produce(t, env.rawClient(t), in, rec)

		msgs := waitTexts(t, store, "carol", 1, 5*time.Second)
		if len(msgs) != 1 || msgs[0].Text != "hello" {
			t.Fatalf("store = %+v, want one message \"hello\"", msgs)
		}
		if !msgs[0].Time.Equal(ts) {
			t.Errorf("time = %v, want record timestamp %v", msgs[0].Time, ts)
		}
		if took := time.Since(start); took > 5*time.Second {
			t.Errorf("took %v, want <= 5s", took)
		}
	})

	t.Run("drop-invalid", func(t *testing.T) {
		in := env.createTopic(t, "drop")
		store := messaging.NewStore()
		logs := &syncBuf{}
		logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
		env.startConsumer(t, "SASL_SSL", in, uniq("g"), store, logger)

		produce(t, env.rawClient(t), in,
			&kgo.Record{Key: []byte("a"), Value: []byte("not json")},
			&kgo.Record{Key: []byte("a"), Value: []byte(`{"user":"a"}`)},
			valid("a", "good"),
		)
		msgs := waitTexts(t, store, "a", 1, 10*time.Second)
		if got := texts(msgs); len(got) != 1 || got[0] != "good" {
			t.Fatalf("stored = %v, want only [good]", got)
		}
		// The valid record is processed last, so both drops were already logged.
		var warns []map[string]any
		for _, l := range logs.lines() {
			if l["level"] == "WARN" && l["msg"] == "dropped invalid record" {
				warns = append(warns, l)
			}
		}
		if len(warns) != 2 {
			t.Fatalf("got %d drop warnings, want 2: %v", len(warns), warns)
		}
		for i, w := range warns {
			off, ok := w["offset"].(float64)
			first, _ := warns[0]["offset"].(float64)
			if !ok || int(off) != int(first)+i {
				t.Errorf("warning %d offset = %v, want consecutive from %v", i, w["offset"], first)
			}
		}
	})

	t.Run("new-group-latest", func(t *testing.T) {
		in := env.createTopic(t, "latest")
		raw := env.rawClient(t)
		produce(t, raw, in, valid("d", "old1"), valid("d", "old2"), valid("d", "old3"))

		store := messaging.NewStore()
		env.startConsumer(t, "SASL_SSL", in, uniq("never-used"), store, nil)
		produce(t, raw, in, valid("d", "new"))

		msgs := waitTexts(t, store, "d", 1, 10*time.Second)
		time.Sleep(2 * time.Second) // give any wrongly replayed old record time to arrive
		msgs = store.Get("d")
		if got := texts(msgs); len(got) != 1 || got[0] != "new" {
			t.Fatalf("delivered = %v, want only [new]", got)
		}
	})

	t.Run("offset-resume", func(t *testing.T) {
		in := env.createTopic(t, "resume")
		group := uniq("resume-group")
		raw := env.rawClient(t)

		first := messaging.NewStore()
		rc := env.startConsumer(t, "SASL_SSL", in, group, first, nil)
		produce(t, raw, in, valid("e", "r1"), valid("e", "r2"), valid("e", "r3"))
		if got := waitTexts(t, first, "e", 3, 10*time.Second); len(got) != 3 {
			t.Fatalf("first consumer got %d records, want 3", len(got))
		}
		rc.stop(t)

		produce(t, raw, in, valid("e", "r4"), valid("e", "r5"))
		second := messaging.NewStore()
		env.startConsumer(t, "SASL_SSL", in, group, second, nil)
		waitTexts(t, second, "e", 2, 15*time.Second)
		time.Sleep(2 * time.Second) // give any wrongly replayed record time to arrive
		if got := texts(second.Get("e")); len(got) != 2 || got[0] != "r4" || got[1] != "r5" {
			t.Fatalf("second consumer delivered %v, want [r4 r5]", got)
		}
	})

	t.Run("plaintext-mode", func(t *testing.T) {
		out := env.createTopic(t, "plain")
		pub := kafka.NewPublisher(env.connect(t, env.cfgFor("PLAINTEXT", "unused-in", out, uniq("g"))), nil)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := pub.Publish(ctx, "frank", "plain hello"); err != nil {
			t.Fatalf("Publish over PLAINTEXT: %v", err)
		}
	})

	t.Run("bad-credentials", func(t *testing.T) {
		out := env.createTopic(t, "badcred")
		cfg := env.cfgFor("SASL_SSL", "unused-in", out, uniq("g"))
		cfg.KafkaSaslPassword = "definitely-wrong-" + env.pass
		pub := kafka.NewPublisher(env.connect(t, cfg), nil)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := pub.Publish(ctx, "mallory", "nope")
		if err == nil {
			t.Fatal("Publish succeeded with a wrong password")
		}
		if !errors.Is(err, messaging.ErrKafkaUnavailable) {
			t.Errorf("error = %v, want it to wrap ErrKafkaUnavailable", err)
		}
	})

	t.Run("latency-throughput", func(t *testing.T) {
		const (
			sends   = 500
			workers = 10
			warmup  = 10
		)
		out := env.createTopic(t, "perf")
		pub := kafka.NewPublisher(env.connect(t, env.cfgFor("SASL_SSL", "unused-in", out, uniq("g"))),
			slog.New(slog.NewTextHandler(discard{}, nil)))
		srv := httptest.NewServer(httpapi.NewServer(httpapi.Deps{
			Logger:       slog.New(slog.NewTextHandler(discard{}, nil)),
			Censor:       messaging.NewCensor(""),
			Publisher:    pub,
			Store:        messaging.NewStore(),
			InstanceName: "it",
		}).Handler())
		defer srv.Close()

		// 256 code points mixing 1-, 2-, 3- and 4-byte runes.
		text := strings.Repeat("aé€😀", 64)
		body, err := json.Marshal(map[string]string{"user": "perf", "text": text})
		if err != nil {
			t.Fatal(err)
		}
		httpc := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: workers}}
		defer httpc.CloseIdleConnections()
		send := func() (time.Duration, error) {
			start := time.Now()
			resp, err := httpc.Post(srv.URL+"/api/messages", "application/json", bytes.NewReader(body))
			if err != nil {
				return 0, err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted {
				return 0, fmt.Errorf("status %d", resp.StatusCode)
			}
			return time.Since(start), nil
		}

		// Warm up so connection setup and metadata lookups are not billed to the measured sends.
		for range warmup {
			if _, err := send(); err != nil {
				t.Fatalf("warm-up send: %v", err)
			}
		}

		var (
			mu        sync.Mutex
			durations = make([]time.Duration, 0, sends)
			failures  int
			wg        sync.WaitGroup
			jobs      = make(chan struct{}, sends)
		)
		for range sends {
			jobs <- struct{}{}
		}
		close(jobs)
		begin := time.Now()
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range jobs {
					d, err := send()
					mu.Lock()
					if err != nil {
						failures++
					} else {
						durations = append(durations, d)
					}
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(begin)

		if failures != 0 {
			t.Errorf("%d of %d sends failed, want 0", failures, sends)
		}
		if len(durations) == 0 {
			t.Fatal("no successful sends")
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		pct := func(p float64) time.Duration { return durations[min(len(durations)-1, int(float64(len(durations))*p))] }
		p95, p99 := pct(0.95), pct(0.99)
		rate := float64(len(durations)) / elapsed.Seconds()
		t.Logf("latency P95=%v P99=%v; throughput=%.1f sends/s over %v", p95, p99, rate, elapsed)
		if p95 > 200*time.Millisecond {
			t.Errorf("P95 = %v, want <= 200ms", p95)
		}
		if p99 > 500*time.Millisecond {
			t.Errorf("P99 = %v, want <= 500ms", p99)
		}
		if rate < 50 {
			t.Errorf("throughput = %.1f sends/s, want >= 50", rate)
		}
	})
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
