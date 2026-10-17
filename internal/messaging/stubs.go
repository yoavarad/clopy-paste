package messaging

import (
	"context"
)

// NewPublisher returns the default Publisher implementation (business logic TODO).
func NewPublisher() Publisher {
	return &publisherStub{}
}

type publisherStub struct{}

// Publish implements Publisher.
func (*publisherStub) Publish(ctx context.Context, user string, text string) error {
	panic("not implemented") // ydk:contract:messaging/Publisher.Publish: Produce one ydk:event:messaging/message-record to the output topic (acks=all, idempotent) and block until the broker...
}
