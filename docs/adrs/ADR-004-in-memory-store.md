# ADR-004: In-memory message store, single replica

## Status
Accepted

## Context
Each instance keeps up to 100 users x 100 received messages. Need to decide durability across
pod restarts.

## Decision
Store lives only in process memory. Deployment runs exactly 1 replica with `Recreate` strategy
(no overlap of old/new pod consuming the same group). Kafka consumer uses a consumer group and
resumes from the committed offset after restart.

## Alternatives
- Rebuild from Kafka on startup (read input topic from earliest) — survives restarts but depends
  on topic retention owned by the external pipeline; more code.
- Persist to PVC file — needs volume, file format, corruption handling; RWO complicates rollout.

## Consequences
- Restart clears received history; messages received during downtime are delivered after
  restart (offset resume), older ones are gone.
- No PVC in the Helm chart. Horizontal scaling not supported (replicas fixed at 1).
