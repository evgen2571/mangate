# Python API

The Python package runs its bundled Go executable as a subprocess. Python 3.10+ is required; released wheels don't require Go.

## Install

```sh
uv add mangate
# or, inside an activated environment:
pip install mangate
uv run mangate tui
```

Wheels cover Linux x86_64/ARM64 on glibc or musl, macOS 12+ x86_64/ARM64, and Windows x86_64. Other platforms get a clear unsupported-platform error; build Go yourself and pass `Client(executable="/path/to/mangate")` if supported by Go.

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

`output=` is the exact chapter directory, implemented by the Go downloader. Omit it to use the CLI's organized title/chapter hierarchy. An empty directory or an identified matching complete/incomplete chapter can be used; unrelated nonempty directories, different chapter/provider identities, files, and symlink destinations are rejected. `DownloadOptions` controls `existing_files` (`skip`, `replace`, `fail`), `output_format` (`directory`, `png`, `jpeg`), chapter `language`, and `page_downloads`. `fail` rejects nonempty chapter destinations before transfer; `skip` reuses matching pages and resumes missing ones. Automatic page-URL refresh retries can reuse files created by the current invocation; a new `fail` invocation rejects partial destinations too. Preflight failures leave existing image bytes and completion metadata unchanged. Replacement is explicit and may overwrite matching chapter pages; failed transfers keep existing finalized files. Archives and multiple-chapter selections remain available through `Client`.

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

An optional `Session(executable=None, provider="mangadex", timeout=None, output_dir=None)` reuses defaults for `search`, `chapters`, and `download`; each call still starts a separate bundled executable. Top-level functions also accept `provider`, `executable`, and `timeout`. A timeout bounds each subprocess invocation; cleanup allows two seconds for graceful exit and up to two more seconds to reap a forcibly terminated process. Both output streams are captured in temporary files, so inherited output handles cannot block cleanup. POSIX uses an isolated process group; Windows starts the process in a job that owns its descendants and uses a console break signal where available. Forced cleanup terminates the process group or job, including descendants left after the parent exits.

The existing dictionary-based `Client` remains compatible. New CLI chapter result fields (`chapter`, `imagePaths`) expose the same metadata and ordered filesystem paths as the typed API. Result status is `complete`, `partial`, `failed`, or `cancelled`; existing per-chapter `status` values such as `incomplete` and `skipped` are retained. Archives return `archivePath`; after source removal they have no image paths.

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

Manifest version 2 includes `ManifestImage.bytes` and `ManifestImage.sha256`. Go and Python can load/save both versions 1 and 2; local verification requires version 2 and verifies chapter identity/completion/page count as well as image checksums. Replay upgrades legacy manifests to version 2; plans remain version 1. Reporting failures retain available `DownloadError.result` and its acquisition manifest.

Provider-random selection is nondeterministic. A seed selects from a bounded catalog snapshot and records its seed/candidate IDs; catalog changes can change future selections. Replay preserves the chosen IDs. Chapters within a title use stable ID order. No visual scoring is performed. See the [CLI guide](cli.md#random-acquisition-planning-and-replay) for the shared selection and manifest semantics.

Progress callbacks run synchronously on the calling thread and receive frozen `ProgressEvent` values for discovery, selection, validation, downloading, and completion. Keep callbacks short; callback failures stop the owned subprocess. `cancel_event` and `timeout` cover discovery and selection as well as downloading through the same process-group/job cleanup as Stage A. `Session` also provides `plan_random`, `download_random`, and `execute`, reusing executable/timeout/provider defaults; random operations use the session output directory as an exact destination when no output is supplied. Automation formats are `directory`, `png`, and `jpeg`; a plan's format cannot be overridden on execution. `DownloadOptions.language`, if supplied, must agree with the random policy or selected chapter.

## Interactive chapter picker

Run this in a real terminal with stdin and stdout attached:

```python
from mangate import Client, MangateError

try:
    result = Client().interactive_download(
        output_dir="./chapters",
        max_chapters=1,
        timeout=600,
    )
    chapter = result["chapters"][0]
    print(result["title"]["title"], chapter["chapter"]["number"])
    images = chapter["imagePaths"]
    # Pass images to your processing pipeline here.
except MangateError as error:
    if error.category == "cancelled":
        print("Selection cancelled")
    else:
        print(error.category, error.message)
```

The existing search/title/chapter TUI lets the user select chapters and edit the proposed output directory. Integration mode saves image directories, displays progress, restores the terminal, and exits after the download attempt. `max_chapters=0` (the default) permits several chapters; each has its own sequence. `max_chapters=1` requires at most one. Optional `language` filters the TUI; `timeout` overrides the client's subprocess deadline.

The file contract is version `"1"`: `status`, selected `title` metadata, absolute `outputDir`, and `chapters`. Every chapter contains `chapter` metadata, `status` (`success` or `failed`), absolute `outputDir`, `expectedPages`, and `imagePaths` in page order. Successful paths point to existing nonempty files inside that chapter's directory. Chapter ordering is deterministic by number, then label and ID. Chapters are never concatenated.

Terminal stdin/stdout/stderr are inherited, and a private temporary result file is removed after reading. Only a complete validated download returns normally. `MangateError.category` distinguishes `terminal`, `executable`, `cancelled`, `timeout`, `partial`, `error`, and `invalid_result`. Cancellation or partial failure may provide a validated result through `error.result`; failed chapters expose no image paths. Missing or malformed results, unsafe paths, and contradictory exit statuses are rejected. Interrupted downloads retain files for retry, but aren't reported as complete. Python restores POSIX terminal settings even on a timeout; Bubble Tea performs its normal terminal cleanup on supported platforms.

## Search, inspect, and download

```python
from mangate import Client, MangateError

client = Client(output_dir="./library", timeout=120)

try:
    results = client.search("title", limit=5)
    if results:
        title_id = results[0]["id"]
        metadata = client.title(title_id)
        print(metadata["title"]["title"])
        chapters = client.chapters(title_id, language="en")
        if chapters:
            result = client.download(
                title_id,
                chapter_ids=[chapters[0]["id"]],
                output_format="cbz",
                assume_yes=True,
            )
            print(result["status"], result["chapters"])
except MangateError as error:
    print(error.category, error.message)
```

`search` returns a list of manga records and returns an empty list for a valid no-results response. `title` returns a record containing `provider` and `title`. `chapters` returns the chapter list. IDs identify specific releases; chapter numbers can be ambiguous.

## Client defaults

| Argument | Default | Meaning |
| --- | --- | --- |
| `executable` | `None` | Bundled executable; explicit command/path overrides it |
| `provider` | `"mangadex"` | Default provider |
| `output_dir` | `None` | Optional chapter output root |
| `timeout` | `None` | Per-command wrapper deadline in seconds |
| `page_downloads` | `None` | Optional page-transfer limit |
| `chapter_downloads` | `None` | Optional chapter-worker limit |
| `existing_files` | `"skip"` | `skip`, `replace`, or `fail` |
| `output_format` | `"directory"` | `directory`, `png`, `jpeg`, `cbz`, or `zip` |

The wrapper forwards its explicit defaults as CLI arguments. In particular, its format and existing-file defaults override saved CLI configuration. `timeout` bounds the whole subprocess operation; the Go configuration's `http.timeout` separately bounds requests.

`version()` returns text. `providers()` returns provider records, each containing `info` and `usable`. `provider_info()` returns one such record:

```python
from mangate import Client

client = Client()
print(client.version())
print(client.provider_info()["info"]["id"])
```

## Download selection

`download(title_id, ...)` supports `chapter_ids`, `chapter_numbers`, `chapter_range`, `first`, `latest`, `all_chapters`, and `language`. It also accepts `provider`, `output_format`, `dry_run`, `assume_yes`, `cancel_event`, and `output` (an exact single-chapter image directory).

Use one selection strategy, following the [CLI selection rules](cli.md). `dry_run=True` returns a plan. A `threading.Event` passed as `cancel_event` requests cancellation when set; inspect the resulting error or partial result.

## Local archive operations

```python
from mangate import Client

client = Client()
created = client.convert(
    "./chapter",
    output_format="cbz",
    output="./chapter.cbz",
)
print(created["includedPages"])
print(client.inspect_archive("./chapter.cbz")["pageCount"])
print(client.verify_archive("./chapter.cbz")["valid"])
```

The source must be an existing directory containing ordered page files. `convert` accepts `cbz` or `zip`, plus `remove_source`, `dry_run`, and `assume_yes`. Conversion keeps sources by default. `convert_many` processes directories in input order; an optional `outputs` sequence must have exactly the same length.

## Errors and partial results

`MangateError` is a `RuntimeError` with `category`, `message`, `stderr`, and optional `result`. JSON command failures, malformed JSON output, and wrapper cancellation/timeouts are reported through this exception. `version()` uses the same safe subprocess runner. Missing executables and process creation failures raise `MangateError` with category `executable`; timeouts raise category `timeout`. A supplied timeout must be finite and positive.

Partial download results are returned even though the CLI exits nonzero. Inspect `result["status"]` and each chapter's `status`, `outputPath`, and optional `archivePath`. A fully unsuccessful operation raises `MangateError`. Unsupported output formats and mismatched batch destinations raise `ValueError` before starting a subprocess.

## Dataset operations

The wrapper also exposes `dataset_plan`, `dataset_collect`, `dataset_status`, `dataset_verify`, and `dataset_export`. Plan and collect accept `collection_config`, `resume`, and additional CLI arguments through `extra_args`. Collect supports confirmation, dry-run, and cancellation. Verification can request repair; export can filter a split and include duplicates or rejected records.

Dataset collection has its own immutable plan and configuration. Follow the [dataset guide](dataset.md) before using these methods.
