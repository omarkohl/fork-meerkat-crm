import { test } from 'node:test';
import assert from 'node:assert/strict';
import { keyOf, staleEntries, check } from './arch-frontend.mjs';

const entry = (from, to, rule) => ({ from, to, rule: { name: rule, severity: 'error' } });

test('keyOf identifies a violation by rule, from and to', () => {
  assert.equal(keyOf(entry('src/A.tsx', 'src/api/x.ts', 'r')), 'r|src/A.tsx|src/api/x.ts');
});

test('staleEntries lists recorded violations that no longer occur', () => {
  const recorded = [entry('a', 'b', 'r'), entry('c', 'd', 'r')];
  const current = [entry('a', 'b', 'r')];
  assert.deepEqual(staleEntries(recorded, current), ['r|c|d']);
});

test('staleEntries is empty when nothing was fixed', () => {
  const recorded = [entry('a', 'b', 'r')];
  assert.deepEqual(staleEntries(recorded, [entry('a', 'b', 'r'), entry('x', 'y', 'r')]), []);
});

// check(deps) with stubbed dependency-cruiser runs: [exit code, output]
const deps = ({ strict, fresh, recorded = [entry('a', 'b', 'r')], update = false }) => {
  const written = [];
  return {
    cruise: (mode) => (mode === 'strict' ? strict : fresh),
    readBaseline: () => recorded,
    writeBaseline: (v) => written.push(v),
    update,
    written,
  };
};

test('check passes when nothing is new and nothing is stale', () => {
  const d = deps({
    strict: { status: 0, stdout: 'ok', stderr: '' },
    fresh: { status: 0, stdout: JSON.stringify([entry('a', 'b', 'r')]), stderr: '' },
  });
  assert.equal(check(d).code, 0);
});

test('check fails on a new violation', () => {
  const d = deps({
    strict: { status: 1, stdout: 'error r: x', stderr: '' },
    fresh: { status: 0, stdout: '[]', stderr: '' },
  });
  const r = check(d);
  assert.equal(r.code, 1);
  assert.match(r.messages.join('\n'), /new violations/);
});

test('check fails clearly when dependency-cruiser itself fails', () => {
  const d = deps({ strict: { status: 0, stdout: '', stderr: '' }, fresh: { status: 2, stdout: '', stderr: 'boom' } });
  const r = check(d);
  assert.equal(r.code, 1);
  assert.match(r.messages.join('\n'), /dependency-cruiser failed/);
});

test('check fails when the baseline output is not JSON', () => {
  const d = deps({
    strict: { status: 0, stdout: '', stderr: '' },
    fresh: { status: 0, stdout: 'no json here', stderr: '' },
  });
  assert.equal(check(d).code, 1);
});

test('check fails on a stale entry and tells how to update', () => {
  const d = deps({ strict: { status: 0, stdout: '', stderr: '' }, fresh: { status: 0, stdout: '[]', stderr: '' } });
  const r = check(d);
  assert.equal(r.code, 1);
  assert.match(r.messages.join('\n'), /--update/);
});

test('check --update drops stale entries and passes', () => {
  const d = deps({
    update: true,
    strict: { status: 0, stdout: '', stderr: '' },
    fresh: { status: 0, stdout: '[]', stderr: '' },
  });
  const r = check(d);
  assert.equal(r.code, 0);
  assert.deepEqual(d.written, [[]]);
});
