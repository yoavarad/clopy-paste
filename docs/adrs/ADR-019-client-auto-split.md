# ADR-019: Client splits long text into numbered messages; server still rejects > 256

## Status
Accepted. Amends ADR-006 (outgoing part).

## Context
The text limit is 256 code points (ADR-006, lowered from 1000). Users paste longer text and
the browser `maxlength` silently cut it (and counted UTF-16 units, not code points). The
server API, Kafka record contract (ADR-007) and store must not change.

## Decision
- The page splits, the server does not. `web/split.js` exports a pure `splitText(text)` (no
  DOM). It is embedded with `go:embed` and inlined into `main.html` by the index handler
  (`{{SPLIT_JS}}`), so the page stays one document and no route is added.
- Length is counted in code points (`Array.from`). <= 256 code points: one message, no marker.
  Longer: chunks prefixed `"(i/N) "`; each body <= 248 code points (longest prefix
  `"(10/10) "` is 8), cut after the last whitespace in the window, hard cut only when the
  window has none. Code points are never split. At most 10 chunks; more is "too long".
- The textarea has no `maxlength`. Chunks are POSTed one at a time, each after the previous
  202 (same user key -> same partition -> ordered). On a failure the page stops, puts the
  unsent text back in the textarea and shows `sent k/N - <error>`.
- Server validation is unchanged: text > 256 code points is still HTTP 400 (ADR-006).
- `node --test web/split.test.js` runs in `make test` (CI stage 2).

## Alternatives
- Server-side split — changes the API contract and the one-request-one-message model.
- Keep `maxlength` — loses pasted content silently.
- Separate `/split.js` route — adds an API route for one small script.

## Consequences
- A long paste arrives as up to 10 ordered messages; the receiver sees the `(i/N)` markers.
- A mid-sequence failure leaves earlier chunks delivered; the remainder stays editable.
- CI needs `node` on PATH for `make test`.
