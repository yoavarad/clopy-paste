# clopy-paste — Overview

## Problem

A developer works on two VMs in isolated networks. The only link between them is a Kafka
cluster with an external pipeline in between. Copying a command, a path or a log snippet
from one VM to the other needs a tool on each side.

We are building **clopy-paste**: one minimal web app deployed as two identical instances,
one per side. On either side the developer types text under a username and sends it; the
other side shows it in a list of received messages for that username, with a copy button.
Both directions work the same way.

Product context and conventions: `docs/project-rules.md`. Decisions: `docs/adrs/ADR-001` to
`ADR-015`.

## Success Criteria

1. A message sent on side A is visible on side B for the same username within the
   [ydk:nfr:system/e2e-latency] target, and the reverse direction works identically
   ([ydk:test:system/round-trip.e2e]).
2. Every character in the configured denylist is replaced before the message leaves the
   instance ([ydk:req:messaging/censor-denylist]).
3. Limits hold: 100 usernames with least-recently-used eviction
   ([ydk:req:messaging/lru-eviction]), 100 messages per username
   ([ydk:req:messaging/fifo-per-user]), text and username lengths
   ([ydk:req:messaging/text-length], [ydk:req:messaging/user-length]).
4. The same image and Helm chart run on kind with Strimzi and on OpenShift with AMQ Streams,
   changing only Helm values ([ydk:crosscut:ops/openshift-conventions]).
5. The full pipeline runs from `make` on GitHub and on air-gapped GitLab
   ([ydk:crosscut:ops/ci-pipeline]).
6. All NFRs in this spec pass their test methods.

## Architecture

```
  Browser on VM A                               Browser on VM B
        │ HTTPS (Route edge TLS / Ingress)            │
┌───────▼─────────────────────────┐   ┌───────────────▼─────────────────┐
│ instance "side A"               │   │ instance "side B"               │
│  httpapi ─► censor              │   │  identical code, other config   │
│     │  └──► store ◄── consumer  │   │                                 │
│     └─► Publisher               │   │                                 │
└──────────┬───────────────▲──────┘   └──────────┬───────────────▲──────┘
   produce │       consume │             produce │       consume │
           ▼               │                     ▼               │
   [A output topic]  [A input topic]     [B output topic]  [B input topic]

   [A output topic] ── external pipeline ──► [B input topic]
   [B output topic] ── external pipeline ──► [A input topic]
```

Each instance knows only its own input and output topic names (ADR-001). The
[ydk:ext:kafka/external-pipeline] copies records between topics; this system never calls it.
Locally the pipeline is replaced by topic wiring ([ydk:crosscut:ops/local-environment]).
Both instances use the same [ydk:ext:kafka/cluster].

## Modules

One Go binary (ADR-002, ADR-015). Each piece of behaviour has exactly one owner.

| Module | Responsibility | Depends on | Forbidden dependencies |
|---|---|---|---|
| cmd/clopy-paste | Wiring: load config, build logger, Kafka adapter, store, HTTP server; signal handling and ordered shutdown ([ydk:crosscut:system/graceful-shutdown]) | all internal modules | none |
| internal/config | Read and validate environment ([ydk:crosscut:ops/environment]); report [ydk:error:ops/invalid-config] | stdlib | every other internal module |
| internal/messaging | Domain type [ydk:entity:messaging/Message] with length limits and validation shared by send and receive paths; denylist replacement [ydk:contract:messaging/Censor]; in-memory [ydk:entity:messaging/MessageStore] with FIFO and LRU, operations per [ydk:contract:messaging/Store]; the [ydk:contract:messaging/Publisher] port and its fake | stdlib | config, httpapi, adapters/kafka, franz-go |
| internal/adapters/kafka | franz-go adapter: implements [ydk:contract:messaging/Publisher]; consumer loop that validates records and calls the store | messaging, franz-go | httpapi; never calls the Censor (received records are never re-censored) |
| internal/httpapi | Routes, handlers, error format, health endpoints; serves the page | messaging, web | adapters/kafka, franz-go |
| web | Page [ydk:page:ui/main] as HTML embedded with go:embed | stdlib | every internal module |

Depended-on-by: messaging is used by adapters/kafka and httpapi; config by cmd only. No module
imports cmd. Message, Censor and Store share one package (ADR-018), so the rules between them
are review rules: the store never calls the censor; censor and message validation never touch
the store.

## Scope

**IN**
- Two-instance bi-directional text transfer through Kafka with per-instance input and output
  topics (ADR-001), send and receive flows in `messaging.md`.
- Single page with send form, received list, copy button and 2 s polling (ADR-003), described
  in `api-and-ui.md`.
- In-memory store with LRU users and FIFO messages (ADR-004, ADR-013).
- Configurable denylist censoring of username and text (ADR-005).
- Limit validation and invalid-record dropping (ADR-006).
- Kafka record contract (ADR-007), SASL_SSL/SCRAM-SHA-512 and PLAINTEXT modes (ADR-008).
- Helm chart for OpenShift and Kubernetes, kind + Strimzi local and CI environment (ADR-009).
- Make-driven CI/CD on GitHub and GitLab with lint, test, build, scan, e2e, publish, deploy,
  smoke (ADR-010, ADR-011).
- JSON logs to stdout (ADR-012). Network-restricted exposure without app auth (ADR-014).
- Readiness independent of Kafka; Kafka state shown on failed sends (ADR-016).
- Configurable consumer group, `latest` reset for new groups, fixed operational defaults
  (ADR-017).

**OUT**
- Authentication or authorization of any kind.
- Persistence of messages across restarts; volumes.
- More than one replica; horizontal scaling.
- WebSocket or Server-Sent Events.
- Editing or deleting messages; storing sent messages.
- Multiple rooms or channels; more than two sides.
- File or binary transfer.
- Admin UI.
- Prometheus metrics endpoint.
- Topic creation by the app; forwarder code between topics.

**DEFERRED**
- Filebeat sidecar, LOG_FILE option and Logstash mock (design in ADR-012).
- Real censor denylist; the default `$!~` is PENDING until the list is delivered (ADR-005).
- Vulnerability DB mirror for the GitLab scan stage (ADR-011).

## Narratives

- `messaging.md` — send, receive, censoring, limits, store, Kafka, E2E walkthrough,
  adversarial review.
- `api-and-ui.md` — routes, errors, page, wireframe, UI behaviour.
- `operations.md` — configuration, chart, OpenShift conventions, local environment, CI/CD,
  logging, health, shutdown, NFRs.
- `testing.md` — test levels, rules, coverage gate.

## Glossary

| Term | Meaning | DO NOT USE |
|---|---|---|
| Instance | One deployment of clopy-paste (one Helm release, one pod). Two exist: side A and side B. | node, server, app copy |
| Side | User-facing name of an instance ("side A"), shown from INSTANCE_NAME. "Other side" = the peer instance. | end, peer site, remote |
| Username | Free-text label (1..128 code points) that groups messages. Not an identity. | account, login, user ID |
| Message | Text sent under a username. On the receiving instance it becomes [ydk:entity:messaging/Message]. | note, post, paste |
| Send | Publishing a message from this instance to its output topic. | post, push, submit |
| Received list | Messages from the other side for the username in the username field. | inbox, feed, history |
| Output topic | Topic an instance produces to. | outgoing queue, send topic |
| Input topic | Topic an instance consumes from. | incoming queue, receive topic |
| External pipeline | The system that moves records from one output topic to the other input topic. | bridge, mirror, forwarder |
| Denylist | Set of characters replaced by `_` before publishing. | blacklist, filter, sanitizer list |
| Censor | Applying the denylist. | sanitize, escape, scrub |
| Store | The in-memory structure holding received messages. | cache, database, buffer |
| Record | One Kafka record, [ydk:entity:messaging/KafkaRecord]. | event, packet |
