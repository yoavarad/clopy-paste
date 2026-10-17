package messaging

import "errors"

// Sentinel errors from ydk:error manifests.
var (
	// ErrKafkaUnavailable -- ydk:error:api/kafka-unavailable (KAFKA_UNAVAILABLE)
	ErrKafkaUnavailable = errors.New("kafka unavailable, message not sent")
	// ErrBadIncomingRecord -- ydk:error:messaging/bad-incoming-record (BAD_INCOMING_RECORD)
	ErrBadIncomingRecord = errors.New("dropped invalid record: ${reason}")
)
