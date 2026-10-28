# API and UI

HTTP surface and the single page. Exact paths, shapes and status codes live in the route
manifests; this narrative explains behaviour. Decisions: ADR-003, ADR-005, ADR-006, ADR-013,
ADR-014.

## Routes

| Route | Purpose |
|---|---|
| [ydk:route:api/index] | Serves the page, with the instance name inserted server-side. |
| [ydk:route:api/send-message] | Validates, censors and publishes one message; answers after the Kafka ack. |
| [ydk:route:api/get-messages] | Returns the received list for one username, newest first, at most 100; refreshes recency for known usernames. |
| [ydk:route:api/healthz] | Liveness: process serves HTTP. |
| [ydk:route:api/readyz] | Readiness: server up with valid configuration and not shutting down; independent of Kafka (ADR-016). |

No route requires authentication (ADR-014, [ydk:req:ops/network-restricted-exposure]).
Application-level settings (title, no auth, no CORS, middleware) are in [ydk:config:ops/app].
Uniform rules for port, timestamps, body size, CORS and pagination are in
[ydk:crosscut:system/http-conventions].

## Errors

Every error response uses one body format defined in [ydk:crosscut:system/error-format].

| Error | When |
|---|---|
| [ydk:error:api/invalid-json] | Send body is not a JSON object with string fields. |
| [ydk:error:api/validation-failed] | Username or text empty or over limit, body over the size cap, or read without a username. |
| [ydk:error:api/not-found] | Unknown path. |
| [ydk:error:api/method-not-allowed] | Known path, wrong method. |
| [ydk:error:api/internal-error] | Unexpected handler fault. |
| [ydk:error:api/kafka-unavailable] | Publish failed or not acknowledged in 5 s. |
| [ydk:error:api/not-ready] | Readiness during shutdown. |

The server is the only authority on limits. The browser enforces `maxlength` as a
convenience, but it counts UTF-16 units, so for emoji the browser and server counts differ;
the server result wins (ADR-006).

## Page

[ydk:page:ui/main] is one HTML file with inline CSS and JavaScript, embedded in the binary,
no build step, no external resources (ADR-015, [ydk:req:system/dependency-policy]).

```
┌──────────────────────────────────────┬──────────────────────────────────────┐
│ clopy-paste                 [side A] │ Received from other side             │
│ User: [bob__________________]        │ 14:02:31                    [copy]   │
│ ┌──────────────────────────────────┐ │ ┌──────────────────────────────────┐ │
│ │  free text...                    │ │ │ kubectl get pods -n foo          │ │
│ └──────────────────────────────────┘ │ └──────────────────────────────────┘ │
│ 0/256                      [Send]   │ ...  (newest first, max 100)         │
│ status: sent / error: text > 256    │                                      │
└──────────────────────────────────────┴──────────────────────────────────────┘
```

### Left column — [ydk:component:ui/send-form]

- Title and the instance label in brackets, taken from INSTANCE_NAME.
- Username field limited to 128 characters; its value is remembered in the browser's local
  storage and restored on load.
- Text area of about half the screen height, limited to 256 characters, with a live counter.
- Send button, disabled while a send is in flight.
- Status line: "sent" after success (text area cleared, counter reset); "Kafka: disconnected"
  when the send is rejected because Kafka is unavailable; "error: …" with the server's message
  on other failures. The text is kept on every failure ([ydk:req:ui/keep-text-on-error]).

### Right column — [ydk:component:ui/received-list]

- Heading "Received from other side".
- Polls the received list every 2 s for the username in the field (ADR-003); a change of the
  username triggers an immediate refresh. No polling while the field is empty.
- Each item shows the local time, the text with whitespace and line breaks preserved, and a
  copy button. Text is written as plain text, never as HTML
  ([ydk:req:ui/plain-text-rendering]).
- Copy uses the asynchronous clipboard API in a secure context (HTTPS via Route/Ingress).
  Over plain HTTP (for example a port-forward) the browser blocks that API, so the page falls
  back to selecting a hidden text area and issuing the legacy copy command.
- On a failed poll the last list stays on screen.

### API Calls per Page

| Page | Calls |
|---|---|
| [ydk:page:ui/main] | [ydk:route:api/index] once on load; [ydk:route:api/get-messages] every 2 s and on username change; [ydk:route:api/send-message] on Send. |

## Behaviour During a Kafka Outage

Readiness does not depend on Kafka (ADR-016), so the page and the received list stay
reachable during an outage.

1. A send gets [ydk:error:api/kafka-unavailable] after at most 5 s. The status line shows
   "Kafka: disconnected" and the text stays in the text area.
2. The received list keeps polling and shows what the store holds; no new messages arrive
   while the consumer cannot reach Kafka.
3. When Kafka returns, the next send succeeds and the status line shows "sent".

The send response is the only Kafka-state signal the page uses. No new route or response
field exists for it: the page learns the state when the user sends, which is the only moment
the state affects the user.
