# Delta: api-and-ui.md

## MODIFIED

- Text limit is 256 Unicode code points (was 1000). The server is still the only authority
  and rejects text > 256 with the validation-failed error ([ydk:req:messaging/text-length]).
- The text area no longer has `maxlength`. The live counter shows code points (`Array.from`)
  and the number of chunks the text will be sent as.

## ADDED

- Client-side auto-split ([ydk:component:ui/send-form]): text <= 256 code points is sent as
  one message with no marker. Longer text is split into chunks prefixed `"(i/N) "`, each body
  <= 248 code points, cut at the last whitespace before the limit (hard cut only if none).
- Button label: `Send` / `Send as X messages` (2..10); disabled with a "too long" note when
  more than 10 chunks would be needed.
- Chunks are sent one by one, each awaiting its 202. On a failure the page stops, puts the
  unsent text back in the text area and shows `sent k/N — <error>`.
