// Package kafka adapts ydk:ext:kafka/cluster, ydk:ext:kafka/external-pipeline to domain contracts using franz-go.
package kafka

import (
	"context"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"

	"clopy-paste/internal/messaging"
)

// NewClient creates a franz-go client for the seed brokers; pass extra kgo options for TLS/SASL.
func NewClient(seeds []string, opts ...kgo.Opt) (*kgo.Client, error) {
	return kgo.NewClient(append([]kgo.Opt{kgo.SeedBrokers(seeds...)}, opts...)...)
}

// Connect creates the client main.go wires into the adapters, seeded from KAFKA_BOOTSTRAP_SERVERS
// (comma-separated host:port, default localhost:9092); add TLS/SASL options here as the ext requires.
func Connect() (*kgo.Client, error) {
	var seeds []string
	for _, seed := range strings.Split(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"), ",") {
		if seed = strings.TrimSpace(seed); seed != "" {
			seeds = append(seeds, seed)
		}
	}
	if len(seeds) == 0 {
		seeds = []string{"localhost:9092"}
	}
	return NewClient(seeds)
}

// Publisher implements messaging.Publisher (ydk:contract:messaging/Publisher) on a franz-go client.
type Publisher struct {
	client *kgo.Client
}

var _ messaging.Publisher = (*Publisher)(nil)

// NewPublisher wraps an existing client.
func NewPublisher(client *kgo.Client) *Publisher {
	return &Publisher{client: client}
}

// Publish: Produce one ydk:event:messaging/message-record to the output topic (acks=all, idempotent) and block until the broker...
func (recv *Publisher) Publish(ctx context.Context, user string, text string) error {
	panic("not implemented") // ydk:contract:messaging/Publisher.Publish via franz-go
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
