# Messaging

How a message travels from one side to the other: send path, Kafka record, receive path,
store. Decisions: ADR-001, ADR-004, ADR-005, ADR-006, ADR-007, ADR-008, ADR-013.

## Send Flow

1. The page posts the username and text to [ydk:route:api/send-message].
2. The handler caps the body size and decodes it. A malformed body returns
   [ydk:error:api/invalid-json].
3. The handler validates both fields in Unicode code points before censoring:
   [ydk:req:messaging/user-length] and [ydk:req:messaging/text-length]. A violation returns
   [ydk:error:api/validation-failed]; nothing is published and nothing is truncated (ADR-006).
4. The handler censors username and text with [ydk:contract:messaging/Censor]
   ([ydk:req:messaging/censor-denylist]). Replacement is one character for one character,
   so the limits checked in step 3 still hold.
5. The handler calls [ydk:contract:messaging/Publisher] with a 5 s deadline. The Kafka
   adapter produces one [ydk:event:messaging/message-record] to the output topic with
   acks=all and an idempotent producer, and waits for the broker ack.
6. Ack received: 202. Failure or deadline: [ydk:error:api/kafka-unavailable]; the page keeps
   the text ([ydk:req:ui/keep-text-on-error]).
7. The send path never touches the local store ([ydk:req:messaging/sent-not-stored]).
   The log line carries username, length, topic, partition, offset and duration, never the
   text ([ydk:req:system/no-text-in-logs]).

## Kafka Record

The record shape is defined once in [ydk:entity:messaging/KafkaRecord] and its delivery
rules in [ydk:event:messaging/message-record] (ADR-007):

- Key: the censored username, so all records for one username share a partition and keep
  produce order as long as the pipeline preserves keys.
- Value: a JSON object with exactly the username and text fields. No headers. No timestamp in
  the payload; the receiver displays the Kafka record timestamp.
- Delivery is at-least-once end to end. The producer is idempotent; the consumer commits
  after processing, so a crash between storing and committing shows that message twice.

## Receive Flow

1. The Kafka adapter consumes the input topic in the consumer group named by
   KAFKA_CONSUMER_GROUP (default clopy-paste-<INSTANCE_NAME>; prod supplies its own). A new
   group with no committed offset starts at the latest offset, so records produced before
   the first start are not shown; afterwards it resumes from the committed offset (ADR-004,
   ADR-017).
2. Each record is decoded and validated against the same limits as the send path
   ([ydk:req:messaging/drop-invalid-incoming]). Unknown extra JSON fields are ignored; the
   record key is ignored (the username comes from the value).
3. Invalid record: dropped, logged at WARN with topic, partition, offset and reason
   ([ydk:error:messaging/bad-incoming-record]), offset committed, consumption continues.
4. Valid record: a [ydk:entity:messaging/Message] is built from the text and the record
   timestamp and added to the store under the username. Received text is not censored again.
5. The offset is marked after the store call returns; marked offsets are committed every
   5 s and on shutdown.

## Store

[ydk:entity:messaging/MessageStore] lives in process memory only; a restart empties it
(ADR-004). It holds up to 100 [ydk:entity:messaging/UserMessages] buckets, one per username,
with operations defined in [ydk:contract:messaging/Store]. One mutex guards the map and the
recency list; every operation is O(1) apart from copying at most 100 messages on read.

Rules:
- Per-username cap of 100 messages; the oldest is dropped first
  ([ydk:req:messaging/fifo-per-user]).
- Recency is refreshed by receiving a message for the username and by a read for an existing
  username ([ydk:req:messaging/lru-touch]). Sending does not refresh recency.
- A 101st username evicts the least-recently-used username with all its messages
  ([ydk:req:messaging/lru-eviction]).
- A read for an unknown username returns an empty list and creates nothing
  ([ydk:req:messaging/unknown-user-not-created]), so arbitrary lookups cannot evict real users.
- Reads return messages newest first.

## Censoring

[ydk:req:messaging/censor-denylist] holds the rule. The denylist comes from configuration
([ydk:crosscut:ops/environment]); the provisional default is `$!~` and is PENDING replacement
by the externally delivered list. Censoring runs on the sending instance for username and
text. The read path censors the queried username with the same denylist, so a developer
typing "bob$" on both sides reads the messages stored under "bob_". Both instances must run
with the same denylist for this to hold.

## Kafka Connection

[ydk:ext:kafka/cluster] documents both connection modes (ADR-008). SASL_SSL uses
SCRAM-SHA-512 with credentials from a Kubernetes Secret ([ydk:req:ops/secrets-by-reference])
and an optional mounted CA file; PLAINTEXT is for development. Any other value stops the
process at startup with [ydk:error:ops/invalid-config]. The franz-go client reconnects on its
own. Readiness does not reflect broker reachability (ADR-016,
[ydk:crosscut:system/health-checks]); Kafka problems surface only as failed sends.

## E2E Walkthrough

Setup: instance side A (output a.out, input b.out) and side B (output b.out, input a.out),
default denylist.

1. On side A the developer enters username "bob" and text "kubectl get pods -n foo$", then
   presses Send.
2. Side A validates (3 and 24 code points), censors to "bob" / "kubectl get pods -n foo_",
   produces a [ydk:entity:messaging/KafkaRecord] with key "bob" to a.out, receives the ack and
   returns 202. The textarea clears; status "sent". Side A's store is unchanged.
3. Side B's consumer reads the record from a.out, validates it, and the store creates the
   "bob" bucket (store had 37 users, now 38) holding one [ydk:entity:messaging/Message] with
   the record timestamp. Offset committed.
4. Side B's page has "bob" in the username field and polls [ydk:route:api/get-messages]
   every 2 s. The next poll returns the message; "bob" becomes most recently used; the
   received list shows the local time, the text and a copy button.
5. The developer on side B clicks copy, edits the output, and replies as "bob" with
   "done!". Side B censors to "done_", produces to b.out; side A's consumer stores it under
   "bob"; side A's page shows it on its next poll.
6. Eviction: side B holds 100 usernames; "carol" is least recently used. A record for the new
   username "dave" arrives. The store removes "carol" and all her messages, then creates
   "dave". A read for "carol" now returns an empty list and does not recreate her.

Every step maps to a manifest field: limits on the record and bucket, recency on the bucket,
timestamp on the record, key on the record. No step needs data the model lacks.

## Adversarial Review

| Challenge | Mechanism in this spec |
|---|---|
| Kafka down for hours | Send returns [ydk:error:api/kafka-unavailable] after 5 s and the page shows "Kafka: disconnected" with the text kept. Readiness and liveness stay 200 (ADR-016): the page stays reachable and the pod is not restarted. Consumer reconnects and resumes from the committed offset. |
| External pipeline down | Sends still succeed (ack from our cluster); messages arrive when it resumes. Not detectable here; documented in [ydk:ext:kafka/external-pipeline]. |
| Poison record on input topic | Dropped and logged, offset committed ([ydk:req:messaging/drop-invalid-incoming]); never blocks the partition. |
| Flood of distinct usernames | Store capped at 100 users x 100 messages; memory bounded per [ydk:nfr:system/memory]. Real users can be evicted by a flood; accepted (trusted network, ADR-014). |
| Lookup of many unknown names | Reads never create entries ([ydk:req:messaging/unknown-user-not-created]). |
| Oversized or malformed request | Body cap 16384 bytes, code-point limits, JSON decode errors: all 400 ([ydk:crosscut:system/http-conventions]). |
| HTML/script in text | Rendered with textContent only ([ydk:req:ui/plain-text-rendering]). |
| Emoji and multi-byte text | Limits counted in code points on the server; the browser counter may differ and the server decides ([ydk:req:messaging/text-length]). |
| Double-click Send | Button disabled while a request is in flight; a deliberate resend publishes again (no idempotency key, by design). |
| Concurrent receive and read | One mutex in the store; race detector runs in the unit tests ([ydk:test:messaging/store.unit]). |
| Crash between store insert and offset commit | Message re-delivered and shown twice; accepted (at-least-once). |
| Pod restart | History lost (ADR-004); records produced during downtime are consumed after restart. |
| Two pods consuming one group during rollout | Prevented by one replica and Recreate strategy ([ydk:req:ops/single-replica]). |
| Same topic configured as input and output | Not rejected (topic names are opaque, ADR-001); the instance would display its own sends. Chart values for each environment are reviewed in the deploy merge request. |
| 10x traffic | One developer per pair is the expected load; the [ydk:nfr:system/throughput] target leaves headroom above one send per second. |
