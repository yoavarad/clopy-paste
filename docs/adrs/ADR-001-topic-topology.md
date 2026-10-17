# ADR-001: Per-instance input/output topics, external pipeline links them

## Status
Accepted

## Context
Two instances must exchange messages bi-directionally. Both reach one Kafka cluster, but
between the sides there is an external pipeline (invisible to us) that moves records from one
instance's output topic to the other instance's input topic.

## Decision
Each instance is configured with exactly two topic names:
- `outputTopic` — where it produces messages typed by its user.
- `inputTopic` — where it consumes messages from the other side.

Topic names are opaque configuration (Helm values / env vars). The app makes no assumption
about naming or about how records travel between topics.

## Alternatives
- One shared topic with an `origin` field — rejected: adds a payload field beyond `{user, text}`
  and does not fit the external pipeline.
- Hard-coded directional pair (`a-to-b`, `b-to-a`) — rejected: prod topic names are dictated by
  the pipeline.

## Consequences
- Instance code is identical on both sides; only config differs.
- Local mock must emulate the pipeline (e.g. A.output == B.input, or a tiny forwarder).
- No echo filtering needed.
