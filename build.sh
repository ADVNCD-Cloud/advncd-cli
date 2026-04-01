#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT_DIR}"

APP_NAME="advncd"
APP_BIN="${ROOT_DIR}/${APP_NAME}"
EXAMPLE_BIN="${ROOT_DIR}/example/${APP_NAME}"

export GOCACHE="${GOCACHE:-${ROOT_DIR}/.gocache}"
export GOTMPDIR="${GOTMPDIR:-${ROOT_DIR}/.gotmp}"
mkdir -p "${GOCACHE}" "${GOTMPDIR}"

if command -v templ >/dev/null 2>&1; then
  TEMPL_BIN="templ"
elif [[ -x "${HOME}/go/bin/templ" ]]; then
  TEMPL_BIN="${HOME}/go/bin/templ"
else
  echo "templ CLI not found. Install it or add to PATH." >&2
  exit 1
fi

echo "==> Regenerating templ files"
"${TEMPL_BIN}" generate ./internal/dashboard/views

echo "==> Running tests"
# Compile-check all packages and tests.
go test ./... -run '^$'
# Run focused package tests used by current sprint work.
go test ./cmd ./internal/advncdyaml ./internal/detect ./internal/registry

echo "==> Building ${APP_NAME}"
go build -o "${APP_NAME}" .

echo "==> Replacing example binary"
rm -f "${EXAMPLE_BIN}"
cp "${APP_BIN}" "${EXAMPLE_BIN}"
chmod +x "${EXAMPLE_BIN}"

echo "Done: ${APP_BIN}"
echo "Done: ${EXAMPLE_BIN}"
