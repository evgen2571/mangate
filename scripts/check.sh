#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

unformatted="$(gofmt -l cmd internal integration)"
if [[ -n "$unformatted" ]]; then
  printf 'Run gofmt on:\n%s\n' "$unformatted" >&2
  exit 1
fi
uv run --project python --locked ruff check --config python/pyproject.toml python scripts
uv run --project python --locked ruff format --config python/pyproject.toml --check python scripts
go mod tidy -diff
go vet ./...
go test ./...
go build ./...
uv run --project python --locked pytest python/tests
uv run --project python --locked python scripts/verify_docs.py
