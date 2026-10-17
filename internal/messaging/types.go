package messaging

import (
	"time"
)

// KafkaRecord -- ydk:entity:messaging/KafkaRecord: The Kafka record exchanged between instances through the external pipeline (ADR-007).
type KafkaRecord struct {
	Key       []byte    `json:"key"`       // Partitioning key; preserves per-user ordering
	Value     []byte    `json:"value"`     // Payload, e.g. {"user":"bob","text":"hello_"}
	Headers   []any     `json:"headers"`   // Kafka record headers
	Timestamp time.Time `json:"timestamp"` // Becomes Message.time on the receiving side
	Topic     string    `json:"topic"`     // Output topic on produce, input topic on consume
	Partition int       `json:"partition"` // Partition; logged in invalid-record warnings
	Offset    int       `json:"offset"`    // Offset; logged in invalid-record warnings; committed after processing
}

// Message -- ydk:entity:messaging/Message: One message received from the other side via the input topic and held in the in-memory
type Message struct {
	Text string    `json:"text"` // Message body as received in the record value field 'text'
	Time time.Time `json:"time"` // When the record was produced or appended according to Kafka
}

// MessageStore -- ydk:entity:messaging/MessageStore: Process-wide in-memory store of received messages (ADR-004, ADR-013). One instance per
type MessageStore struct {
	Buckets            map[string]any `json:"buckets"`               // Username to bucket lookup
	Lru                []any          `json:"lru"`                   // Recency order of usernames
	MaxUsers           int            `json:"max_users"`             // User capacity
	MaxMessagesPerUser int            `json:"max_messages_per_user"` // Per-user message capacity
}

// UserMessages -- ydk:entity:messaging/UserMessages: The bucket of received messages for one username inside the store. Holds at most 100
type UserMessages struct {
	User       string `json:"user"`        // Username taken from the record value field 'user'
	Messages   []any  `json:"messages"`    // Received messages for this username
	LruElement any    `json:"lru_element"` // Recency position used for LRU eviction
}
