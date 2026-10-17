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

// NewStore returns the default Store implementation (business logic TODO).
func NewStore() Store {
	return &storeStub{}
}

type storeStub struct{}

// Add implements Store.
func (*storeStub) Add(user string, msg Message) {
	panic("not implemented") // ydk:contract:messaging/Store.Add: Append msg to the user's bucket. If the user is absent and 100 users exist, evict the least-recently-used user first,...
}

// Get implements Store.
func (*storeStub) Get(user string) []Message {
	panic("not implemented") // ydk:contract:messaging/Store.Get: If the user exists, move it to most-recently-used and return a copy of its messages newest first. If not, return an e...
}
