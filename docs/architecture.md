# Architecture and development

Mangate has one Go executable and a Python subprocess wrapper. Go implementation packages live under `internal`; there is no supported public Go library API.

## Request and download flow

`cmd/mangate` loads configuration, creates the application, installs signal cancellation, and executes the Cobra root command. `internal/app` owns the HTTP client, provider registry, downloader, and cache. Applying configuration rebuilds those dependencies when values change.

`internal/providers` registers provider factories. The MangaDex adapter maps remote responses into the types in `internal/source`. Search, title, chapter, cover, page-location, and catalog requests share response validation and bounded retries. Chapter and search collection methods follow pagination; catalog browsing returns one page for dataset planning.

`internal/usecase` coordinates browsing, chapter listing, and cover caching behind small interfaces. `internal/acquisition.Service` owns selected-chapter execution, destination policies, progress, result normalization, and archive finalization. It copies selected chapters into a download-owned manga so page discovery does not modify the UI's selection. Request contexts and HTTP timeouts flow down to the provider and downloader.

`internal/downloader` limits chapter workers and shares a global page-transfer limit across them. It discovers page URLs lazily, refreshes them after a forbidden image response, and writes page files through `.part` files. Failed chapters retain completed pages and record incomplete state in `.mangate.json`. Ordinary chapter directories and exact destinations use the same chapter-transfer loop, including preflight, retries, partial state, conversion, and atomic finalization. Progress callbacks are serialized, identify chapters independently of display names, and never count recovered pages twice during URL refresh.

`internal/archive` creates, inspects, and verifies CBZ/ZIP files. The acquisition service finalizes completed chapter directories into archives, including completed siblings of failed chapters. Source pages are removed after archive validation. Existing valid archives can be reused without downloading their pages again.

`internal/acquisition` owns typed selection policies, bounded random/catalog discovery, stable plans, and versioned replay manifests. Its service revalidates those plans and executes them through the same acquisition lifecycle as explicitly selected chapters. Refresh loaders retain the plan's eligibility checks. Results retain the existing chapter status strings (`complete`, `skipped`, `incomplete`, `failed`, `archive_failed`) and normalize the overall outcome to `complete`, `partial`, `failed`, or `cancelled`; plans use `planned`. For random/replay results, the single chapter retains its established overall-status spelling (`partial` rather than directory acquisition's `incomplete`). Finalized files remain available on validation failure for inspection and recovery; they cannot mark a chapter complete or become successful TUI integration images. Preflight rejection never exposes files from an earlier completion as the rejected request's output. CLI automation owns manifest persistence and JSON Lines delivery, including reporting failures. Python progress uses an event file drained by the ordinary subprocess runner. Acquisition remains separate from dataset collection and starts no daemon.

`internal/cache` caches validated covers and search history. Concurrent requests for the same cover destination share one transfer; waiting callers can cancel independently. Corrupt cached covers are downloaded again.

## User interfaces

`internal/cli` handles arguments, chapter selection, confirmation, text/JSON output, exit codes, standalone archive commands, and dataset commands. Explicit, random, and replay downloads call the application's acquisition service; they do not own transfer or archive lifecycles. The terminal interface in `internal/interactive` is a Bubble Tea state machine. Commands invoke browsing or acquisition services and return messages; the model owns selection, navigation, and rendering. Its file-only integration result maps the shared acquisition result to the existing `success`/`failed` presentation contract. Cover responses carry a request identifier so stale results cannot replace the current selection.

`python/src/mangate` invokes the executable and decodes its JSON envelopes. It does not implement providers or image downloads. Released wheels bundle the executable; editable checkouts use an explicit path or a separately built executable. Public functions and Session share acquisition operations. Private executable resolution, process/JSON transport, terminal integration, and result decoders have separate modules; archive and dataset operations expose focused typed modules. Public acquisition operations use typed models.

`internal/dataset` adds bounded planning, resumable JSON state, validation, and exports on top of provider browsing and the downloader. Dataset collection is separate from ordinary chapter downloads. See [dataset collection](dataset.md).

## Verify a change

From the repository root, with Go and uv installed:

```sh
bash scripts/check.sh
go test -race ./internal/acquisition ./internal/downloader ./internal/cache ./internal/providers/... ./internal/usecase ./internal/interactive ./internal/dataset
uv run --project python --locked python scripts/install_smoke.py
```

The check script runs gofmt checks, Ruff lint/format checks, dependency tidiness, `go vet`, Go tests, Go builds, and Python tests. The Go integration suite compiles the actual CLI and shares `internal/testprovider`'s local HTTP fixture with acquisition and TUI boundary tests. Python integration tests also build and invoke the executable using a shared `provider_fixture.py`. Recovery, preflight conflicts, cancellation, conversion, URL refresh, progress ordering, concurrent archive naming, random/replay selection, and manifest integrity are covered without live MangaDex responses. `go test ./internal/acquisition -run '^$' -bench BenchmarkExecute -benchmem` measures ordinary and exact chapter execution against the local fixture. The installation smoke test uses temporary binary and Python environment directories, then imports the installed package outside the repository.

The GitHub Actions workflow runs on pushes and pull requests with Linux, macOS, and Windows jobs. These automated fixtures do not prove live provider availability or terminal rendering. Those need separate runtime verification.

## Configuration ownership

`internal/config` owns defaults, file parsing, serialization, and structural validation. CLI flags bind to the application configuration. Provider-specific construction belongs to the registry; remote response validation belongs to the adapter. Filesystem errors belong to the code performing the write. Keep error context as errors cross these boundaries so users can identify the operation and destination that failed.
