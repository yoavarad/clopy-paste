# ADR-006: Reject over-limit input; drop and log invalid incoming records

## Status
Accepted

## Context
Limits: username 1..128 chars, text 1..256 chars. Counted in Unicode code points, not bytes.
Need behavior on violation for both outgoing (user input) and incoming (Kafka) messages.

## Decision
- Outgoing: UI sets `maxlength`; server re-validates and returns HTTP 400 with an error body;
  nothing is published. No truncation.
- Incoming: records that are not valid JSON, miss a field, or violate a limit are dropped,
  logged at WARN with topic/partition/offset, offset committed, consumption continues.

## Alternatives
- Truncate outgoing — silently loses the tail of pasted content.
- Crash/retry on bad incoming record — one poison record blocks the consumer forever.

## Consequences
- Server is the single authority on limits (browser `maxlength` counts UTF-16 units, may differ
  for emoji; server check wins).
- Invalid incoming records are visible only in logs.
