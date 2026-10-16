// PreToolUse(Bash): enforce the push rules of .claude/AUTONOMOUS-RUNS.md.
// Locally, during a run, Claude pushes nothing; in the cloud, never main, never `gh pr`.
import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const input = JSON.parse(readFileSync(0, 'utf8'));
const command = input.tool_input?.command ?? '';
const remote = process.env.CLAUDE_CODE_REMOTE === 'true';

function deny(reason) {
  console.log(
    JSON.stringify({
      hookSpecificOutput: {
        hookEventName: 'PreToolUse',
        permissionDecision: 'deny',
        permissionDecisionReason: reason,
      },
    }),
  );
  process.exit(0);
}

const pushes = /\b(git\s+push|jj\s+git\s+push)\b/.test(command);
const opensPr =
  /\bgh\s+pr\s+(create|edit|merge)\b/.test(command) ||
  /\bgh\s+api\b[^|;&]*\bpulls\b[^|;&]*(\s-[fF]\s|--(raw-)?field|-X\s*(POST|PATCH|PUT))/.test(command);

const runActive = existsSync(join(process.env.CLAUDE_PROJECT_DIR ?? '.', '.claude', 'run', 'PLAN.md'));

if (!remote && runActive && (pushes || opensPr)) {
  deny(
    'Pushing and opening PRs is for cloud sessions only (.claude/AUTONOMOUS-RUNS.md). Ask the user; they can run it with the ! prefix.',
  );
}

if (remote) {
  // Only the push itself, not the commands chained after it.
  const pushArgs = command.match(/\bgit\s+push\b[^|;&]*/g) ?? [];
  if (pushArgs.some((p) => /(\s|:|\+)(refs\/heads\/)?main(\s|$)/.test(p))) {
    deny('Never push to main. Push the step branch (see .claude/AUTONOMOUS-RUNS.md).');
  }
  // A push without an explicit branch (or with HEAD) pushes the current branch.
  const implicit = pushArgs.some((p) => {
    const refs = p
      .trim()
      .split(/\s+/)
      .slice(2)
      .filter((a) => !a.startsWith('-'));
    return refs.length < 2 || refs.slice(1).every((r) => /^\+?HEAD$/.test(r));
  });
  if (implicit) {
    const head = spawnSync('git', ['rev-parse', '--abbrev-ref', 'HEAD'], {
      cwd: process.env.CLAUDE_PROJECT_DIR ?? '.',
      encoding: 'utf8',
    }).stdout?.trim();
    if (head === 'main') {
      deny('The current branch is main: never push it. Create the step branch first.');
    }
  }
  if (/\bgh\s+pr\b/.test(command)) {
    deny(
      '`gh pr` uses GraphQL, which the cloud GitHub proxy blocks. Use the REST API: gh api repos/{owner}/{repo}/pulls ...',
    );
  }
  if (
    pushArgs.some((p) => /\s(-d|--delete)\b|\s:[\w/-]+/.test(p)) ||
    /\bgh\s+api\b[^|;&]*(\/merge\b|(-X\s*|--method\s+)DELETE\b)/.test(command)
  ) {
    deny('Merging PRs and deleting branches is for the user.');
  }
}
