package kafka

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	"clopy-paste/internal/messaging"
)

// Consumer polls the input topic (ydk:ext:kafka/cluster endpoint consume) into the Store.
type Consumer struct {
	client *kgo.Client
	topic  string
	store  messaging.Store
	logger *slog.Logger
	mark   func(...*kgo.Record) // client.MarkCommitRecords; replaced in tests
	// resubscribe purges and re-adds the input topic so a recreated topic (new topic ID) is
	// rediscovered; franz-go never adopts the new ID by itself. Replaced in tests.
	resubscribe func()
}

// NewConsumer wraps a client from Connect; Run subscribes it to topic. logger is slog.Default when nil.
func NewConsumer(client *kgo.Client, topic string, store messaging.Store, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Consumer{client: client, topic: topic, store: store, logger: logger, mark: client.MarkCommitRecords}
	c.resubscribe = func() {
		client.PurgeTopicsFromConsuming(topic)
		client.AddConsumeTopics(topic)
	}
	return c
}

// Run subscribes to the input topic (joining the consumer group) and polls until ctx is done
// or the client is closed, then returns nil. Fetch errors are logged at WARN and polling
// continues. Every record, stored or dropped, is marked for the 5 s auto-commit; on shutdown
// the caller commits the remaining marks (CommitMarkedOffsets) and leaves the group.
func (recv *Consumer) Run(ctx context.Context) error {
	recv.client.AddConsumeTopics(recv.topic)
	for {
		fetches := recv.client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		recv.process(ctx, fetches)
	}
}

// process logs each fetch error at WARN, then handles and marks every record. An
// UNKNOWN_TOPIC_ID error on the input topic (deleted and recreated; franz-go surfaces it only
// once it persists) resubscribes the topic once per poll so consumption resumes without a restart.
func (recv *Consumer) process(ctx context.Context, fetches kgo.Fetches) {
	recreated := false
	fetches.EachError(func(topic string, partition int32, err error) {
		recv.logger.WarnContext(ctx, "kafka fetch error", "topic", topic, "partition", partition, "error", err)
		if topic == recv.topic && errors.Is(err, kerr.UnknownTopicID) {
			recreated = true
		}
	})
	if recreated {
		recv.logger.WarnContext(ctx, "kafka topic recreated, resubscribing", "topic", recv.topic)
		recv.resubscribe()
	}
	fetches.EachRecord(func(rec *kgo.Record) {
		recv.handleRecord(ctx, rec)
		recv.mark(rec)
	})
}

// handleRecord decodes one record (key ignored) and stores it with the record timestamp, or
// drops it with a WARN (ydk:error:messaging/bad-incoming-record). The value is never logged.
func (recv *Consumer) handleRecord(ctx context.Context, rec *kgo.Record) {
	user, text, err := messaging.DecodeRecordValue(rec.Value)
	if err != nil {
		reason := "invalid_record"
		var de *messaging.DecodeError
		if errors.As(err, &de) {
			reason = de.Reason
		}
		recv.logger.WarnContext(ctx, "dropped invalid record",
			"reason", reason,
			"topic", rec.Topic,
			"partition", rec.Partition,
			"offset", rec.Offset,
		)
		return
	}
	recv.store.Add(user, messaging.Message{Text: text, Time: rec.Timestamp})
	recv.logger.InfoContext(ctx, "message received",
		"user", user,
		"length", utf8.RuneCountInString(text),
		"topic", rec.Topic,
		"partition", rec.Partition,
		"offset", rec.Offset,
	)
}
