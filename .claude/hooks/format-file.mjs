// PostToolUse(Edit|Write): gofmt the touched Go file, so formatting never costs a
// failed `make format-check` later. Silent; other file types and files outside
// the project (e.g. Claude's memory) are skipped. The frontend has no formatter.
import { readFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { resolve, sep } from 'node:path'

const input = JSON.parse(readFileSync(0, 'utf8'))
const file = input.tool_input?.file_path
const project = resolve(process.env.CLAUDE_PROJECT_DIR ?? '.')
if (file && resolve(file).startsWith(project + sep) && file.endsWith('.go')) {
  spawnSync('gofmt', ['-w', file], { cwd: project, stdio: 'ignore' })
}
