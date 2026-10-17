# ADR-002: Go for the server

## Status
Accepted

## Context
Server serves one HTML page and bridges HTTP <-> Kafka. Goals: minimal code, minimal third-party
deps, small image. Prod (GitLab + OpenShift) is air-gapped: builds have no internet access.

## Decision
Go (latest stable at implementation time). Stdlib for HTTP, JSON, embed, logging (`log/slog`),
tests. Single third-party dependency: `github.com/twmb/franz-go` (pure-Go Kafka client).
Dependencies vendored (`go mod vendor`, `vendor/` committed) so builds need no network.

## Alternatives
- Python — familiar and fits YDK python tooling, but production HTTP needs a framework,
  `confluent-kafka` bundles a C library, larger image, needs PyPI mirror when air-gapped.
- Node.js — `kafkajs` unmaintained since 2023; maintained client wraps librdkafka; large
  dependency tree; needs npm mirror.

## Consequences
- Static binary, small image (distroless/UBI-micro class), runs as arbitrary UID on OpenShift.
- Verification uses `gofmt`, `go vet`, `go test` instead of YDK `python-quality` plugin.
- Go toolchain must be installed locally and present in CI build images.
