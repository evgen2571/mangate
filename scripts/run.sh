#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "[run] go is not installed or not in PATH" >&2
  exit 1
fi

cd "$repo_root"
exec go run ./cmd/mangate "$@"
