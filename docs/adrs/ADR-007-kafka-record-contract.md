# ADR-007: Kafka record contract

## Status
Accepted

## Context
Records cross an external pipeline between instances; the format is an external interface.
Requirement: simple JSON with a username field and a text field.

## Decision
- Key: username, UTF-8 bytes.
- Value: `{"user":"<username>","text":"<text>"}` — UTF-8 JSON, exactly these two string fields.
- Headers: none.
- No timestamp in payload; receiver uses the Kafka record timestamp for display.

## Alternatives
- No key — per-user ordering lost across partitions.
- Timestamp/origin fields in payload — beyond the two-field requirement.

## Consequences
- Per-user ordering preserved within the pipeline as long as it preserves keys.
- Receiver ignores unknown extra fields; missing/invalid `user`/`text` cause drop (ADR-006).
