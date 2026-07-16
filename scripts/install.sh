#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
project_name="mangate"
bin_dir="${HOME}/.local/bin"
install_path="$bin_dir/$project_name"
if [[ -n "${MANGATE_CONFIG:-}" ]]; then
  config_file="$MANGATE_CONFIG"
else
  config_root="${XDG_CONFIG_HOME:-$HOME/.config}"
  config_file="$config_root/$project_name/config.json"
fi
config_dir="$(dirname -- "$config_file")"

echo "[install] repo: $repo_root"
echo "[install] binary path: $install_path"
echo "[install] config path: $config_file"

if ! command -v go >/dev/null 2>&1; then
  echo "[install] go is not installed or not in PATH" >&2
  exit 1
fi

mkdir -p "$bin_dir"
mkdir -p "$config_dir"

cd "$repo_root"
go build -o "$install_path" ./cmd/mangate
chmod +x "$install_path"

echo "[install] built $project_name into $install_path"

if [[ -f "$config_file" ]]; then
  echo "[install] config already exists, leaving it unchanged"
else
  download_dir="$HOME/downloads/mangate"
  cache_root="${XDG_CACHE_HOME:-$HOME/.cache}"
  cache_dir="$cache_root/mangate"

  cat > "$config_file" <<EOF
{
  "provider": "mangadex",
  "language": "en",
  "providers": {
    "mangadex": {
      "siteUrl": "https://mangadex.org",
      "baseUrl": "https://api.mangadex.org",
      "uploadsUrl": "https://uploads.mangadex.org"
    }
  },
  "http": {
    "timeout": "30s"
  },
  "download": {
    "dir": "$download_dir",
    "format": "directory",
    "existingFileMode": "skip",
    "retainSource": true
  },
  "concurrency": {
    "pageDownloads": 8,
    "chapterDownloads": 6
  },
  "search": {
    "historyMax": 100
  },
  "dirs": {
    "cache": "$cache_dir"
  }
}
EOF

  echo "[install] wrote default config to $config_file"
fi

if [[ ":${PATH}:" != *":$bin_dir:"* ]]; then
  echo "[install] warning: $bin_dir is not in PATH"
  echo "[install] add this to your shell rc if you want to run '$project_name' directly:"
  echo "  export PATH=\"$bin_dir:\$PATH\""
fi

echo "[install] done"
