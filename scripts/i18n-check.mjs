#!/usr/bin/env node
// Checks the locale files of the frontend and the backend.
//   errors:   a key missing in a locale (compared with en), a key that is not in en,
//             a value that is not a string
//   warnings: duplicate keys in one object, keys that no source file seems to use
// Usage: node scripts/i18n-check.mjs   (exit code 1 on errors, warnings do not fail)
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

export const LOCALES = ['en', 'de', 'it', 'es', 'fr'];
const PLURAL_SUFFIX = /_(zero|one|two|few|many|other)$/;

// Parses JSON and also reports duplicate keys, which JSON.parse silently collapses.
// Throws on invalid JSON (trailing comma, truncation, BOM, empty file, trailing text).
export function parseWithDuplicates(text) {
  JSON.parse(text); // clear error first; the scan below can then rely on valid input
  const duplicates = [];
  let i = 0;
  const ws = () => {
    while (i < text.length && /\s/.test(text[i])) i++;
  };
  const string = () => {
    const start = i;
    i++; // opening quote
    while (i < text.length && text[i] !== '"') i += text[i] === '\\' ? 2 : 1;
    i++;
    return JSON.parse(text.slice(start, i));
  };
  const value = (path) => {
    ws();
    const c = text[i];
    if (c === '{') {
      i++;
      const obj = Object.create(null);
      ws();
      if (text[i] === '}') {
        i++;
        return obj;
      }
      for (;;) {
        ws();
        const key = string();
        ws();
        i++; // colon
        const full = path ? `${path}.${key}` : key;
        if (Object.hasOwn(obj, key)) duplicates.push(full);
        obj[key] = value(full);
        ws();
        if (text[i] === ',') {
          i++;
          continue;
        }
        i++; // closing brace
        return obj;
      }
    }
    if (c === '[') {
      const start = i;
      let depth = 0;
      do {
        if (text[i] === '"') string();
        else {
          if (text[i] === '[') depth++;
          if (text[i] === ']') depth--;
          i++;
        }
      } while (depth > 0);
      return JSON.parse(text.slice(start, i));
    }
    if (c === '"') return string();
    const m = /^-?[\d.eE+-]+|^true|^false|^null/.exec(text.slice(i));
    i += m[0].length;
    return JSON.parse(m[0]);
  };
  const data = value('');
  return { data, duplicates };
}

// Flattens nested objects to dotted keys. Non-string leaves are returned in `invalid`.
export function flatten(obj, prefix = '', out = Object.create(null), invalid = []) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, key, out, invalid);
    else if (typeof v === 'string') out[key] = v;
    else invalid.push(key);
  }
  return { flat: out, invalid };
}

// Plural forms differ per language (en has _one/_other, others may add _many), so with
// plurals on, a key family is compared by its base name and every locale must have at
// least the `_other` form where en has plurals. The backend has no plural support:
// there keys are compared exactly.
export const baseKey = (key) => key.replace(PLURAL_SUFFIX, '');
const placeholders = (text) => [...new Set([...text.matchAll(/\{\{\s*([^}\s]+)\s*\}\}/g)].map((m) => m[1]))].sort();

// locales: { en: {flat, duplicates, invalid}, de: ... } -> { errors, warnings }
export function compareLocales(locales, label = '', { plurals = true } = {}) {
  const errors = [];
  const warnings = [];
  const prefix = label ? `${label}: ` : '';
  const en = locales.en.flat;
  const family = (k) => (plurals ? baseKey(k) : k);
  // required keys of a locale: exact for plain keys, `<base>_other` for plural families
  const required = new Set();
  const enByFamily = new Map();
  for (const k of Object.keys(en)) {
    const f = family(k);
    if (!enByFamily.has(f)) enByFamily.set(f, []);
    enByFamily.get(f).push(k);
    required.add(plurals && f !== k ? `${f}_other` : k);
  }
  for (const lang of Object.keys(locales)) {
    const { flat, duplicates, invalid } = locales[lang];
    for (const k of invalid) errors.push(`${prefix}${lang}: "${k}" is not a string`);
    for (const k of duplicates) warnings.push(`${prefix}${lang}: duplicate key "${k}"`);
    for (const [k, v] of Object.entries(flat)) {
      if (v.trim() === '') errors.push(`${prefix}${lang}: "${k}" is empty`);
    }
    if (lang === 'en') continue;
    for (const k of required) if (!(k in flat)) errors.push(`${prefix}${lang}: missing key "${k}"`);
    for (const [k, v] of Object.entries(flat)) {
      const f = family(k);
      const counterparts = enByFamily.get(f);
      if (!counterparts) {
        errors.push(`${prefix}${lang}: key "${k}" is not in en`);
        continue;
      }
      // a plural form of a family that is plain in en (or the reverse) does not belong
      if (plurals && counterparts.every((c) => (c === f) !== (k === f))) {
        errors.push(`${prefix}${lang}: key "${k}" is not in en`);
        continue;
      }
      const ref = en[k] ?? en[`${f}_other`] ?? en[counterparts[0]];
      const want = placeholders(ref).join(', ');
      const got = placeholders(v).join(', ');
      if (want !== got) errors.push(`${prefix}${lang}: "${k}" has placeholders {{${got}}}, en has {{${want}}}`);
    }
  }
  return { errors, warnings };
}

// Keys no source seems to use. A string literal equal to a key (or its base, for
// plurals) counts as use; a literal ending in "." or a template literal prefix
// ("settings.x." or `settings.${y}`) marks all keys below it as used, so dynamic
// keys do not produce false warnings.
export function findUnused(keys, sources) {
  const literals = new Set();
  const prefixes = new Set();
  const re = /(['"`])((?:\\.|(?!\1)[^\\\n])*?)\1/g;
  for (const src of sources) {
    for (const m of src.matchAll(re)) {
      const body = m[2];
      if (m[1] === '`' && body.includes('${')) {
        const p = body.slice(0, body.indexOf('${'));
        if (p) prefixes.add(p);
      } else {
        literals.add(body);
        if (body.endsWith('.') || body.endsWith(':')) prefixes.add(body);
      }
    }
  }
  const prefixList = [...prefixes].filter((p) => p.length > 2);
  return keys.filter((key) => {
    const base = baseKey(key);
    if (literals.has(key) || literals.has(base)) return false;
    return !prefixList.some((p) => key.startsWith(p));
  });
}

function readLocaleDir(dir) {
  const locales = {};
  for (const lang of LOCALES) {
    const text = readFileSync(join(dir, `${lang}.json`), 'utf8');
    const { data, duplicates } = parseWithDuplicates(text);
    const { flat, invalid } = flatten(data);
    locales[lang] = { flat, duplicates, invalid };
  }
  return locales;
}

function walk(dir, exts, skip, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (skip(p)) continue;
    if (statSync(p).isDirectory()) walk(p, exts, skip, out);
    else if (exts.some((e) => name.endsWith(e))) out.push(p);
  }
  return out;
}

export function run(root) {
  const errors = [];
  const warnings = [];
  const targets = [
    {
      label: 'frontend',
      plurals: true,
      dir: join(root, 'frontend/src/i18n/locales'),
      sources: walk(
        join(root, 'frontend/src'),
        ['.ts', '.tsx'],
        (p) => p.includes('/i18n/locales') || /\.test\.tsx?$/.test(p),
      ),
    },
    {
      label: 'backend',
      plurals: false,
      dir: join(root, 'backend/i18n/locales'),
      sources: walk(join(root, 'backend'), ['.go'], (p) => /_test\.go$/.test(p) || p.endsWith('/testdata')),
    },
  ];
  for (const t of targets) {
    const locales = readLocaleDir(t.dir);
    const res = compareLocales(locales, t.label, { plurals: t.plurals });
    errors.push(...res.errors);
    warnings.push(...res.warnings);
    const sources = t.sources.map((f) => readFileSync(f, 'utf8'));
    for (const k of findUnused(Object.keys(locales.en.flat), sources)) {
      warnings.push(`${t.label}: key "${k}" seems unused`);
    }
  }
  return { errors, warnings };
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const root = join(dirname(fileURLToPath(import.meta.url)), '..');
  const { errors, warnings } = run(root);
  for (const w of warnings) console.warn(`warning: ${w}`);
  for (const e of errors) console.error(`error: ${e}`);
  console.log(
    `i18n check: ${errors.length} error(s), ${warnings.length} warning(s) (${relative(process.cwd(), root) || '.'})`,
  );
  process.exit(errors.length ? 1 : 0);
}
