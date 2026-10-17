package messaging

import (
	"context"
)

// Censor -- ydk:contract:messaging/Censor: Pure function built from CENSOR_CHARS that replaces every code point in the denylist with '_'
type Censor interface {
	// Apply: Return s with each denied code point replaced by '_'; code-point length unchanged; all other code points unchanged
	Apply(s string) string
}

// Publisher -- ydk:contract:messaging/Publisher: Go interface (port) used by the send handler to publish a censored message. Implemented by
type Publisher interface {
	// Publish: Produce one ydk:event:messaging/message-record to the output topic (acks=all, idempotent) and block until the broker...
	Publish(ctx context.Context, user string, text string) error
}

// Store -- ydk:contract:messaging/Store: Operations on ydk:entity:messaging/MessageStore; concrete Go type, not an interface; tests
type Store interface {
	// Add: Append msg to the user's bucket. If the user is absent and 100 users exist, evict the least-recently-used user first,...
	Add(user string, msg Message)
	// Get: If the user exists, move it to most-recently-used and return a copy of its messages newest first. If not, return an e...
	Get(user string) []Message
}
