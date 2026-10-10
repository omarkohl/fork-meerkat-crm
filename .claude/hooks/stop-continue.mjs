// Stop (cloud only): while .claude/run/PLAN.md has open steps and PROGRESS.md
// has no "## Blocked" entry, nudge the orchestrator once to go on. A second
// stop in a row (stop_hook_active) always ends the turn, so this cannot loop.
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const input = JSON.parse(readFileSync(0, 'utf8'))
if (process.env.CLAUDE_CODE_REMOTE !== 'true' || input.stop_hook_active) process.exit(0)

const run = join(process.env.CLAUDE_PROJECT_DIR ?? '.', '.claude', 'run')
const plan = join(run, 'PLAN.md')
if (!existsSync(plan)) process.exit(0)

const progress = existsSync(join(run, 'PROGRESS.md'))
  ? readFileSync(join(run, 'PROGRESS.md'), 'utf8')
  : ''
const blocked = /^## Blocked[ \t]*\n(?:[ \t]*\n)*[ \t]*[^#\s]/m.test(progress)
const open = /^\s*- \[ \]/m.test(readFileSync(plan, 'utf8'))

if (open && !blocked) {
  console.log(
    JSON.stringify({
      decision: 'block',
      reason:
        'PLAN.md has open steps and PROGRESS.md has no "## Blocked" entry. Continue with the next step. If you cannot, write the reason under "## Blocked" in PROGRESS.md, commit and push it, then stop.',
    })
  )
}
