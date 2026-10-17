// Package kafka adapts ydk:ext:kafka/cluster, ydk:ext:kafka/external-pipeline to domain contracts using franz-go.
package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"
	"unicode/utf8"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"clopy-paste/internal/config"
	"clopy-paste/internal/messaging"
)

const (
	// publishTimeout caps one send (ADR-017); a shorter caller deadline wins.
	publishTimeout = 5 * time.Second
	// autoCommitInterval is how often marked offsets are committed (ADR-017).
	autoCommitInterval = 5 * time.Second
)

// NewClient creates a franz-go client for the seed brokers; pass extra kgo options for TLS/SASL.
func NewClient(seeds []string, opts ...kgo.Opt) (*kgo.Client, error) {
	return kgo.NewClient(append([]kgo.Opt{kgo.SeedBrokers(seeds...)}, opts...)...)
}

// Connect creates the client main.go wires into the adapters from the loaded config:
// security mode (ADR-008), acks=all idempotent producer to the output topic, and the
// consumer group on the input topic starting at the end (ADR-017).
func Connect(cfg config.Config) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.DefaultProduceTopic(cfg.KafkaOutputTopic),
		kgo.ConsumerGroup(cfg.KafkaConsumerGroup),
		kgo.ConsumeTopics(cfg.KafkaInputTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(autoCommitInterval),
	}
	switch cfg.KafkaSecurityProtocol {
	case "PLAINTEXT":
	case "SASL_SSL":
		tlsCfg, err := tlsConfig(cfg.KafkaCaFile)
		if err != nil {
			return nil, err
		}
		auth := scram.Auth{User: cfg.KafkaSaslUsername, Pass: cfg.KafkaSaslPassword}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg), kgo.SASL(auth.AsSha512Mechanism()))
	default:
		return nil, fmt.Errorf("kafka: unsupported KAFKA_SECURITY_PROTOCOL %q", cfg.KafkaSecurityProtocol)
	}
	return NewClient(cfg.KafkaBootstrapServers, opts...)
}

// tlsConfig trusts the PEM CA bundle at caFile, or the system roots when caFile is empty.
func tlsConfig(caFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return cfg, nil
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("kafka: read KAFKA_CA_FILE: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("kafka: KAFKA_CA_FILE %q has no PEM certificates", caFile)
	}
	cfg.RootCAs = pool
	return cfg, nil
}

// Publisher implements messaging.Publisher (ydk:contract:messaging/Publisher) on a franz-go client.
type Publisher struct {
	client *kgo.Client
	logger *slog.Logger
}

var _ messaging.Publisher = (*Publisher)(nil)

// NewPublisher wraps an existing client; logger (slog.Default when nil) receives the "message sent" line.
func NewPublisher(client *kgo.Client, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{client: client, logger: logger}
}

// newRecord builds one ydk:event:messaging/message-record (ADR-007): key = user, JSON value,
// no headers, topic left empty so DefaultProduceTopic applies.
func newRecord(user, text string) *kgo.Record {
	return &kgo.Record{Key: []byte(user), Value: messaging.EncodeRecordValue(user, text)}
}

// Publish produces one record to the output topic (acks=all, idempotent) and blocks until the
// broker acknowledges it, ctx is done, or publishTimeout passes. Failures wrap
// messaging.ErrKafkaUnavailable. The text is never logged.
func (recv *Publisher) Publish(ctx context.Context, user string, text string) error {
	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	start := time.Now()
	rec, err := recv.client.ProduceSync(ctx, newRecord(user, text)).First()
	if err != nil {
		return fmt.Errorf("%w: %w", messaging.ErrKafkaUnavailable, err)
	}
	recv.logger.InfoContext(ctx, "message sent",
		"user", user,
		"length", utf8.RuneCountInString(text),
		"topic", rec.Topic,
		"partition", rec.Partition,
		"offset", rec.Offset,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// Consumer polls the input topic (ydk:ext:kafka/cluster endpoint consume).
type Consumer struct {
	client *kgo.Client
}

// NewConsumer wraps an existing client configured with ConsumeTopics/ConsumerGroup.
func NewConsumer(client *kgo.Client) *Consumer {
	return &Consumer{client: client}
}

// Run polls until ctx is done: Fetch records from the other side for the in-memory store
func (recv *Consumer) Run(ctx context.Context) error {
	panic("not implemented") // ydk:ext:kafka/cluster.consume: poll, validate, hand records to the domain
}
