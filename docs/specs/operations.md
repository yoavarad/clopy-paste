# Operations

Configuration, packaging, environments, pipeline and runtime behaviour. Decisions: ADR-002,
ADR-004, ADR-008 to ADR-012, ADR-014, ADR-015.

## Configuration

All runtime settings are environment variables read once at startup; the Helm chart maps its
values onto them. The full table (names, Helm keys, defaults, rules) is
[ydk:crosscut:ops/environment]. Highlights:

- INSTANCE_NAME labels the page and forms the default consumer group name.
- KAFKA_CONSUMER_GROUP overrides the consumer group; prod supplies its own value. A new group
  starts at the latest offset (ADR-017).
- Input and output topic names are opaque (ADR-001).
- Security protocol is SASL_SSL or PLAINTEXT only (ADR-008). Credentials come from a Secret
  by reference ([ydk:req:ops/secrets-by-reference]).
- CENSOR_CHARS defaults to `$!~`, PENDING the delivered list (ADR-005).
- LOG_LEVEL defaults to INFO. The HTTP port is fixed at 8080.
- Fixed operational defaults (5 s publish deadline, 16 KiB body cap, GOMEMLIMIT 100MiB,
  port 8080, local security modes per release) are recorded in ADR-017.

Invalid configuration stops the process before it listens ([ydk:error:ops/invalid-config]).
The application version is injected at build time ([ydk:config:ops/app]).

## Container Image

Multi-stage build: Go 1.27 builder, static binary (no cgo, vendored modules, no network),
runtime on UBI 9 micro (ADR-015). The image runs as a numeric non-root user with group-0
permissions so OpenShift can assign an arbitrary UID. The root filesystem is read-only; the
process writes nothing to disk. Size target: [ydk:nfr:system/image-size].

## Helm Chart

One chart deploys one instance; two releases form a pair. Shape and pod security are
specified in [ydk:crosscut:ops/openshift-conventions]:

- Deployment with exactly one replica and Recreate strategy ([ydk:req:ops/single-replica]),
  15 s termination grace period, CPU and memory per [ydk:nfr:system/cpu] and
  [ydk:nfr:system/memory].
- ClusterIP Service on 8080.
- Route with edge TLS when `openshift.enabled` is true, otherwise Ingress.
- NetworkPolicy admitting traffic only from the router or ingress-controller namespace
  ([ydk:req:ops/network-restricted-exposure]).
- Liveness and readiness probes per [ydk:crosscut:system/health-checks].
- values.schema.json rejecting unknown keys; no field for a password value.

## OpenShift Conventions

The pod runs under SCC restricted-v2 without changes: no fixed runAsUser or fsGroup,
non-root, all capabilities dropped, no privilege escalation, RuntimeDefault seccomp,
read-only root filesystem, no service-account token. These are exercised on kind with the
same chart; Route admission and SCC enforcement are only exercised on OpenShift (ADR-009).

## Local and CI Environment

[ydk:crosscut:ops/local-environment] defines the kind cluster, Strimzi Kafka with a TLS+SCRAM
listener and a plaintext listener, ingress-nginx, and two releases:

| Release | Instance name | Output topic | Input topic | Kafka mode |
|---|---|---|---|---|
| clopy-paste-a | side-a | a.out | b.out | SASL_SSL |
| clopy-paste-b | side-b | b.out | a.out | PLAINTEXT |

Wiring A's output topic as B's input topic (and the reverse) stands in for the
[ydk:ext:kafka/external-pipeline]; no forwarder code exists. Running the two releases on
different security modes exercises both code paths in every e2e run. The same scripts run
locally and in GitHub Actions.

## CI/CD

[ydk:crosscut:ops/ci-pipeline] lists stages, triggers, environments and versioning
(ADR-010, ADR-011). All logic sits in make targets and POSIX shell scripts; GitHub and GitLab
files only call make. The scan stage is switched off on GitLab until a vulnerability DB mirror
exists (DEFERRED). Releases deploy with an atomic Helm upgrade that rolls back on failure,
followed by a smoke test.

## Logging

JSON lines on stdout at a configurable level, never containing message text
([ydk:crosscut:system/logging], [ydk:req:system/no-text-in-logs]). File output and the
Filebeat sidecar are DEFERRED (ADR-012).

## Health and Shutdown

Neither liveness nor readiness depends on Kafka ([ydk:crosscut:system/health-checks],
ADR-016). A Kafka outage keeps the pod in the Service and does not restart it; users see it as
failed sends.

On SIGTERM the process stops reporting ready, finishes in-flight requests, flushes the
producer, commits consumer offsets, leaves the group and exits
([ydk:crosscut:system/graceful-shutdown]) within [ydk:nfr:system/shutdown-time].

## Non-Functional Requirements

| NFR | Target |
|---|---|
| [ydk:nfr:system/send-latency] | P95 <= 200 ms, P99 <= 500 ms to Kafka ack |
| [ydk:nfr:system/e2e-latency] | P95 <= 5 s from send on one side to display on the other (local) |
| [ydk:nfr:system/throughput] | >= 50 sends/s per instance |
| [ydk:nfr:system/memory] | 64Mi request, 128Mi limit; worst-case store 40 MB |
| [ydk:nfr:system/cpu] | 50m request, 500m limit |
| [ydk:nfr:system/startup-time] | Ready <= 10 s after container start |
| [ydk:nfr:system/image-size] | <= 60 MB compressed |
| [ydk:nfr:system/availability] | 1 replica, no SLA, restart on crash |
| [ydk:nfr:system/shutdown-time] | Exit <= 10 s after SIGTERM |

No metrics endpoint exists (OUT of scope); NFRs are verified by tests, not by monitoring.
