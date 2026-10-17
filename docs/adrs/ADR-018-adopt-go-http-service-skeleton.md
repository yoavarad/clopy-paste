# ADR-018: Adopt the ydk go-http-service skeleton

## Status
Accepted

## Context
The service skeleton (wiring, config, HTTP routes, error format, health endpoints, Kafka
adapter, Helm chart, Makefile, scripts, CI) is generated from the component manifests by the
ydk `go-http-service` ignition pack (`ydk ignite`). The pack derives the package layout from
manifest namespaces: one `internal/<ns>/` package per namespace, `internal/httpapi/` for
routes and errors, `internal/config/` for the environment, `internal/adapters/<tech>/` per ext
technology and `web/` for pages. Our earlier module table split the messaging namespace into
`internal/message`, `internal/censor` and `internal/store`, with the adapter in
`internal/kafka`. Generated TODO stubs and later regeneration only line up with the pack's
layout.

## Decision
- Adopt the `go-http-service` ignition pack and its layout as generated:
  `cmd/clopy-paste`, `internal/config`, `internal/messaging` (Message, Censor, Store, the
  Publisher port and its fake), `internal/adapters/kafka`, `internal/httpapi`, `web/`.
- Keep the `messaging` manifest namespace; do not rename namespaces to force separate packages.
- Run `ydk ignite --skip-verify`: no Anthropic API key is available, so `ydk spec verify` is
  pending and must run before the first implementation task is closed.

## Alternatives
- Rename manifest namespaces (`message`, `censor`, `store`) so ignite emits one package each —
  churns ids and every `[ydk:...]` link, and splits one domain concept across namespaces only to
  steer a generator.
- Ignite, then move files into the old packages by hand — breaks regeneration and TODO
  traceability; every re-ignite would undo or conflict with the move.

## Consequences
- Censor, Store and Message are files in one package (`internal/messaging`). The former
  forbidden imports between them become code-review rules: the store never calls the censor,
  censor and message validation never touch the store.
- The Kafka adapter lives in `internal/adapters/kafka`; it must not import `internal/httpapi`
  and must not call the Censor (received records are never re-censored).
- The coverage gate covers `internal/messaging`, `internal/httpapi` and `internal/config`.
- Spec verification is outstanding until an API key is configured.
