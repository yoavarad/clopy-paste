// Unit tests for web/split.js. Run: node --test web/split.test.js
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { splitText, LIMIT, BODY_LIMIT, MAX_CHUNKS } = require('./split.js');

const cp = (s) => Array.from(s).length;
const strip = (c) => c.replace(/^\(\d+\/\d+\) /, '');
const join = (chunks) => chunks.map(strip).join('');

function assertChunks(text, chunks) {
  const n = chunks.length;
  chunks.forEach((c, i) => {
    assert.ok(c.startsWith('(' + (i + 1) + '/' + n + ') '), 'prefix of chunk ' + i);
    assert.ok(cp(c) <= LIMIT, 'chunk ' + i + ' has ' + cp(c) + ' code points');
    assert.ok(cp(strip(c)) <= BODY_LIMIT, 'body ' + i);
  });
  assert.equal(join(chunks), text);
}

test('constants', () => {
  assert.equal(LIMIT, 256);
  assert.equal(BODY_LIMIT, 248);
  assert.equal(MAX_CHUNKS, 10);
});

test('empty and short text is one chunk without marker', () => {
  assert.deepEqual(splitText(''), ['']);
  assert.deepEqual(splitText('hi there'), ['hi there']);
});

test('256 code points -> 1 chunk, no marker', () => {
  const t = 'a'.repeat(256);
  assert.deepEqual(splitText(t), [t]);
});

test('257 code points -> 2 chunks (1/2) and (2/2)', () => {
  const t = 'a'.repeat(257);
  const chunks = splitText(t);
  assert.equal(chunks.length, 2);
  assert.ok(chunks[0].startsWith('(1/2) '));
  assert.ok(chunks[1].startsWith('(2/2) '));
  assertChunks(t, chunks);
});

test('no whitespace is hard-cut at 248 code points', () => {
  const t = 'x'.repeat(600);
  const chunks = splitText(t);
  assert.equal(chunks.length, 3);
  assert.equal(cp(strip(chunks[0])), 248);
  assert.equal(cp(strip(chunks[1])), 248);
  assert.equal(cp(strip(chunks[2])), 104);
  assertChunks(t, chunks);
});

test('cuts after the last whitespace before the body limit', () => {
  const t = 'a'.repeat(200) + ' ' + 'b'.repeat(100);
  const chunks = splitText(t);
  assert.equal(chunks.length, 2);
  assert.equal(strip(chunks[0]), 'a'.repeat(200) + ' ');
  assert.equal(strip(chunks[1]), 'b'.repeat(100));
  assertChunks(t, chunks);
});

test('newline and tab count as whitespace', () => {
  const t = 'a'.repeat(100) + '\n' + 'b'.repeat(100) + '\t' + 'c'.repeat(100);
  const chunks = splitText(t);
  assert.equal(strip(chunks[0]), 'a'.repeat(100) + '\n' + 'b'.repeat(100) + '\t');
  assertChunks(t, chunks);
});

test('emoji are counted and cut by code point, never split', () => {
  const t = '\u{1F600}'.repeat(300);
  assert.equal(t.length, 600); // UTF-16 units
  const chunks = splitText(t);
  assert.equal(chunks.length, 2);
  assert.equal(cp(strip(chunks[0])), 248);
  for (const c of chunks) {
    assert.ok(!/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?:^|[^\uD800-\uDBFF])[\uDC00-\uDFFF]/.test(c), 'broken surrogate');
  }
  assertChunks(t, chunks);
});

test('256 emoji fit in one chunk (code points, not UTF-16 units)', () => {
  const t = '\u{1F600}'.repeat(256);
  assert.deepEqual(splitText(t), [t]);
});

test('exactly 10 chunks fit; (10/10) prefix keeps chunk <= 256', () => {
  const t = 'z'.repeat(248 * 10);
  const chunks = splitText(t);
  assert.equal(chunks.length, 10);
  assert.ok(chunks[9].startsWith('(10/10) '));
  assert.equal(cp(chunks[9]), 256);
  assertChunks(t, chunks);
});

test('input needing 11 chunks is too long', () => {
  assert.throws(() => splitText('z'.repeat(248 * 10 + 1)), { name: 'RangeError', message: 'too long' });
});

test('round-trip of mixed prose', () => {
  const words = [];
  for (let i = 0; i < 200; i++) words.push('word' + i + (i % 7 === 0 ? '\u{1F680}' : ''));
  const t = words.join(' ');
  assertChunks(t, splitText(t));
});
