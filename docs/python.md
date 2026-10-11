# Python API

The Python package runs its bundled Go executable as a subprocess. Python 3.10+ is required; released wheels don't require Go.

## Install

```sh
uv add mangate
uv run mangate tui
```

Wheels cover Linux x86_64/ARM64 on glibc or musl, macOS 12+ x86_64/ARM64, and Windows x86_64. Other platforms get a clear unsupported-platform error; build Go yourself and pass `Session(executable="/path/to/mangate")` if supported by Go.

To develop from the checkout, install Go 1.26+ and run `uv pip install ./python`. This builds a native wheel locally. Editable installs (`uv run --project python ...`) use a separately built executable on `PATH`, or an explicit executable override. No source distributions are published because the Python subdirectory alone cannot build the Go application.

## Function-first API

```python
import mangate

result = mangate.download(
    title="Solo Leveling",
    chapter=1,
    output="./output/chapter",
    timeout=120,
)
print(result.manga.title)
print(result.chapter.number)
print(result.image_paths)  # tuple of existing pathlib.Path objects in page order
```

A title string must uniquely match the primary title returned by search (case-insensitive). Ambiguous titles or chapter numbers fail rather than choosing a release silently. Stable IDs avoid that ambiguity:

```python
import mangate

matches = mangate.search("Solo Leveling", limit=5)
if matches:
    releases = mangate.chapters(matches[0], language="en")
    if releases:
        result = mangate.download(
            title_id=matches[0].id,
            chapter_id=releases[0].id,
            output="./chapter",
            options=mangate.DownloadOptions(existing_files="skip", page_downloads=4),
        )
        print(result.complete, result.pages)
```

`search()` returns `list[Manga]`; `chapters()` accepts a `Manga` or stable title ID and returns `list[Chapter]`. `download()` accepts exactly one of `title` (a name or `Manga`) and `title_id`, and exactly one of `chapter` (a number or `Chapter`) and `chapter_id`. It acquires one explicit chapter and returns `DownloadResult`. Provider IDs accompany manga and chapter metadata; chapter numbers stay strings to preserve fractional and nonnumeric labels. `Manga`, `Chapter`, `Page`, `DownloadResult`, and `DownloadOptions` are frozen dataclasses. `Page.index` is one-based and `Page.path` is an absolute filesystem path.

`output=` is the exact chapter directory, implemented by the Go downloader. Omit it to use the CLI's organized title/chapter hierarchy. An empty directory or an identified matching complete/incomplete chapter can be used; unrelated nonempty directories, different chapter/provider identities, files, and symlink destinations are rejected. `DownloadOptions` controls `existing_files` (`skip`, `replace`, `fail`), `output_format` (`directory`, `png`, `jpeg`), chapter `language`, and `page_downloads`. `fail` rejects nonempty chapter destinations before transfer; `skip` reuses matching pages and resumes missing ones. Automatic page-URL refresh retries can reuse files created by the current invocation; a new `fail` invocation rejects partial destinations too. Preflight failures leave existing image bytes and completion metadata unchanged. Replacement is explicit and may overwrite matching chapter pages; failed transfers keep existing finalized files. Use `archive.convert()` for archives and `download_many()` for multiple chapters.

`DownloadResult` includes `manga`, `chapter`, ordered `pages` and `image_paths`, `output`, `expected_pages`, `status`, `complete`, and `error`. Status is `complete`, `partial`, `failed`, or `cancelled`. Only complete acquisitions return normally. `DownloadError` extends `MangateError`; its `result` holds a typed result when available, including finalized pages from a partial chapter. Missing pages retain gaps in `Page.index`. Timeout and cancellation keep their exception category even when a partial record is available. Forced termination before the executable writes a result can leave `error.result` as `None`; downloaded files remain available for retry.

```python
from threading import Event
import mangate

cancel = Event()  # Another thread can call cancel.set().
try:
    result = mangate.download(
        title_id="stable-title-id", chapter_id="stable-chapter-id",
        output="./chapter", timeout=120, cancel_event=cancel,
    )
except mangate.DownloadError as error:
    print(error.category, error.message)
    if error.result is not None:
        print(error.result.status, error.result.image_paths)
```

An optional `Session(executable=None, provider="mangadex", timeout=None, output_dir=None)` reuses defaults for `search`, `title`, `chapters`, `download`, and `download_many`; each call still starts a separate bundled executable. Top-level functions also accept `provider`, `executable`, and `timeout`. A timeout bounds each subprocess invocation; cleanup allows two seconds for graceful exit and up to two more seconds to reap a forcibly terminated process. Both output streams are captured in temporary files, so inherited output handles cannot block cleanup. POSIX uses an isolated process group; Windows starts the process in a job that owns its descendants and uses a console break signal where available. Forced cleanup terminates the process group or job, including descendants left after the parent exits.

## Discovery and automation

```python
import mangate

result = mangate.download_random(
    output="./output/chapter", original_language="ko", chapter_language="en",
    min_pages=10, max_attempts=20, timeout=120, manifest="./chapter.json",
)
images = result.image_paths
```

Use individual policy keywords or one typed `SelectionPolicy`. It supports `provider`, `original_language`, `chapter_language`, `min_pages`, optional `max_pages`, inclusive numeric `chapter_min`/`chapter_max`, `max_attempts`, and optional signed 64-bit `seed`. Invalid policies fail before execution. Missing required metadata rejects candidates; unknown page counts are resolved before selection and inconsistent counts fail eligibility. There is no arbitrary-title fallback.

```python
from threading import Event
import mangate

cancel = Event()
policy = mangate.SelectionPolicy(
    original_language="ko", chapter_language="en", min_pages=10,
    max_pages=60, chapter_min=1, chapter_max=50, max_attempts=20, seed=42,
)

def progress(event: mangate.ProgressEvent):
    print(event.stage, event.status, event.completed_pages, event.expected_pages)
    # Another thread, or this callback, can call cancel.set().

plan = mangate.plan_random(
    "./planned-chapter", policy=policy, timeout=120,
    cancel_event=cancel, progress=progress, manifest="./selection.json",
)
result = mangate.execute(
    plan, options=mangate.DownloadOptions(existing_files="skip"),
    timeout=120, cancel_event=cancel, progress=progress, manifest="./acquired.json",
)
loaded = mangate.load_manifest("./acquired.json")
replayed = mangate.execute(loaded, output="./replayed-chapter", timeout=120)
verified = mangate.verify_manifest("./acquired.json")  # Local files only.
mangate.save_manifest(plan, "./another-selection.json")
```

`plan_random()` downloads no images and returns a frozen `DownloadPlan` with stable manga/chapter IDs and metadata, policy, strategy, candidate IDs, expected page count, exact output, format, and validation state. `execute()` accepts a plan, an `AcquisitionManifest`, or a manifest path; it revalidates the same selection without discovery. Changed eligibility or counts fail before image transfer, including during URL refresh. `download_random()` performs planning and execution in one subprocess and returns the same `DownloadResult` as explicit downloads. Random/replayed results additionally carry `result.manifest`; `DownloadError.result` preserves finalized partial pages and their manifest when available. `manifest=` saves planned, complete, or incomplete acquisitions atomically. Save manifests outside the chapter directory. Invalid downloaded images retain an incomplete manifest with `format="unknown"` and cannot count as a complete acquisition.

Manifest image paths are relative filenames: resolve them against `manifest.plan.output`, or use the absolute ordered `result.image_paths` for acquisition consumers. Manifest version 2 includes `ManifestImage.bytes` and `ManifestImage.sha256`. Go and Python can load/save both versions 1 and 2; local verification requires version 2 and verifies chapter identity/completion/page count as well as image checksums. Replay upgrades legacy manifests to version 2; plans remain version 1. Reporting failures retain available `DownloadError.result` and its acquisition manifest.

Provider-random selection is nondeterministic. A seed selects from a bounded catalog snapshot and records its seed/candidate IDs; catalog changes can change future selections. Replay preserves the chosen IDs. Chapters within a title use stable ID order. No visual scoring is performed. See the [CLI guide](cli.md#random-acquisition-planning-and-replay) for the shared selection and manifest semantics.

Progress callbacks run synchronously on the calling thread and receive frozen `ProgressEvent` values for discovery, selection, validation, downloading, and completion. Keep callbacks short; callback failures stop the owned subprocess. `cancel_event` and `timeout` cover discovery and selection as well as downloading through the same process-group/job cleanup as Stage A. `Session` also provides `plan_random`, `download_random`, and `execute`, reusing executable/timeout/provider defaults; random operations use the session output directory as an exact destination when no output is supplied. Automation formats are `directory`, `png`, and `jpeg`; a plan's format cannot be overridden on execution. `DownloadOptions.language`, if supplied, must agree with the random policy or selected chapter.

## Interactive selection

```python
import mangate

session = mangate.Session(provider="mangadex", timeout=600, output_dir="./chapters")
result = session.interactive_download(max_chapters=1, language="en")
for download in result.downloads:
    print(download.manga.title, download.chapter.number, download.image_paths)
```

`interactive_download()` launches the Go Bubble Tea TUI, inherits terminal streams, and reads its version 1 result-file protocol. A real terminal on stdin and stdout is required. `max_chapters=1` limits selection to one chapter; `0` allows unlimited selection. Search, metadata, cover preview, chapter selection, and downloading use the existing Go implementation. Each chapter returns the same `DownloadResult` used by headless acquisition. `InteractiveResult.downloads` is an ordered tuple; its status is `complete`, `partial`, `failed`, or `cancelled`.

Explicit user cancellation raises `DownloadError(category="cancelled")`; partial download failures use `partial`. Completed chapters remain in `error.result.downloads`. A timeout uses category `timeout`. If a valid handoff exists after shutdown, it is retained. Forced termination before the TUI writes a handoff leaves `error.result=None`. Terminal settings and temporary result files are restored/removed in all exit paths; process groups or Windows jobs own and clean up descendants.

## Reusable configuration and metadata

`Session` supports search, stable-ID title lookup, chapters, explicit and batch downloads, random downloads, planning, replay, and interactive downloads. It starts no persistent process, so no context manager or close call is needed. All operations use the same implementation as top-level functions.

```python
import mangate

print(mangate.cli_version())
for provider in mangate.providers():
    print(provider.id, provider.capabilities, provider.usable)
print(mangate.provider_info("mangadex").download_permitted)
```

`title(title_id)` retrieves a typed manga by stable ID without search. Provider metadata is returned as frozen `ProviderInfo` models. Functions accept `executable=` and `timeout=` overrides. Unsupported platforms and missing executables raise `MangateError` with category `executable`.

## Multiple chapters and archives

```python
import mangate

batch = mangate.download_many(
    "stable-title-id", chapter_ids=("first-chapter-id", "second-chapter-id"),
    output_dir="./library", timeout=120,
)
for chapter in batch.downloads:
    archived = mangate.archive.convert(chapter.output, output_format="cbz")
    print(archived.output_path, archived.included_pages)
    print(mangate.archive.verify(archived.output_path).valid)
```

`download_many()` accepts stable `chapter_ids`, `chapter_numbers`, `chapter_range`, `first`, `latest`, or `all_chapters`. Go validates incompatible or ambiguous selections. Use `assume_yes=True` to confirm broad selections (25 or more chapters). `Session.download_many()` applies the same selections with session provider, executable, timeout, and output-root defaults. Results use `InteractiveResult` for the shared ordered chapter batch contract. Partial failures raise `DownloadError` with the completed chapters retained.

`archive.convert()` returns `Conversion`; `dry_run=True` returns `ConversionPlan` without writing an archive. `output=` sets the destination. `remove_source=True` requires `assume_yes=True` and deletes source pages only after archive validation. `archive.inspect()` and `archive.verify()` return `Inspection`, including validation, entries, and optional typed metadata. Batch conversion is a loop over `convert()`. Acquisition always returns page files; archive conversion is a separate explicit operation.

## Dataset workflows

```python
from mangate import dataset

planned = dataset.plan(collection_config="./dataset-config.json")
print(planned.plan.estimated_pages)
collected = dataset.collect(collection_config="./dataset-config.json", resume=True)
print(collected.counters.valid_pages)
info = dataset.status("./dataset")
verified = dataset.verify("./dataset")
exported = dataset.export("./dataset", split="train")
```

These functions return frozen typed records (`PlanResult`, `CollectResult`, `DatasetInfo`, `Verification`, and `ExportResult`). Nested counters and configuration are typed; split counts are immutable mappings. Use the existing versioned collection configuration and [dataset guide](dataset.md). Planning persists a plan without downloading images. Collection supports `resume`, `dry_run`, cancellation, provider/output overrides, and a per-invocation timeout. Advanced Go flags can be supplied through `extra_args`; collection confirms writes by default (`assume_yes=True`). `verify(repair=True)` repairs local state without provider downloads.

## Error contract

Invalid Python arguments raise `ValueError`. CLI failures raise `MangateError` with `category`, `message`, and captured `stderr`. Categories include `invalid_input`, `unknown_provider`, `not_found`, `unsupported_operation`, `network`, `filesystem`, `archive`, `cancelled`, `timeout`, `executable`, `invalid_result`, and `internal`; acquisition-specific failures also include `no_candidate`, `partial`, and `failed`. Malformed/version-incompatible machine responses raise `invalid_result`. Interactive terminal failures use `terminal`.

Acquisition operations raise `DownloadError`, a `MangateError` subclass. `result` contains a validated `DownloadResult` or `InteractiveResult` when available. A successful result always contains existing nonempty paths in page order. Incomplete downloads never return normally; rerun with `existing_files="skip"` to recover matching finalized pages. Failed chapters may have no pages, and partial chapters preserve original page indices. Ordinary headless operations never launch terminal interaction.
