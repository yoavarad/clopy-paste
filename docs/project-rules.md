# Project Rules — clopy-paste

## Product Context

Minimal web app, deployed as two identical instances on two isolated VMs/clusters, letting a
developer type text under a username on one side and receive it on the other. Bi-directional.
Kafka is the only link between the sides.

- UI: username field, free-text area (~half screen), Send button; other half shows latest
  messages received from the other side for the username currently typed in the box.
- Limits: max 100 users (least-recently-used user evicted when the 101st arrives); max 100
  messages per user (FIFO); text max 256 chars; username max 128 chars.
- Special characters in text are replaced with `_` before publishing to Kafka.
- Kafka payload: flat JSON with username field and text field.
- Runtime: OpenShift/Kubernetes, released via Helm charts. CI/CD on GitHub here; GitLab in prod.
- Logging: stdout now; later a Filebeat sidecar ships to a central Logstash.
- No real infra locally: mock it and run as close to prod as possible.
- Minimal appearance, code, third-party dependencies and features.

## Conventions

- Language: Go, stdlib first. Only allowed third-party dep: `github.com/twmb/franz-go` (ADR-002).
  Adding any other dependency requires a new ADR.
- Prod is air-gapped: `vendor/` committed; builds, images and CI MUST NOT need internet access.
- Each instance has `inputTopic` + `outputTopic` as opaque config (ADR-001).

## Known Gotchas

- OpenShift: a sidecar cannot read another container's stdout (needs hostPath to
  `/var/log/containers`, forbidden by SCC restricted-v2). The deferred Filebeat phase must tee
  logs to a shared `emptyDir` file instead (ADR-012).
- `navigator.clipboard` works only in a secure context (HTTPS or localhost). Over plain HTTP
  (e.g. port-forward to a non-localhost address) the copy button must fall back to a hidden
  textarea + `document.execCommand('copy')`.
- Browser `maxlength` counts UTF-16 code units; the server counts Unicode code points. An
  emoji is 2 units but 1 code point. The server check is the authority (ADR-006).
- OpenShift router default idle timeout is 30 s. Irrelevant here: the UI polls with short
  requests every 2 s (ADR-003); revisit only if SSE/WebSocket is ever added.
- Never set a fixed `runAsUser`/`fsGroup` in the chart: OpenShift assigns an arbitrary UID
  from the namespace range. The image must work for any UID in group 0.
- Readiness must NOT depend on Kafka (ADR-016): a Kafka-gated readiness would pull the pod
  out of the Service and make the router answer 503 for the whole page. Kafka state reaches
  the UI only via 503 on send ("Kafka: disconnected").
- A new consumer group starts at the latest offset (ADR-017): records produced before an
  instance first starts are never shown. e2e must wait for both releases to be Ready and
  consuming before the first send.
- Go's ServeMux writes plain-text 404/405 bodies; wrap it to emit the JSON error format.

## Error Handling Rules

- Every non-2xx HTTP response body is exactly `{"error":"<message>"}`; statuses used are
  400, 404, 405, 500, 503 only.
- Validate before censoring; never truncate; nothing is published on any 4xx.
- Kafka publish failure or 5 s timeout -> 503; the UI keeps the user's text.
- Invalid incoming Kafka records: drop, log WARN with topic/partition/offset/reason, commit,
  continue. Never crash or retry on a bad record.
- Invalid configuration: log one ERROR naming the variable and exit 1 before listening.
- Error messages and logs never contain message text, record values or credentials.
- Recover handler panics -> 500 with a fixed message.

## Testing Rules

- Fake only external boundaries (Kafka via the `Publisher` interface), never internal types
  (store, censor, validation, config, router).
- No Kafka mocks in integration tests: run against real Strimzi in kind.
- e2e talks to the system only over HTTP through each release's Ingress.
- `go test -race` always; coverage >= 80% on core packages (internal/messaging, internal/httpapi,
  internal/config).
- Every requirement acceptance criterion maps to at least one test case.
