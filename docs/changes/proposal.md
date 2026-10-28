# text-limit-256-auto-split

Text limit 256 + client-side auto-split.

## Why

The text limit drops from 1000 to 256 Unicode code points. Users who paste longer text
should not hit a hard rejection: the page splits long text into numbered chunks and sends
them as separate messages. The server stays the only authority on limits.

## What

### Task 1: limit 1000 -> 256

- `internal/messaging/message.go:13` `MaxTextLen = 256`.
- Tests at the new boundary (256 passes, 257 fails, including emoji / multi-byte code points):
  `internal/messaging/message_test.go`, `internal/httpapi/send_test.go`,
  `internal/adapters/kafka/consumer_test.go`, `test/e2e/round-trip.sh`.
- Manifests: `ydk:req:messaging/text-length`, `ydk:entity:messaging/Message`,
  `ydk:entity:messaging/KafkaRecord`, `ydk:error:api/validation-failed`,
  `ydk:component:ui/send-form`, `ydk:test:api/send-message.unit`,
  `ydk:test:system/round-trip.e2e`, `ydk:nfr:system/send-latency`, `ydk:nfr:system/throughput`.
- Docs: `docs/specs/api-and-ui.md`, `docs/project-rules.md:35`, `docs/planning/batch.yaml`.
- Done: go tests and e2e green at the new boundary.

### Task 2: client-side auto-split (depends on Task 1)

- New `web/split.js` with a pure exported `splitText(text)`, tested with `node --test`
  (`web/split.test.js`); the node test is wired into CI / verification.
- Remove the textarea `maxlength`; length counted in code points (`Array.from`).
- <= 256 code points -> one message, no marker.
- > 256 -> chunks prefixed `"(i/N) "`; chunk body <= 248 code points (prefix max 8, for `"(10/10) "`).
- Split at the last whitespace before the limit; hard cut only if there is none.
- Button: `Send` (<= 256) / `Send as X messages` (2..10) / disabled plus a "too long" note when > 10 chunks.
- Counter shows characters and chunk count.
- Chunks are sent sequentially, awaiting each 202 (same user key -> same partition -> ordered).
- Mid-failure: stop, put the unsent text back in the textarea, show `sent k/N — <error>`.
- Tests: boundary, emoji, no-whitespace, 10 vs 11 chunks.
- New ADR amending ADR-006 (client splits; server still rejects > 256). Update the
  `ydk:component:ui/send-form` manifest.

## Scope

### IN

- Text limit constant, its tests, manifests and docs.
- Client-side split logic, send-form behaviour, node test in CI, new ADR.

### OUT

- Server API shape, Kafka record contract, store (100 per user), 16384-byte body cap: unchanged.
- Server-side splitting or reassembly.
