package messaging

import (
	"context"
)

// FakePublisher is a configurable Publisher for unit tests; unset funcs return zero values.
type FakePublisher struct {
	PublishFn func(context.Context, string, string) error
}

var _ Publisher = (*FakePublisher)(nil)

// Publish calls PublishFn when set.
func (recv *FakePublisher) Publish(ctx context.Context, user string, text string) error {
	if recv.PublishFn != nil {
		return recv.PublishFn(ctx, user, text)
	}
	var out error
	return out
}
