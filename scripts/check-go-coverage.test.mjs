import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { parseTotal, checkCoverage } from './check-go-coverage.mjs';

const output = ['example.com/x/a.go:10:\tf\t100.0%', 'total:\t\t\t\t\t(statements)\t62.9%', ''].join('\n');

test('parseTotal reads the total line of go tool cover -func', () => {
  assert.equal(parseTotal(output), 62.9);
});

test('parseTotal rejects output without a total', () => {
  assert.throws(() => parseTotal('nothing here'), /no total/);
});

test('checkCoverage passes at or above the threshold', () => {
  assert.equal(checkCoverage(62.9, 62), null);
  assert.equal(checkCoverage(62, 62), null);
});

test('checkCoverage explains how to fix a drop', () => {
  const msg = checkCoverage(61.5, 62);
  assert.match(msg, /61\.5%/);
  assert.match(msg, /62%/);
  assert.match(msg, /add tests/i);
});

test('isMain compares file URLs, so paths with spaces and non-ASCII work', async () => {
  const { isMain } = await import('./check-go-coverage.mjs');
  assert.equal(isMain('file:///tmp/a%20b/x.mjs', '/tmp/a b/x.mjs'), true);
  assert.equal(isMain('file:///tmp/a%20b/x.mjs', '/tmp/other.mjs'), false);
});

test('parseThreshold rejects empty, blank and non-numeric values', async () => {
  const { parseThreshold } = await import('./check-go-coverage.mjs');
  assert.equal(parseThreshold('62'), 62);
  for (const bad of [undefined, '', '  ', 'abc']) assert.throws(() => parseThreshold(bad), /threshold/);
});

test('exits 2 on a blank threshold argument', async () => {
  const { spawnSync } = await import('node:child_process');
  const r = spawnSync('node', [fileURLToPath(new URL('./check-go-coverage.mjs', import.meta.url)), ' '], {
    encoding: 'utf8',
  });
  assert.equal(r.status, 2);
  assert.match(r.stderr, /usage/);
});
