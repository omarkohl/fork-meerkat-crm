#!/usr/bin/env node
// Fails when the total Go statement coverage is below a threshold.
// Usage: node scripts/check-go-coverage.mjs <min-percent> [coverage.out]  (run from backend/)
// The threshold lives in the Makefile (GO_COVERAGE_MIN). Raise it when coverage rises.
import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

export function parseTotal(funcOutput) {
  const m = /^total:\s+\(statements\)\s+([\d.]+)%/m.exec(funcOutput);
  if (!m) throw new Error('no total line in go tool cover output');
  return Number(m[1]);
}

export function checkCoverage(total, min) {
  if (total >= min) return null;
  return `Go coverage ${total}% is below the threshold of ${min}%. Add tests for the new code; do not lower the threshold.`;
}

export function isMain(metaUrl, argv1) {
  return argv1 !== undefined && metaUrl === pathToFileURL(argv1).href;
}

export function parseThreshold(arg) {
  if (arg === undefined || arg.trim() === '' || !Number.isFinite(Number(arg))) {
    throw new Error(`invalid threshold ${JSON.stringify(arg)}`);
  }
  return Number(arg);
}

if (isMain(import.meta.url, process.argv[1])) {
  const file = process.argv[3] ?? 'coverage.out';
  let min;
  try {
    min = parseThreshold(process.argv[2]);
  } catch (e) {
    console.error(`${e.message}\nusage: check-go-coverage.mjs <min-percent> [coverage.out]`);
    process.exit(2);
  }
  const total = parseTotal(execFileSync('go', ['tool', 'cover', `-func=${file}`], { encoding: 'utf8' }));
  const problem = checkCoverage(total, min);
  if (problem) {
    console.error(problem);
    process.exit(1);
  }
  console.log(`Go coverage ${total}% (threshold ${min}%)`);
}
