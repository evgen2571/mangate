# Configuration

You don't need a config file to get started. Run `mangate config` to see the settings in use.

To save defaults, open **Settings** in the TUI or create `mangate/config.json` inside your OS's user config directory (usually `~/.config` on Linux). Set `MANGATE_CONFIG` to use a different file. CLI flags override the file; omitted fields keep their defaults. Unknown fields are errors.

## Example: save CBZ files by default

```json
{
  "download": {
    "dir": "./library",
    "format": "cbz",
    "existingFileMode": "skip"
  },
  "http": {"timeout": "1m"}
}
```

## What to change

| To change… | JSON field | CLI override |
| --- | --- | --- |
| Download location | `download.dir` | `--output` |
| File format | `download.format` | `--format` |
| What happens to existing files | `download.existingFileMode` | `--existing-files` |
| Provider or preferred text language | `provider`, `language` | `--provider`, `--language` |
| Parallel downloads | `concurrency.pageDownloads`, `concurrency.chapterDownloads` | `--page-downloads`, `--chapter-downloads` |
| Cache location or history size | `dirs.cache`, `search.historyMax` | `--cache-dir`, `--search-history-max` |
| Request timeout (default `30s`) | `http.timeout` | — |

Set `search.historyMax` to `0` to turn off search history. If you script `mangate --json config`, use `data.config`, not the entire response, as your config file.

[Dataset collection](dataset.md) has its own config file. Python's [`Session`](python.md) can pass its own overrides.
