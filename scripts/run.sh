#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
project_name="mangate"
config_root="${XDG_CONFIG_HOME:-$HOME/.config}"
config_dir="$config_root/$project_name"
config_file="$config_dir/config.json"

mkdir -p "$config_dir"

if [[ ! -f "$config_file" ]]; then
  echo "[run] config file not found at $config_file"
  echo "[run] continuing with built-in defaults"
fi

if ! command -v chafa >/dev/null 2>&1; then
  echo "[run] warning: chafa is not installed. Cover rendering in the TUI will fail until it is installed." >&2
  echo "[run] install it manually or extend scripts/install.sh later if you want dependency setup there too" >&2
fi

if ! command -v go >/dev/null 2>&1; then
  echo "[run] go is not installed or not in PATH" >&2
  exit 1
fi

cd "$repo_root"
exec go run ./cmd/mangate "$@"
