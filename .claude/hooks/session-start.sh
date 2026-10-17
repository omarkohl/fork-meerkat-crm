#!/usr/bin/env bash
# SessionStart: in cloud sessions, install dependencies, print the autonomous-run
# rules and the run state. Local sessions: no output, no install.
set -u
cd "$CLAUDE_PROJECT_DIR" || exit 0

[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0

# Go: go.mod pins the version; GOTOOLCHAIN=auto fetches it if the image is older.
if command -v go >/dev/null; then
  (cd backend && GOTOOLCHAIN=auto go mod download >/tmp/go-install.log 2>&1) ||
    echo "WARNING: go mod download failed, see /tmp/go-install.log"
else
  echo "WARNING: go not found. Install it in the environment setup script."
fi

# Frontend: skip if the lockfile is unchanged.
stamp=frontend/node_modules/.install-stamp
if [ ! -f "$stamp" ] || [ frontend/package-lock.json -nt "$stamp" ] || [ frontend/package.json -nt "$stamp" ]; then
  if (cd frontend && npm ci >/tmp/npm-install.log 2>&1); then
    touch "$stamp"
  else
    echo "WARNING: frontend install failed, see /tmp/npm-install.log"
  fi
fi

if ! node -e 'process.exit(+process.versions.node.split(".")[0] >= 26 ? 0 : 1)'; then
  echo "WARNING: node $(node -v) is older than 26 (see .node-version)."
fi

# Playwright browser in the background: the download needs allowlisted hosts.
(cd frontend && npx playwright install chromium >/tmp/playwright-install.log 2>&1 &)

cat .claude/AUTONOMOUS-RUNS.md
if [ -f .claude/run/PLAN.md ]; then
  echo
  echo "Autonomous run active: follow .claude/AUTONOMOUS-RUNS.md (cloud: git, stacked PRs)."
  echo
  cat .claude/run/PLAN.md
  echo
  cat .claude/run/PROGRESS.md 2>/dev/null
fi
exit 0
