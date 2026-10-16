// PostToolUse(Edit|Write): gofmt the touched Go file and Prettier the other
// supported files, so formatting never costs a failed `make format-check` later.
// Silent; files outside the project (e.g. Claude's memory) are skipped.
import { readFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { resolve, sep } from 'node:path'

const input = JSON.parse(readFileSync(0, 'utf8'))
const file = input.tool_input?.file_path
const project = resolve(process.env.CLAUDE_PROJECT_DIR ?? '.')
if (file && resolve(file).startsWith(project + sep)) {
  if (file.endsWith('.go')) {
    spawnSync('gofmt', ['-w', file], { cwd: project, stdio: 'ignore' })
  } else {
    // Prettier skips unsupported and ignored files on its own.
    spawnSync(resolve(project, 'frontend/node_modules/.bin/prettier'), ['--write', '--log-level', 'silent', file], {
      cwd: project,
      stdio: 'ignore',
    })
  }
}
