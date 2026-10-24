# Task 98 orchestrator progress

## Plan (done)
- franz-go v1.22.1 never adopts a recreated topic's new ID; documented fix = purge + re-add (client.go PurgeTopicsFromClient doc, source.go cursor comment).
- ADR-016 forbids Kafka-gated readiness -> choose in-process recovery, no restart.
- Consumer.process: on fetch error errors.Is(kerr.UnknownTopicID) for input topic -> PurgeTopicsFromConsuming + AddConsumeTopics (once per poll), WARN log.
- Publisher.Publish: on produce error errors.Is(kerr.UnknownTopicID) -> PurgeTopicsFromProducing(topic), WARN log; that request still 503, next one rediscovers.
- Unit tests with injected errors in consumer_test.go / kafka_test.go. Integration test skipped (needs cluster; not cheap locally).

## Build (done)
- consumer.go/kafka.go recovery + 5 unit tests; messaging.md failure-mode row. go test ./... green, vet -tags integration ok.

## Review (round 1, done)
0 bugs, 4 risks accepted (documented in PR body):
- group-mode purge+re-add causes two rebalances; no cooldown (re-trigger needs >5 new UNKNOWN_TOPIC_ID fails, unlikely).
- narrow purges leave metaCache/id2t entries; those only serve admin sharded requests, not produce/consume.
- Publisher purge runs via blockingMetadataFn after the 5 s deadline, unbounded (normally fast).
- concurrent failing Publishes each purge; transient churn, converges.
