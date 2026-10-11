# Configuration

Mangate works without a configuration file. Defaults use MangaDex, English text where available, original image files in directories, eight page transfers, six chapter workers, and a 30-second HTTP timeout.

## File location and precedence

Set `MANGATE_CONFIG` to choose an explicit JSON file. Otherwise Mangate uses the operating system's user configuration directory followed by `mangate/config.json`. On Linux this normally follows `XDG_CONFIG_HOME` or `~/.config`.

Defaults are applied first, then fields from the configuration file, then explicit CLI flags. Unknown JSON fields and invalid values are errors. A missing configuration file uses defaults. `MANGATE_CONFIG` selects the file; there are no general environment-variable overrides for individual options.

Provider, language, format, and existing-file values are trimmed and lowercased. Filesystem paths retain their spelling. The application checks provider names against its registry. Saving settings writes a temporary file in the configuration directory before replacing the previous file.

Inspect effective settings:

```sh
mangate config
mangate --json config
mangate --format cbz --output ./library config
```

The JSON command wraps the configuration in `data.config`; extract that object before using it as a configuration file. HTTP timeout values use duration strings such as `30s` or `2m`.

## Example file

Paths below are examples; choose directories writable by your account.

```json
{
  "provider": "mangadex",
  "language": "en",
  "http": {"timeout": "30s"},
  "download": {
    "dir": "./library",
    "format": "cbz",
    "existingFileMode": "skip"
  },
  "concurrency": {"pageDownloads": 8, "chapterDownloads": 6},
  "search": {"historyMax": 100},
  "dirs": {"cache": "./cache"}
}
```

## Option mapping

| JSON field | CLI flag | Meaning |
| --- | --- | --- |
| `provider` | `--provider` | Registered provider identifier |
| `language` | `--language` | Preferred text language; explicitly passing the flag also filters supported CLI/TUI search workflows |
| `download.dir` | `--output`, `--download-dir` | Root directory for chapter downloads |
| `download.format` | `--format` | `directory`, `png`, `jpeg`, `cbz`, or `zip` |
| `download.existingFileMode` | `--existing-files` | `skip`, `replace`, or `fail` |
| `concurrency.pageDownloads` | `--page-downloads` | Positive global page-transfer limit |
| `concurrency.chapterDownloads` | `--chapter-downloads` | Positive chapter-worker limit |
| `search.historyMax` | `--search-history-max` | Nonnegative history limit; zero disables history |
| `dirs.cache` | `--cache-dir` | Cover and history cache directory |
| `http.timeout` | None | Positive HTTP/request timeout |

Standalone archive conversion has its own `--output` flag for the destination archive file. Dataset commands use `--output` as the dataset root and have a separate collection configuration; see [dataset collection](dataset.md).

Provider endpoints may be configured under `providers.mangadex` with `siteUrl`, `baseUrl`, and `uploadsUrl`. Each must be an HTTP or HTTPS URL. This is useful for controlled integration fixtures; normal use should keep the defaults.

The TUI settings screen can apply settings for the current session or save them to the configured file. See the on-screen key bindings. Python's `Session` forwards its own explicit defaults and options as CLI flags; consult the [Python guide](python.md) rather than assuming it inherits every saved default.
