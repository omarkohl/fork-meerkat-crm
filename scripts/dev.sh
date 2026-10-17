#!/usr/bin/env bash
# Runs the backend and the frontend dev server together; Ctrl+C stops both.
# Works without configuration: data lives in .dev/ of this checkout, so other
# checkouts never share a database. Optional overrides go in dev.env (shell
# KEY=value lines, any backend variable) or the environment.
set -e
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

set -a
[ ! -f dev.env ] || . ./dev.env
set +a

: "${BACKEND_PORT:=8080}" "${FRONTEND_PORT:=7300}" "${DEV_DATA_DIR:=$root/.dev}"
: "${SQLITE_DB_PATH:=$DEV_DATA_DIR/meerkat.db}" "${PROFILE_PHOTO_DIR:=$DEV_DATA_DIR/photos}"
: "${JWT_SECRET_KEY:=insecure-dev-only-jwt-secret-key-do-not-use-in-production}"
: "${FRONTEND_URL:=http://localhost:$FRONTEND_PORT}"
export SQLITE_DB_PATH PROFILE_PHOTO_DIR JWT_SECRET_KEY FRONTEND_URL
mkdir -p "$PROFILE_PHOTO_DIR"

# Stop everything when one process exits or on Ctrl+C.
trap 'trap - INT TERM EXIT; kill 0' INT TERM EXIT
(cd backend && PORT=$BACKEND_PORT exec go run .) &
(cd frontend && PORT=$FRONTEND_PORT VITE_API_URL=http://localhost:$BACKEND_PORT exec npm start) &
wait -n
