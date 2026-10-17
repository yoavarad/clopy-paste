# ADR-009: Local and CI environment — kind + Strimzi

## Status
Accepted

## Context
No local Kubernetes/OpenShift/Kafka. Need an environment close to prod (OpenShift + Kafka with
SASL_SSL/SCRAM) that also runs in GitHub Actions. Machine: 32 GB RAM, WSL2 Ubuntu, no Docker.

## Decision
- Docker Engine inside WSL2 Ubuntu; `kind` cluster; `helm`, `kubectl` in WSL.
- Kafka via Strimzi operator in-cluster (upstream of AMQ Streams), KRaft single node, with two
  listeners: SCRAM-SHA-512 over TLS (prod-like) and plaintext (dev).
- Same setup scripted for GitHub Actions CI.
- OpenShift gaps covered by chart conventions: `Route` when `openshift.enabled=true` else
  `Ingress`; pods run non-root with arbitrary UID, no fixed `runAsUser` (SCC restricted-v2).

## Alternatives
- OpenShift Local (CRC) — real OpenShift, but ~11 GB RAM, slow start, cannot run in GitHub
  Actions.
- Both kind and CRC — most coverage, two environments to maintain.

## Consequences
- OpenShift-only behaviors (Route admission, SCC enforcement) are not exercised locally.
- Local setup is reproducible from scripts in repo.
