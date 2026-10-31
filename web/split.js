// splitText: client-side auto-split of long text (ADR-019). Pure, no DOM.
// Inlined into main.html by the server; also loadable by node (web/split.test.js).
(function (root) {
  'use strict';
  var LIMIT = 256; // max code points per message (server rule, ADR-006)
  var BODY_LIMIT = 248; // LIMIT minus the longest prefix "(10/10) "
  var MAX_CHUNKS = 10;

  // splitText returns [text] when it fits in LIMIT code points; otherwise chunks prefixed
  // "(i/N) " whose bodies are <= BODY_LIMIT code points, cut after the last whitespace in the
  // window (hard cut when there is none). Throws RangeError('too long') above MAX_CHUNKS.
  function splitText(text) {
    var cps = Array.from(text);
    if (cps.length <= LIMIT) { return [text]; }
    var bodies = [];
    var start = 0;
    while (cps.length - start > BODY_LIMIT) {
      if (bodies.length === MAX_CHUNKS) { throw new RangeError('too long'); }
      var end = start + BODY_LIMIT;
      for (var i = end - 1; i >= start; i--) {
        if (/\s/.test(cps[i])) { end = i + 1; break; }
      }
      bodies.push(cps.slice(start, end).join(''));
      start = end;
    }
    bodies.push(cps.slice(start).join(''));
    if (bodies.length > MAX_CHUNKS) { throw new RangeError('too long'); }
    var n = bodies.length;
    return bodies.map(function (b, k) { return '(' + (k + 1) + '/' + n + ') ' + b; });
  }

  var api = { splitText: splitText, LIMIT: LIMIT, BODY_LIMIT: BODY_LIMIT, MAX_CHUNKS: MAX_CHUNKS };
  if (typeof module === 'object' && module.exports) { module.exports = api; } else { root.clopySplit = api; }
})(this);
