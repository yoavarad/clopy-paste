# ADR-017: Consumer group, offset reset and operational defaults

## Status
Accepted

## Context
Prod dictates consumer group names (ACLs are per group), so a group derived only from
INSTANCE_NAME may not be allowed. A new group needs a start position. Several operational
defaults chosen during spec writing also need recording.

## Decision
- Consumer group: env var `KAFKA_CONSUMER_GROUP` (Helm value `kafka.consumerGroup`), default
  `clopy-paste-<INSTANCE_NAME>`. Prod supplies its own value.
- A group with no committed offset starts at `latest`. Existing groups resume from the
  committed offset (ADR-004).
- Publish deadline: 5 s per send; failure or timeout -> 503.
- Request body cap: 16 KiB (`http.MaxBytesReader`); larger -> 400.
- Read path censors the queried username with the same denylist as the send path (ADR-005),
  so a username typed identically on both sides matches.
- `GOMEMLIMIT=100MiB` set in the chart, below the 128Mi container limit.
- HTTP port fixed at 8080, not configurable.
- Success bodies: send 202 `{}`; health and readiness 200 `{"status":"ok"}`.
- Local environment: release A uses SASL_SSL (SCRAM-SHA-512 over TLS), release B uses
  PLAINTEXT, so every e2e run exercises both modes; ingress-nginx runs in kind and e2e reaches
  each release through its Ingress with a Host header.

## Alternatives
- Group fixed to `clopy-paste-<INSTANCE_NAME>` — breaks where prod assigns group names.
- `earliest` reset — replays whatever the pipeline retains on first start, including stale
  messages from previous deployments.
- Configurable port, deadline and body cap — no consumer for these knobs.

## Consequences
- Records produced before an instance's group first exists are never shown; e2e must warm up
  both directions before asserting.
- Renaming the group in prod restarts consumption at `latest` (records in between are skipped).
- Chart values schema gains `kafka.consumerGroup` (optional).
