# Architecture and development

Mangate has one Go executable and a Python subprocess wrapper. Go implementation packages live under `internal`; there is no supported public Go library API.

## Request and download flow

`cmd/mangate` loads configuration, creates the application, installs signal cancellation, and executes the Cobra root command. `internal/app` owns the HTTP client, provider registry, downloader, and cache. Applying configuration rebuilds those dependencies when values change.

`internal/providers` registers provider factories. The MangaDex adapter maps remote responses into the types in `internal/source`. Search, title, chapter, cover, page-location, and catalog requests share response validation and bounded retries. Chapter and search collection methods follow pagination; catalog browsing returns one page for dataset planning.

`internal/usecase` coordinates provider access, cover caching, and chapter downloads behind small interfaces. It copies selected chapters into a download-owned manga so page discovery does not modify the UI's selection. Request contexts and HTTP timeouts flow down to the provider and downloader.

`internal/downloader` limits chapter workers and shares a global page-transfer limit across them. It discovers page URLs lazily, refreshes them after a forbidden image response, and writes page files through `.part` files. Failed chapters retain completed pages and record incomplete state in `.mangate.json`. Progress snapshots identify chapters independently of display names.

`internal/archive` creates, inspects, and verifies CBZ/ZIP files. CLI and TUI download commands finalize completed chapter directories into archives. Source pages are removed after archive validation. Existing valid archives can be reused without downloading their pages again.

`internal/cache` caches validated covers and search history. Concurrent requests for the same cover destination share one transfer; waiting callers can cancel independently. Corrupt cached covers are downloaded again.

## User interfaces

`internal/cli` handles arguments, chapter selection, text/JSON output, exit codes, archive conversion, and dataset commands. The terminal interface in `internal/interactive` is a Bubble Tea state machine. Commands perform I/O and return messages; the model owns selection, navigation, and rendering. Cover responses carry a request identifier so stale results cannot replace the current selection.

`python/src/mangate` invokes the executable and decodes its JSON envelopes. It does not implement providers or image downloads. Python callers therefore need both the package and a discoverable Mangate executable.

`internal/dataset` adds bounded planning, resumable JSON state, validation, and exports on top of provider browsing and the downloader. Dataset collection is separate from ordinary chapter downloads. See [dataset collection](dataset.md).

## Verify a change

From the repository root, with Go and uv installed:

```sh
bash scripts/check.sh
go test -race ./internal/downloader ./internal/cache ./internal/providers/... ./internal/usecase ./internal/interactive
uv run --project python --locked python scripts/install_smoke.py
```

The check script runs gofmt checks, Ruff lint/format checks, dependency tidiness, `go vet`, Go tests, Go builds, and Python tests. The Go integration suite compiles the actual CLI and exercises a local HTTP fixture. Python integration tests also build and invoke the executable. The installation smoke test uses temporary binary and Python environment directories, then imports the installed package outside the repository.

The GitHub Actions workflow runs on pushes and pull requests with Linux, macOS, and Windows jobs. These automated fixtures do not prove live provider availability or terminal rendering. Those need separate runtime verification.

## Configuration ownership

`internal/config` owns defaults, file parsing, serialization, and structural validation. CLI flags bind to the application configuration. Provider-specific construction belongs to the registry; remote response validation belongs to the adapter. Filesystem errors belong to the code performing the write. Keep error context as errors cross these boundaries so users can identify the operation and destination that failed.
