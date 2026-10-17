# ADR-012: JSON logs to stdout; Filebeat sidecar deferred

## Status
Accepted

## Context
Step 1: logs to stdout. Future: Filebeat sidecar in the same pod ships to central Logstash.
On OpenShift a sidecar cannot read another container's stdout (needs hostPath to
`/var/log/containers`, forbidden by SCC restricted-v2).

## Decision
- `log/slog` JSON handler to stdout. Level from `LOG_LEVEL` (DEBUG|INFO|WARN|ERROR, default INFO).
- Message text is never logged — only metadata (user, length, topic, partition, offset).
- No file logging or sidecar now.

## Alternatives
- Add `LOG_FILE` option now — unused code until Filebeat exists.
- Build disabled Filebeat sidecar now — untested without a Logstash; speculative.

## Consequences
- DEFERRED: Filebeat phase adds `LOG_FILE` (tee to file on shared `emptyDir`), Filebeat sidecar
  container + ConfigMap in the chart, and a Logstash mock for local/e2e.
- JSON lines need no grok parsing in Logstash.
