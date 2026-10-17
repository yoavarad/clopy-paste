# ADR-003: UI refreshes received messages by polling

## Status
Accepted

## Context
Received-messages panel must show new messages from the other side. Use case: one developer
moving text between two VMs; seconds of latency acceptable.

## Decision
Page polls `GET /api/messages?user=<name>` every 2 seconds and re-renders the panel.

## Alternatives
- Server-Sent Events — instant, stdlib-only, but needs heartbeat/route-timeout handling for the
  OpenShift router (30s idle default) and a subscriber fan-out registry.
- WebSocket — needs a non-stdlib Go dependency; violates ADR-002 dependency rule.

## Consequences
- Up to ~2s display latency.
- Stateless request/response API; trivial to test with curl.
