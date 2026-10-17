# ADR-016: Readiness independent of Kafka

## Status
Accepted

## Context
A Kafka-gated readiness probe removes the pod from Service endpoints during a Kafka outage.
The OpenShift router / ingress controller then answers every request, including the page,
with its own non-JSON 503. The user loses the UI entirely and cannot see already received
messages or the reason for the failure. The UI rule "keep the text on a failed send" assumes
the page is reachable.

## Decision
- `/readyz` returns 200 `{"status":"ok"}` once the HTTP server listens; the process only
  gets there after configuration validated. It returns 503 `{"error":"shutting down"}` after
  SIGTERM. It never checks Kafka.
- `/healthz` stays a pure liveness check.
- Kafka state reaches the UI only through the send response: Kafka unavailable or no ack
  within 5 s -> 503 `{"error":"kafka unavailable, message not sent"}`; the UI status line
  shows "Kafka: disconnected" and keeps the text in the textarea.
- No new route or response field for Kafka state. The Publisher port has no Ping method.
- The Kafka client is created without blocking on broker connection.

## Alternatives
- Kafka-gated readiness (previous draft) — page unreachable during outages; router 503 bodies
  are not JSON.
- `kafka` field in the `/readyz` body read by the UI on every poll — an extra request per
  poll cycle and a Ping implementation, for information the user only needs when sending.
- `kafka` field in the received-messages response — changes the array response to an object.

## Consequences
- During an outage the page and received list stay reachable; sends fail with a clear status.
- The UI does not show Kafka state until the user sends.
- Kubernetes does not detect Kafka outages through probes; platform Kafka monitoring covers it.
- Startup NFR (ready <= 10 s) no longer depends on Kafka reachability.
