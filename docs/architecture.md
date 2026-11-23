# Development

Mangate is one Go executable with a Python wrapper. The CLI, TUI, and Python API share the same provider and download code. There's no public Go library.

## Build and test

From the repository root, with Go 1.26+ and uv installed:

```sh
go install ./cmd/mangate
bash scripts/check.sh
```

`check.sh` checks Go and Python formatting, runs lint and tests, builds Go packages, and validates documentation examples. Integration tests use local provider fixtures, not the live MangaDex API.

For Python development, `uv run --project python --locked pytest python/tests` runs just the Python tests. A local wheel build (`uv pip install ./python` in a virtual environment) bundles the executable. For editable installs, put the separately built executable on `PATH` or pass `executable=`.

## Where to make changes

| If you're changing… | Start here |
| --- | --- |
| Command flags or JSON output | `cmd/mangate`, `internal/cli` |
| Screens, keyboard controls, or terminal rendering | `internal/interactive` |
| Provider requests or response parsing | `internal/providers/mangadex` |
| Chapter selection, plans, destinations, or archive finalization | `internal/acquisition` |
| Image transfers, retries, or on-disk chapter state | `internal/downloader` |
| Dataset planning, validation, state, or exports | `internal/dataset` |
| Python calls or result types | `python/src/mangate` |

Keep chapter transfer and recovery behavior in shared Go code so all three interfaces benefit. Python starts the executable and reads its results; it doesn't talk to MangaDex itself.
