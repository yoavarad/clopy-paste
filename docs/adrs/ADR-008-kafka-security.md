# ADR-008: Kafka connection — SASL_SSL/SCRAM-SHA-512 in prod, plaintext for dev

## Status
Accepted

## Context
Prod Kafka requires SASL_SSL with SCRAM-SHA-512. Local dev needs a zero-setup option.

## Decision
Config-driven, two modes only:
- `KAFKA_SECURITY_PROTOCOL=SASL_SSL` — SCRAM-SHA-512; `KAFKA_SASL_USERNAME`,
  `KAFKA_SASL_PASSWORD` from a Kubernetes Secret; optional `KAFKA_CA_FILE` (CA cert mounted
  from Secret/ConfigMap; system roots if unset).
- `KAFKA_SECURITY_PROTOCOL=PLAINTEXT` — no auth, no TLS; dev only.
Implemented with `franz-go` and its `pkg/sasl/scram` sub-package (same module, no new dep).

## Alternatives
- Also support TLS-only and mTLS — not used in prod; speculative.

## Consequences
- Any other protocol value is a startup error.
- Credentials never in Helm values files or logs; only Secret references.
