import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { parseWithDuplicates, flatten, compareLocales, findUnused, run, LOCALES } from './i18n-check.mjs';

const loc = (obj, text) => {
  const { data, duplicates } = parseWithDuplicates(text ?? JSON.stringify(obj));
  const { flat, invalid } = flatten(data);
  return { flat, duplicates, invalid };
};

test('parseWithDuplicates reports repeated keys with their path', () => {
  const { data, duplicates } = parseWithDuplicates('{"a":{"b":"1","b":"2"},"c":[1,{"x":2}],"d":null}');
  assert.deepEqual(duplicates, ['a.b']);
  assert.equal(data.a.b, '2');
});

test('flatten gives dotted keys and flags non-strings', () => {
  const { flat, invalid } = flatten({ a: { b: 'x' }, n: 3 });
  assert.deepEqual({ ...flat }, { 'a.b': 'x' });
  assert.deepEqual(invalid, ['n']);
});

test('compareLocales: missing and extra keys are errors, duplicates warnings', () => {
  const locales = {
    en: loc({ a: 'A', b: { c: 'C' } }),
    de: loc({ a: 'A' }),
    it: loc({ a: 'A', b: { c: 'C' }, extra: 'x' }),
    fr: loc(null, '{"a":"A","a":"B","b":{"c":"C"}}'),
  };
  const { errors, warnings } = compareLocales(locales);
  assert.deepEqual(errors.sort(), ['de: missing key "b.c"', 'it: key "extra" is not in en']);
  assert.deepEqual(warnings, ['fr: duplicate key "a"']);
});

test('compareLocales: plural forms compare by base name', () => {
  const locales = {
    en: loc({ d_one: '1', d_other: 'n' }),
    es: loc({ d_one: '1', d_many: 'm', d_other: 'n' }),
  };
  assert.deepEqual(compareLocales(locales).errors, []);
});

test('findUnused tolerates dynamic keys', () => {
  const keys = ['a.used', 'a.dyn.x', 'b.unused', 'p.count_one', 'p.count_other', 'c.concat.y'];
  const sources = ["t('a.used'); t(`a.dyn.${k}`); t('p.count', {count: 2}); const p = 'c.concat.'"];
  assert.deepEqual(findUnused(keys, sources), ['b.unused']);
});

test('run fails on a real gap in a fixture tree', () => {
  const root = mkdtempSync(join(tmpdir(), 'i18n-'));
  for (const [dir, src] of [['frontend/src/i18n/locales', 'frontend/src/a.ts'], ['backend/i18n/locales', 'backend/a.go']]) {
    mkdirSync(join(root, dir), { recursive: true });
    mkdirSync(join(root, src, '..'), { recursive: true });
    writeFileSync(join(root, src), "t('k')");
    for (const l of LOCALES) writeFileSync(join(root, dir, `${l}.json`), JSON.stringify(l === 'de' && dir.startsWith('backend') ? {} : { k: 'v' }));
  }
  const { errors } = run(root);
  assert.deepEqual(errors, ['backend: de: missing key "k"']);
});

test('parseWithDuplicates rejects invalid JSON instead of hanging', () => {
  for (const bad of ['{"a":"x",}', '{"a":"x', '', '\ufeff{"a":"x"}', '{"a":"x"} trailing', '{"a":"x\\']) {
    assert.throws(() => parseWithDuplicates(bad), undefined, JSON.stringify(bad));
  }
});

test('a __proto__ key is kept', () => {
  const { data } = parseWithDuplicates('{"__proto__":"x","a":"y"}');
  const { flat } = flatten(data);
  assert.deepEqual(Object.keys(flat).sort(), ['__proto__', 'a']);
});

test('plurals: every locale needs _other where en has plurals', () => {
  const en = loc({ d_one: '{{count}} day', d_other: '{{count}} days' });
  const ok = loc({ d_one: '{{count}} Tag', d_many: '{{count}} Tage', d_other: '{{count}} Tage' });
  const noOther = loc({ d_one: '{{count}} Tag' });
  const plain = loc({ d: '{{count}} Tage' });
  assert.deepEqual(compareLocales({ en, de: ok }).errors, []);
  assert.deepEqual(compareLocales({ en, de: noOther }).errors, ['de: missing key "d_other"']);
  assert.deepEqual(compareLocales({ en, de: plain }).errors.sort(), ['de: key "d" is not in en', 'de: missing key "d_other"']);
});

test('backend locales have no plurals: suffixes are ordinary keys', () => {
  const en = loc({ a_one: 'x', a_other: 'y' });
  const de = loc({ a_other: 'y' });
  assert.deepEqual(compareLocales({ en, de }, 'backend', { plurals: false }).errors, ['backend: de: missing key "a_one"']);
});

test('empty values and different placeholders are errors', () => {
  const en = loc({ a: 'In {{days}} days', b: 'x' });
  const de = loc({ a: 'In {{day}} Tagen', b: '  ' });
  assert.deepEqual(compareLocales({ en, de }).errors.sort(), [
    'de: "a" has placeholders {{day}}, en has {{days}}',
    'de: "b" is empty',
  ]);
});
