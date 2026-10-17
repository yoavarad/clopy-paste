# Testing

Three levels, one rule: fake only the external boundary (Kafka), never internal types.
Tools: Go `testing` and `httptest` for unit and integration, POSIX shell and curl for e2e
(ADR-015). Pipeline placement: [ydk:crosscut:ops/ci-pipeline].

## Rules

1. Only the Kafka boundary is faked, and only in unit tests, through the small
   [ydk:contract:messaging/Publisher] interface. The store, censor, validation, config and
   HTTP router are always the real code.
2. Integration tests use a real Strimzi broker in kind. No Kafka mocks or in-process fakes.
3. e2e tests touch the system only through HTTP via the Ingress of each release, as a user's
   browser would.
4. All Go tests run with the race detector.
5. Coverage gate: >= 80% statement coverage on internal/censor, internal/store,
   internal/message, internal/httpapi and internal/config. The Kafka adapter is covered by
   integration tests and excluded from the gate.
6. Each requirement's acceptance criteria map to at least one test case.

## Unit Tests

| Spec | Covers |
|---|---|
| [ydk:test:messaging/censor.unit] | Denylist replacement, multi-byte characters, empty list, length preservation. |
| [ydk:test:messaging/store.unit] | FIFO per username, LRU eviction and touch, unknown-username reads, concurrency, worst-case heap size. |
| [ydk:test:api/send-message.unit] | Decode, limits at boundaries, censoring before publish, 503 mapping, publish deadline, method handling, no text in logs. |
| [ydk:test:api/get-messages.unit] | Order and shape, unknown username, censored lookup, parameter validation. |
| [ydk:test:api/health.unit] | Liveness, readiness, readiness during shutdown, in-flight completion, JSON 404. |
| [ydk:test:ops/config.unit] | Required variables, protocol values, SASL credential checks, censor default versus empty. |

## Integration Tests

[ydk:test:messaging/kafka-adapter.integration] runs behind a build tag against Strimzi in
kind: record shape on the wire, consume into a real store with the record timestamp, invalid
records dropped, offset resume after restart, both security modes, bad credentials, and the
[ydk:nfr:system/send-latency] and [ydk:nfr:system/throughput] measurements.

## E2E Tests

[ydk:test:system/round-trip.e2e] installs both releases and checks: both directions, censoring
end to end, limits, plain-text round trip of markup, LRU eviction with 101 usernames, the
[ydk:nfr:system/e2e-latency] target, Kafka outage behaviour, and restart with offset resume
([ydk:nfr:system/availability], [ydk:nfr:system/startup-time]).

## Not Tested Automatically

- OpenShift Route admission and SCC enforcement (no OpenShift locally, ADR-009); covered by
  the GitLab deploy and smoke stages.
- Clipboard behaviour in real browsers; checked by hand on both secure and insecure origins
  before a release.
