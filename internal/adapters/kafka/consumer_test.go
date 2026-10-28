package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	"clopy-paste/internal/messaging"
)

// logLines decodes every JSON log line written to buf.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func newTestConsumer(store messaging.Store, buf *bytes.Buffer) *Consumer {
	return NewConsumer(nil, "in-topic", store, slog.New(slog.NewJSONHandler(buf, nil)))
}

func testRecord(value []byte) *kgo.Record {
	return &kgo.Record{
		Key:       []byte("ignored-key"),
		Value:     value,
		Topic:     "in-topic",
		Partition: 3,
		Offset:    42,
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func assertRecordFields(t *testing.T, line map[string]any) {
	t.Helper()
	if line["topic"] != "in-topic" || line["partition"] != float64(3) || line["offset"] != float64(42) {
		t.Errorf("topic/partition/offset = %v/%v/%v", line["topic"], line["partition"], line["offset"])
	}
}

func TestHandleRecordValidStoresWithRecordTimestamp(t *testing.T) {
	var buf bytes.Buffer
	store := messaging.NewStoreWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := newTestConsumer(store, &buf)
	rec := testRecord(messaging.EncodeRecordValue("bob", "héllo secret-text"))

	c.handleRecord(context.Background(), rec)

	got := store.Get("bob")
	want := []messaging.Message{{Text: "héllo secret-text", Time: rec.Timestamp}}
	if !slices.Equal(got, want) {
		t.Fatalf("store = %v, want %v", got, want)
	}
	lines := logLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1: %s", len(lines), buf.String())
	}
	line := lines[0]
	if line["level"] != "INFO" || line["msg"] != "message received" {
		t.Errorf("level/msg = %v/%v", line["level"], line["msg"])
	}
	if line["user"] != "bob" || line["length"] != float64(17) {
		t.Errorf("user/length = %v/%v", line["user"], line["length"])
	}
	assertRecordFields(t, line)
	if strings.Contains(buf.String(), "secret-text") {
		t.Errorf("log leaks the message text: %s", buf.String())
	}
}

func TestHandleRecordInvalidIsDroppedAndWarned(t *testing.T) {
	long := strings.Repeat("x", messaging.MaxTextLen+1)
	cases := []struct {
		name, value, reason string
	}{
		{"not json", `secret-value`, messaging.ReasonInvalidJSON},
		{"not object", `["secret-value"]`, messaging.ReasonInvalidJSON},
		{"invalid utf8", "{\"user\":\"bob\",\"text\":\"secret-value\xff\"}", messaging.ReasonInvalidUTF8},
		{"missing text", `{"user":"bob","note":"secret-value"}`, messaging.ReasonMissingField},
		{"wrong type", `{"user":"bob","text":["secret-value"]}`, messaging.ReasonWrongType},
		{"empty user", `{"user":"","text":"secret-value"}`, messaging.ReasonUserLength},
		{"long text", `{"user":"bob","text":"secret-value` + long + `"}`, messaging.ReasonTextLength},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			store := messaging.NewStore()
			c := newTestConsumer(store, &buf)

			c.handleRecord(context.Background(), testRecord([]byte(tc.value)))

			if got := store.Get("bob"); len(got) != 0 {
				t.Errorf("store has %v, want nothing", got)
			}
			lines := logLines(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("log lines = %d, want 1: %s", len(lines), buf.String())
			}
			line := lines[0]
			if line["level"] != "WARN" || line["msg"] != "dropped invalid record" {
				t.Errorf("level/msg = %v/%v", line["level"], line["msg"])
			}
			if line["reason"] != tc.reason {
				t.Errorf("reason = %v, want %s", line["reason"], tc.reason)
			}
			assertRecordFields(t, line)
			if strings.Contains(buf.String(), "secret-value") {
				t.Errorf("log leaks the record value: %s", buf.String())
			}
		})
	}
}

func TestHandleRecordNilValueDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	c := newTestConsumer(messaging.NewStore(), &buf)

	c.handleRecord(context.Background(), testRecord(nil))

	if !strings.Contains(buf.String(), "dropped invalid record") {
		t.Errorf("nil value not dropped: %s", buf.String())
	}
}

func TestConnectDoesNotSubscribeBeforeRun(t *testing.T) {
	cl := newTestClient(t, baseConfig())

	if got := cl.GetConsumeTopics(); len(got) != 0 {
		t.Errorf("consume topics after Connect = %v, want none until Run", got)
	}
}

func runConsumer(t *testing.T, ctx context.Context, cl *kgo.Client) error {
	t.Helper()
	c := NewConsumer(cl, "in-topic", messaging.NewStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

func TestRunSubscribesAndReturnsNilOnContextCancel(t *testing.T) {
	cl := newTestClient(t, baseConfig())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := runConsumer(t, ctx, cl); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
	if got := cl.GetConsumeTopics(); !slices.Equal(got, []string{"in-topic"}) {
		t.Errorf("consume topics = %v, want [in-topic]", got)
	}
}

func TestRunReturnsNilWhenClientClosed(t *testing.T) {
	cl, err := Connect(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(100*time.Millisecond, cl.Close)

	if err := runConsumer(t, context.Background(), cl); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
}

func TestProcessWarnsOnFetchErrorAndMarksEveryRecord(t *testing.T) {
	var buf bytes.Buffer
	store := messaging.NewStore()
	c := newTestConsumer(store, &buf)
	var marked []int64
	c.mark = func(rs ...*kgo.Record) {
		for _, r := range rs {
			marked = append(marked, r.Offset)
		}
	}
	valid := testRecord(messaging.EncodeRecordValue("bob", "hi"))
	invalid := testRecord([]byte("secret-value"))
	invalid.Offset = 43
	fetches := kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic: "in-topic",
		Partitions: []kgo.FetchPartition{
			{Partition: 1, Err: errors.New("broker gone")},
			{Partition: 3, Records: []*kgo.Record{valid, invalid}},
		},
	}}}}

	c.process(context.Background(), fetches)

	if !slices.Equal(marked, []int64{42, 43}) {
		t.Errorf("marked offsets = %v, want [42 43]", marked)
	}
	if got := store.Get("bob"); len(got) != 1 {
		t.Errorf("store = %v, want the valid record", got)
	}
	var warned bool
	for _, line := range logLines(t, &buf) {
		if line["msg"] == "kafka fetch error" {
			warned = line["level"] == "WARN" && line["topic"] == "in-topic" && line["partition"] == float64(1)
		}
	}
	if !warned {
		t.Errorf("no WARN fetch error line: %s", buf.String())
	}
}

func TestProcessResubscribesOnceOnUnknownTopicID(t *testing.T) {
	var buf bytes.Buffer
	c := newTestConsumer(messaging.NewStore(), &buf)
	c.mark = func(...*kgo.Record) {}
	calls := 0
	c.resubscribe = func() { calls++ }
	fetches := kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic: "in-topic",
		Partitions: []kgo.FetchPartition{
			{Partition: 0, Err: kerr.UnknownTopicID},
			{Partition: 1, Err: kerr.UnknownTopicID},
		},
	}}}}

	c.process(context.Background(), fetches)

	if calls != 1 {
		t.Fatalf("resubscribe calls = %d, want 1 per poll", calls)
	}
	var warned bool
	for _, line := range logLines(t, &buf) {
		if line["msg"] == "kafka topic recreated, resubscribing" {
			warned = line["level"] == "WARN" && line["topic"] == "in-topic"
		}
	}
	if !warned {
		t.Errorf("no WARN resubscribe line: %s", buf.String())
	}
}

func TestProcessDoesNotResubscribeOnOtherErrors(t *testing.T) {
	c := newTestConsumer(messaging.NewStore(), &bytes.Buffer{})
	calls := 0
	c.resubscribe = func() { calls++ }
	fetches := kgo.Fetches{{Topics: []kgo.FetchTopic{
		{Topic: "in-topic", Partitions: []kgo.FetchPartition{{Partition: 0, Err: kerr.NotLeaderForPartition}}},
		{Topic: "other-topic", Partitions: []kgo.FetchPartition{{Partition: 0, Err: kerr.UnknownTopicID}}},
	}}}

	c.process(context.Background(), fetches)

	if calls != 0 {
		t.Errorf("resubscribe calls = %d, want 0", calls)
	}
}

func TestResubscribeReAddsInputTopic(t *testing.T) {
	cl := newTestClient(t, baseConfig())
	c := NewConsumer(cl, "in-topic", messaging.NewStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	cl.AddConsumeTopics("in-topic")

	c.resubscribe()

	if got := cl.GetConsumeTopics(); !slices.Equal(got, []string{"in-topic"}) {
		t.Errorf("consume topics after resubscribe = %v, want [in-topic]", got)
	}
}

func TestHandleRecordTextAtLimitIsStored(t *testing.T) {
	var buf bytes.Buffer
	store := messaging.NewStoreWithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := newTestConsumer(store, &buf)
	text := strings.Repeat("😀", messaging.MaxTextLen)

	c.handleRecord(context.Background(), testRecord(messaging.EncodeRecordValue("bob", text)))

	if got := store.Get("bob"); len(got) != 1 || got[0].Text != text {
		t.Fatalf("256-code-point record not stored: %v", got)
	}
}
