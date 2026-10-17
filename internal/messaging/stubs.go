package messaging

import (
	"context"
)

// NewCensor returns the default Censor implementation (business logic TODO).
func NewCensor() Censor {
	return &censorStub{}
}

type censorStub struct{}

// Apply implements Censor.
func (*censorStub) Apply(s string) string {
	panic("not implemented") // ydk:contract:messaging/Censor.Apply: Return s with each denied code point replaced by '_'; code-point length unchanged; all other code points unchanged
}

// NewPublisher returns the default Publisher implementation (business logic TODO).
func NewPublisher() Publisher {
	return &publisherStub{}
}

type publisherStub struct{}

// Publish implements Publisher.
func (*publisherStub) Publish(ctx context.Context, user string, text string) error {
	panic("not implemented") // ydk:contract:messaging/Publisher.Publish: Produce one ydk:event:messaging/message-record to the output topic (acks=all, idempotent) and block until the broker...
}
