#!/usr/bin/env node
// Runs dependency-cruiser on frontend/src. Fails on a violation that is not in the
// recorded baseline (frontend/.dependency-cruiser-known-violations.json), and on a
// recorded violation that no longer occurs, so the baseline only ever shrinks.
// To remove fixed entries: node scripts/arch-frontend.mjs --update (never to add new ones).
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const keyOf = (v) => `${v.rule.name}|${v.from}|${v.to}`;

export function staleEntries(recorded, current) {
  const now = new Set(current.map(keyOf));
  return recorded.map(keyOf).filter((k) => !now.has(k));
}

// deps: { cruise(mode) -> {status, stdout, stderr}, readBaseline(), writeBaseline(list), update }
// mode 'strict' ignores the baseline, 'fresh' lists every violation as a baseline.
// Returns { code, messages } so the exit paths can be tested.
export function check(deps) {
  const strict = deps.cruise('strict');
  const messages = [strict.stdout, strict.stderr].filter(Boolean);
  if (strict.status !== 0) {
    messages.push(
      'arch (frontend): new violations. Fix them; do not add them to the baseline (docs/adr/0010-target-layering.md).',
    );
    return { code: 1, messages };
  }
  const fresh = deps.cruise('fresh');
  if (fresh.status !== 0) {
    messages.push(
      `arch (frontend): dependency-cruiser failed (exit ${fresh.status}):\n${fresh.stderr || fresh.stdout}`,
    );
    return { code: 1, messages };
  }
  let current;
  try {
    current = JSON.parse(fresh.stdout.slice(fresh.stdout.indexOf('[')));
  } catch {
    messages.push(`arch (frontend): could not read dependency-cruiser's output:\n${fresh.stdout}`);
    return { code: 1, messages };
  }
  const recorded = deps.readBaseline();
  const stale = staleEntries(recorded, current);
  if (stale.length && deps.update) {
    const keep = new Set(current.map(keyOf));
    deps.writeBaseline(recorded.filter((v) => keep.has(keyOf(v))));
    messages.push(`arch (frontend): removed ${stale.length} fixed entries from the baseline`);
  } else if (stale.length) {
    messages.push(
      `arch (frontend): fixed violations still in the baseline:\n  ${stale.join('\n  ')}\nRun: node scripts/arch-frontend.mjs --update`,
    );
    return { code: 1, messages };
  }
  return { code: 0, messages };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const cwd = join(resolve(import.meta.dirname, '..'), 'frontend');
  const baselineFile = join(cwd, '.dependency-cruiser-known-violations.json');
  const run = (extra) =>
    spawnSync('npx', ['depcruise', 'src', '--config', '.dependency-cruiser.cjs', ...extra], { cwd, encoding: 'utf8' });
  const { code, messages } = check({
    cruise: (mode) =>
      mode === 'strict'
        ? run(['--ignore-known', '--output-type', 'err'])
        : run(['--no-ignore-known', '--output-type', 'baseline']),
    readBaseline: () => JSON.parse(readFileSync(baselineFile, 'utf8')),
    writeBaseline: (list) => writeFileSync(baselineFile, JSON.stringify(list, null, 2) + '\n'),
    update: process.argv.includes('--update'),
  });
  for (const m of messages) (code === 0 ? console.log : console.error)(m);
  process.exit(code);
}
