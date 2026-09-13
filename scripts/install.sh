#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
bin_dir="${MANGATE_BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$bin_dir"
cd "$repo_root"
GOBIN="$bin_dir" go install ./cmd/mangate
printf 'Installed Mangate in %s\n' "$bin_dir"
if [[ ":${PATH}:" != *":$bin_dir:"* ]]; then
  printf 'Add this directory to PATH: %s\n' "$bin_dir"
fi
