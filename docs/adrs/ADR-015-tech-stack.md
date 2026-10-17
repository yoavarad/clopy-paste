# ADR-015: Locked tech stack

## Status
Accepted (locked 2026-10-13; not revisited during implementation)

## Context
Minimal code/deps, air-gapped prod (ADR-002), OpenShift target, kind+Strimzi locally (ADR-009).
Pin policy: newest release at least 2 weeks old at lock time.

## Decision
| Area | Choice |
|---|---|
| Language | Go 1.27 (toolchain 1.27.x) |
| Stdlib used | stdlib only (list non-exhaustive), notably: net/http, encoding/json, html/template, embed, log/slog, container/list, sync, os, os/signal, context, time, unicode/utf8, crypto/tls, crypto/x509 |
| Kafka client | github.com/twmb/franz-go v1.22.1 (+ pkg/sasl/scram), vendored |
| UI | single `index.html`, vanilla JS/CSS, `go:embed`, no build step |
| Unit tests | `go test -race -cover` (stdlib `testing`, `net/http/httptest`) |
| E2E tests | POSIX shell + curl against kind |
| Lint | gofmt, go vet |
| Builder image | golang:1.27 |
| Runtime image | registry.access.redhat.com/ubi9/ubi-micro; CGO_ENABLED=0; non-root, arbitrary UID, read-only root FS |
| Chart | Helm chart apiVersion v2 + values.schema.json; helm CLI v4.3.0 |
| Local k8s | kind v0.33.0, Docker Engine in WSL2 Ubuntu |
| Kafka (local/CI) | Strimzi 1.2.0, KRaft single node, SCRAM-SHA-512/TLS + plaintext listeners |
| CI | GitHub Actions + `.gitlab-ci.yml`, both calling make |
| Image scan | trivy (GitHub only; `SCAN_ENABLED`) |

## Alternatives
- distroless static runtime — smaller, but needs mirroring from gcr.io; UBI likely mirrored.
- golangci-lint — extra tool to ship in air-gapped CI image.
- franz-go v1.22.2 / Strimzi 1.3.0 — released < 2 weeks before lock.

## Consequences
- Any new dependency or tool needs a new ADR.
