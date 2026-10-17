# ADR-005: Configurable character denylist, applied to username and text

## Status
Accepted

## Context
Special characters must be replaced with `_` before a message is published to Kafka. The exact
set is defined externally (by the downstream/pipeline side) and will be delivered later.

## Decision
- Denylist: every character (Unicode code point) present in the list is replaced with `_`;
  all other characters pass through unchanged.
- List is configuration: Helm value `censor.chars` -> env var `CENSOR_CHARS` (string, each code
  point is one denied character). Not hard-coded.
- Provisional default until the real list arrives: `$!~` (examples from the requirements).
  PENDING: replace with the delivered list.
- Applied to both username and text, server-side, before publishing. Replacement is 1:1, so
  length in code points is unchanged.

## Alternatives
- Allowlist — safer against unlisted chars, but list is externally defined as a denylist.
- Hard-coded list — every change would need a release through the air-gapped pipeline.
- Censor text only — username is also in the Kafka JSON, same downstream risk.

## Consequences
- Characters not on the list pass through; correctness depends on the delivered list.
- Code/commands typed by users get mangled where they contain denied chars (accepted).
- Received messages are not re-censored; UI renders them as plain text (no HTML interpretation).
