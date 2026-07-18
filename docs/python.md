# Python bindings

The Mangate Python package exposes provider lookup, search, title, chapter, download, and archive operations without requiring callers to parse terminal output. It runs a compatible `mangate` executable as a separate process and reads its JSON results.

## Installation

The package is currently installed from this repository:

~~~bash
python -m pip install ./python
~~~

Install the Go executable first, or pass its path to `Client`. By default, the package runs the command named `mangate` from `PATH`.

The package declares Python 3.10 or newer and currently supports Linux. The repository's package metadata lists Python 3.10 through 3.13. The CLI and Python package are intended to be used from the same Mangate 0.1.x release family.

Verify the package import with:

~~~bash
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=python/src python -c 'from mangate import Client, MangateError, __version__; print(__version__)'
~~~

There is no published package or platform-specific wheel documented by this repository. The Go executable remains a runtime requirement.

## Quick start

This example searches, lists chapters, and downloads the first returned chapter as a directory:

~~~python
from mangate import Client, MangateError

client = Client(executable="./mangate")

try:
    titles = client.search("example title", limit=5)
    if titles:
        title_id = titles[0]["id"]
        chapters = client.chapters(title_id)
        if chapters:
            result = client.download(
                title_id,
                chapter_ids=[chapters[0]["id"]],
            )
            print(result["chapters"][0]["outputPath"])
except MangateError as error:
    print(error.category, error.message)
~~~

The example uses a stable title ID and chapter ID returned by the provider. It does not parse human-readable CLI output.

## Public API principles

All public operations are synchronous and block until the child executable returns, the operation is cancelled, or the configured timeout expires. There is no public async client.

Each method call starts a separate process. A `Client` stores its default command settings but does not own a shared mutable operation object. The package documentation and tests support using one client from multiple Python threads. Calls that write to the same paths still share the filesystem, so use separate output directories when running independent downloads concurrently.

Results are ordinary Python dictionaries and lists decoded from Mangate's JSON output. The package does not export result model classes or enums. Optional values appear as missing dictionary keys or as empty values according to the CLI payload.

The CLI owns provider access, page downloads, archive validation, and durable local page state. The Python package exposes those operations and their structured results.

## Package overview

The public package is small:

| Import | Purpose |
| --- | --- |
| `mangate.Client` | Synchronous client for all supported operations. |
| `mangate.MangateError` | One structured exception class for operation failures. |
| `mangate.__version__` | Python package version string. |
| `mangate.client` | Module that defines `Client` and `MangateError`. |

Only `Client`, `MangateError`, and `__version__` are exported from the package root.

## Dataset collection

The Python client forwards dataset operations to the Go CLI. It does not open
the dataset database or make provider requests itself.

```python
plan = client.dataset_plan(collection_config="dataset.json")
result = client.dataset_collect(collection_config="dataset.json", resume=True)
status = client.dataset_status("./datasets/manhwa-raw-v1")
verification = client.dataset_verify("./datasets/manhwa-raw-v1")
exports = client.dataset_export("./datasets/manhwa-raw-v1")
```

`dataset_collect` accepts `cancel_event` and preserves a returned partial
result. Dataset output formats are `directory`, `png`, `jpeg`, `cbz`, and
`zip`.

## Creating and configuring a client

Create a client with:

~~~text
Client(
    executable="mangate",
    *,
    provider="mangadex",
    output_dir=None,
    timeout=None,
    page_downloads=None,
    chapter_downloads=None,
    existing_files="skip",
    output_format="directory",
    retain_source=True,
)
~~~

Arguments:

- `executable` is a command name or a `str` or `os.PathLike` path to a compatible executable. The default is `mangate`.
- `provider` sets the default provider ID. The default is `mangadex`.
- `output_dir` sets the default download root. It accepts a path-like value and is otherwise unset, which lets the executable use its configured directory.
- `timeout` is a per-command timeout in seconds. `None` disables the Python-side deadline.
- `page_downloads` and `chapter_downloads` override the executable's concurrency settings when set.
- `existing_files` accepts `skip`, `replace`, or `fail`. The default is `skip`.
- `output_format` accepts `directory`, `png`, `jpeg`, `cbz`, or `zip`. The default is `directory`.
- `retain_source` defaults to true and controls source page directories after archive creation.

The constructor raises `ValueError` for an unsupported existing-file policy or output format. It does not check that the executable exists until a method starts a process.

~~~python
from pathlib import Path
from mangate import Client

client = Client(
    executable=Path("./mangate"),
    provider="mangadex",
    output_dir=Path("./library"),
    output_format="cbz",
    existing_files="skip",
    timeout=60,
    page_downloads=4,
    chapter_downloads=2,
)
~~~

Per-call provider, format, retention, timeout, and chapter selection options are described in the method sections below. The output directory, existing-file policy, and concurrency defaults come from the client constructor.

## Providers

### List providers

~~~text
Client.providers() -> list[dict[str, Any]]
~~~

Returns one record for each registered provider. A record has:

- `info`, a provider information dictionary.
- `usable`, a boolean indicating whether the provider could be constructed.
- `error`, an optional construction error.

~~~python
for record in client.providers():
    info = record["info"]
    print(info["id"], info["name"], info["capabilities"])
~~~

### Inspect one provider

~~~text
Client.provider_info(provider: str | None = None) -> dict[str, Any]
~~~

The optional provider ID overrides the client's default. The returned record has the same `info` and `usable` fields as a provider-list record and may include `error`.

The provider information dictionary contains:

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | Stable provider identifier. |
| `name` | string | Display name. |
| `description` | string | Public provider description. |
| `version` | string | Provider-adapter version label when supplied. |
| `capabilities` | list of strings | Operations the provider declares, such as `search`, `title`, `chapters`, `pages`, and `download`. |
| `authentication` | string | Provider authentication description. |
| `restrictions` | list of strings | Public usage restrictions. |
| `downloadPermitted` | boolean | Whether Mangate permits download operations through the provider. |
| `availability` | string | Provider availability reported by Mangate. |

MangaDex is currently the only bundled provider. Its ID is `mangadex`, it declares the search, title, chapters, pages, and download capabilities, and it reports authentication as optional. The current integration has no credential setting. Provider access and availability can change independently of the package.

## Searching

~~~text
Client.search(
    query: str,
    *,
    provider: str | None = None,
    limit: int | None = None,
) -> list[dict[str, Any]]
~~~

Searches one provider by title. The query must contain a non-empty title. The optional provider overrides the client's default. The optional limit is passed to the CLI when it is not `None`.

Each returned title-summary dictionary can contain:

| Field | Type | Optional | Meaning |
| --- | --- | --- | --- |
| `id` | string | No | Stable provider title reference. |
| `url` | string | Yes | Provider title URL. |
| `title` | string | No | Display title selected by the provider. |
| `cover` | dictionary | Yes | Cover URL and filename when supplied. |
| `metadata` | dictionary | No | Title metadata. |

The `metadata` dictionary can contain:

- `description`, a language-to-description dictionary.
- `alternativeTitle`, a display alternative title.
- `chapterCount`, when supplied by a provider.
- `status`, such as an ongoing or completed status.
- `contentType`, such as a provider content rating.
- `language`, the original language.
- `year`, the publication year when known.

The MangaDex provider maps its content rating into `contentType` and its original language into `language`. Missing metadata keys are omitted.

An empty search result returns an empty Python list. It does not raise `MangateError`. A provider failure, timeout, invalid executable, or malformed response is an error.

~~~python
titles = client.search("example title", provider="mangadex", limit=5)
for title in titles:
    print(title["id"], title["title"], title["metadata"].get("language"))
~~~

The Python search method currently exposes no content-type or language-filter keyword. Apply an additional filter to the returned dictionaries in Python, or use the CLI's search flags.

## Title information

~~~text
Client.title(
    title_id: str,
    *,
    provider: str | None = None,
) -> dict[str, Any]
~~~

Retrieves full title data for a stable provider title ID. The optional provider overrides the client's default.

The returned dictionary has:

- `provider`, the provider ID used for the request.
- `title`, a title dictionary with the same title and metadata fields described in [Searching](#searching).

~~~python
title_record = client.title("title-id")
title = title_record["title"]
print(title["title"], title["metadata"].get("description"))
~~~

An unknown title raises `MangateError` with category `not_found` when the executable reports a not-found error. Ambiguous display names are not resolved by this method. Pass the stable ID returned by `search`.

## Chapters

~~~text
Client.chapters(
    title_id: str,
    *,
    provider: str | None = None,
    limit: int | None = None,
) -> list[dict[str, Any]]
~~~

Lists chapters for a stable title ID. The optional provider and limit override the client defaults for this call.

Each chapter dictionary can contain:

| Field | Type | Optional | Meaning |
| --- | --- | --- | --- |
| `id` | string | No | Stable provider release ID. |
| `number` | string | Yes | Provider chapter label. |
| `title` | string | Yes | Chapter title. |
| `volume` | string | Yes | Volume label. |
| `language` | string | Yes | Translated language. |
| `releaseGroup` | string | Yes | Release group when supplied. |
| `publishedAt` | string | Yes | Provider publication timestamp. |
| `pageCount` | integer | Yes | Provider-reported page count. |
| `url` | string | Yes | Provider chapter URL. |

MangaDex returns all available languages and orders numeric chapter labels numerically. Decimal labels remain strings in the returned dictionary. Unnumbered MangaDex chapters are mapped to `0`. Duplicate numbers are separate releases, so select by `id` when a number is ambiguous. Pass `language="en"` to `client.chapters()` to request one language only.

~~~python
chapters = client.chapters("title-id")
for chapter in chapters:
    if chapter.get("language") == "en":
        print(chapter["number"], chapter["id"], chapter.get("releaseGroup"))
~~~

An empty chapter list is returned as an empty list. A not-found or provider error raises `MangateError`.

## Chapter selection

Python downloads accept programmatic chapter selectors rather than a CLI selector string.

| Keyword | Type | Meaning |
| --- | --- | --- |
| `chapter_ids` | iterable of strings | Select exact stable chapter IDs. |
| `chapter_numbers` | iterable of strings | Select chapter labels that identify one release each. |
| `chapter_range` | string or None | Inclusive `START-END` range. |
| `first` | bool | Select the first provider-ordered chapter. |
| `latest` | bool | Select the last provider-ordered chapter. This is a direct API operation, not a TUI action. |
| `all_chapters` | bool | Select all accessible chapters. |
| `language` | string or None | Select an exact provider language value before selection. |

The CLI validates combinations. The Python method passes these selectors to the CLI, so conflicting selectors and ambiguous chapter numbers result in a `MangateError` with category `invalid_input` rather than a Python-side selection object.

~~~python
result = client.download(
    "title-id",
    chapter_ids=["chapter-a", "chapter-b"],
    output_format="cbz",
)
~~~

Use stable IDs when a number appears more than once:

~~~python
chapters = client.chapters("title-id")
same_number = [chapter for chapter in chapters if chapter.get("number") == "1"]
if len(same_number) == 1:
    result = client.download("title-id", chapter_ids=[same_number[0]["id"]])
~~~

## Downloading

~~~text
Client.download(
    title_id: str,
    *,
    provider: str | None = None,
    chapter_ids: Iterable[str] = (),
    chapter_numbers: Iterable[str] = (),
    chapter_range: str | None = None,
    first: bool = False,
    latest: bool = False,
    all_chapters: bool = False,
    language: str | None = None,
    output_format: str | None = None,
    retain_source: bool | None = None,
    dry_run: bool = False,
    assume_yes: bool = False,
    cancel_event: Event | None = None,
) -> dict[str, Any]
~~~

The title ID is required. Select at least one chapter. The optional provider, language, output format, source-retention setting, dry-run setting, acknowledgement, and cancellation event apply only to this call.

The client's output directory, existing-file policy, page concurrency, and chapter concurrency remain the defaults configured on the client. To change them for another operation, create another client or configure the executable.

### Common downloads

~~~python
client = Client(output_dir="./library")

directory_result = client.download(
    "title-id",
    chapter_numbers=["1"],
    output_format="directory",
)

cbz_result = client.download(
    "title-id",
    chapter_ids=["chapter-id"],
    output_format="cbz",
)

zip_result = client.download(
    "title-id",
    chapter_range="1-10",
    output_format="zip",
    assume_yes=True,
)
~~~

The `assume_yes` argument is required for broad selections of 25 or more chapters, replacement mode, and source removal after archive creation. Use `dry_run=True` first to resolve paths without downloading:

~~~python
plan = client.download(
    "title-id",
    latest=True,
    output_format="cbz",
    dry_run=True,
    assume_yes=True,
)
print(plan["status"], plan["chapters"])
~~~

The direct Python API has no prompt. `assume_yes=True` is an acknowledgement, not an instruction to answer an interactive question.

## Download result objects

Download returns the `data` dictionary from the CLI JSON envelope. Its public fields are:

| Field | Type | Meaning |
| --- | --- | --- |
| `provider` | string | Provider ID used. |
| `title` | dictionary | Full title data. |
| `format` | string | Requested output format. |
| `outputRoot` | string | Effective download directory. |
| `status` | string | Overall operation state. |
| `startedAt` | string | UTC start timestamp. |
| `completedAt` | string | UTC completion timestamp. |
| `chapters` | list | Per-chapter result dictionaries. |
| `error` | string | Optional operation error text for partial results. |

Each per-chapter dictionary can contain:

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | Stable chapter ID. |
| `number` | string | Chapter label when present. |
| `title` | string | Chapter title when present. |
| `status` | string | Chapter state. |
| `outputPath` | string | Page-directory path. |
| `archivePath` | string | Archive path for CBZ or ZIP output. |
| `expectedPages` | integer | Expected page count when known. |
| `validation` | dictionary | Archive validation data when an archive was created or reused. |

Common chapter states are `planned`, `pending`, `complete`, `skipped`, `incomplete`, and `archive_failed`. A skipped result is a validated reused output, not a newly downloaded chapter.

The overall status is normally `complete` in the returned data. A multi-chapter operation with completed and failed chapters returns `partial` and preserves completed paths. A dry run returns `planned`. Calls that return an unrecoverable process error raise `MangateError` instead of returning a result.

## Output formats

The accepted Python strings are `directory`, `cbz`, and `zip`. The package does not currently export a format enum or constants.

| Value | Output |
| --- | --- |
| `directory` | Ordered image files in one directory per chapter. |
| `cbz` | One `.cbz` comic-book archive per chapter. |
| `zip` | One ordinary `.zip` archive per chapter. |

The client constructor sets a default format. `download(output_format=...)` overrides it for one call. The result's `format` field reports the requested format.

## Existing-file policies

The accepted constructor values are `skip`, `replace`, and `fail`. There is no public policy enum.

- `skip` is the default. Existing non-empty pages are reused. A complete existing archive is reused after validation and identity checks.
- `replace` overwrites existing page outputs and replaces an existing archive after building and validating a new one. Pass `assume_yes=True`.
- `fail` stops when a destination page or archive already exists.

An existing archive with invalid structure or another chapter's identity is not silently reused. Use `replace` only when that is intentional.

## Progress reporting

The Python package does not expose a progress callback, iterator, event stream, or progress object. Python calls run the executable with `--json` and `--quiet`, so callers do not receive terminal progress events.

Use the CLI directly when a human-readable live progress display is needed:

~~~bash
mangate download <title-id> --chapter 1
~~~

The completed Python result contains page and chapter totals where the CLI has them. It is not a stream and cannot report intermediate progress.

## Cancellation

Downloads accept a `threading.Event` as `cancel_event`:

~~~python
from threading import Event
from mangate import Client, MangateError

cancel = Event()

try:
    result = client.download(
        "title-id",
        all_chapters=True,
        cancel_event=cancel,
    )
except MangateError as error:
    if error.category == "cancelled":
        print("cancelled")
~~~

Set the event from another thread. The client sends an interrupt signal to the executable and raises `MangateError` with category `cancelled`. Completed page files remain available for a later retry. The current call does not expose a partial result when cancellation is raised.

The Python timeout is separate from the CLI HTTP timeout. When `timeout` is set on the client, it is a wall-clock deadline for each child process. Expiration terminates the process and raises `MangateError` with category `timeout`.

## Archive operations

### Convert one directory

~~~text
Client.convert(
    chapter_directory: str | os.PathLike[str],
    *,
    output_format: str = "cbz",
    output: str | os.PathLike[str] | None = None,
    remove_source: bool = False,
    dry_run: bool = False,
    assume_yes: bool = False,
) -> dict[str, Any]
~~~

Converts a local chapter directory without contacting a provider. `output_format` must be `cbz` or `zip`. If `output` is omitted, the destination is the source path plus the selected extension.

~~~python
converted = client.convert(
    "./library/Example-123/Chapter-1",
    output_format="zip",
)
print(converted["outputPath"])
~~~

`remove_source=True` deletes the source directory only after archive validation and requires `assume_yes=True`. `dry_run=True` returns a plan without writing or deleting. The method raises `ValueError` for an unsupported archive format.

The result dictionary can contain:

- `format`.
- `outputPath`.
- `sourceDir`.
- `status`, normally `complete` or `skipped`.
- `includedPages`.
- `validation`.
- `sourceRemoved`.
- `warnings`.

### Convert many directories

~~~text
Client.convert_many(
    chapter_directories: Iterable[str | os.PathLike[str]],
    *,
    output_format: str = "cbz",
    outputs: Iterable[str | os.PathLike[str]] | None = None,
    remove_source: bool = False,
    dry_run: bool = False,
    assume_yes: bool = False,
) -> list[dict[str, Any]]
~~~

Converts directories sequentially in input order. If `outputs` is supplied, it must contain exactly one destination for every source directory. A mismatch raises `ValueError`.

~~~python
results = client.convert_many(
    ["./library/Title/Chapter-1", "./library/Title/Chapter-2"],
    output_format="cbz",
    outputs=["./archives/chapter-1.cbz", "./archives/chapter-2.cbz"],
)
~~~

### Inspect and verify

~~~text
Client.inspect_archive(
    archive_path: str | os.PathLike[str],
) -> dict[str, Any]

Client.verify_archive(
    archive_path: str | os.PathLike[str],
) -> dict[str, Any]
~~~

Both methods inspect a CBZ or ZIP without extracting it. They return:

- Validation fields `valid`, `complete`, `message`, `pageCount`, `format`, `titleId`, and `chapterId`.
- `state`, such as `structurally_invalid`, `metadata_incomplete`, `identity_unconfirmed`, `incomplete`, or `complete`.
- `path`, `entryCount`, `entries`, and optional `unexpectedEntries`.
- `metadataFound` and `identityConfirmed`.
- Optional stored `metadata`.

~~~python
inspection = client.inspect_archive("./archives/chapter-1.cbz")
verification = client.verify_archive("./archives/chapter-1.cbz")
if not verification["complete"]:
    print(verification.get("message"))
~~~

There is no public archive-repair method. Fix the source directory or recreate the archive when verification fails.

## Exceptions

The package exports one exception class:

~~~python
from mangate import MangateError
~~~

`MangateError` is a dataclass with these public fields:

| Field | Type | Meaning |
| --- | --- | --- |
| `category` | string | Stable programmatic category. |
| `message` | string | Human-readable error message. |
| `stderr` | string | Captured executable standard error, which may be empty. |

It inherits from `RuntimeError`. `str(error)` returns `error.message`. There are no public subclasses.

The package uses these categories:

| Category | Meaning | Retry guidance |
| --- | --- | --- |
| `unknown_provider` | The provider ID is not registered. | Change the provider ID. |
| `not_found` | The requested title or resource was not found. | Check the stable ID. |
| `unsupported_operation` | The provider or operation does not permit the request. | Inspect provider capabilities. |
| `timeout` | The Python deadline or an underlying request timeout expired. | Retry with a longer timeout if the provider is reachable. |
| `archive` | Archive creation or inspection failed. | Check the source images or archive path. |
| `filesystem` | A local file or directory could not be created, written, or accessed. | Check permissions and paths. |
| `cancelled` | The caller cancelled a download or the process was interrupted. | Retry the remaining selection. |
| `invalid_input` | A required value or chapter selection is invalid. | Check values and use stable chapter IDs. |
| `network` | The executable reported a network or request failure. | Check connectivity and provider availability. |
| `internal` | The executable failed without a narrower category. | Inspect `stderr` and retry only after checking the setup. |

The constructor and some local argument checks raise built-in `ValueError` instead. A missing executable may raise the operating system's process-start exception before Mangate can return structured JSON.

~~~python
try:
    client.download("title-id", chapter_numbers=["1"])
except MangateError as error:
    if error.category in {"timeout", "network"}:
        print("provider request failed")
    elif error.category == "filesystem":
        print("check the output directory")
    else:
        raise
~~~

## Data models and enums

There are no exported title, chapter, page, result, status, format, or policy classes. The public data model is composed of dictionaries and lists returned from the CLI JSON envelope.

The important dictionary shapes are:

- Title summaries from `search`.
- Title records from `title`.
- Chapter dictionaries from `chapters`.
- Provider records from `providers` and `provider_info`.
- Download records from `download`.
- Archive result dictionaries from `convert`.
- Archive inspection dictionaries from `inspect_archive` and `verify_archive`.

Use dictionary keys shown in the method sections. The package does not guarantee object attributes such as `result.status` or enum members such as `OutputFormat.CBZ`.

## Type hints

The package is implemented with Python type annotations. Public methods use:

- `str` for IDs, provider IDs, queries, format values, and string paths.
- `os.PathLike[str]` for accepted filesystem paths.
- `Iterable[str]` for repeated chapter IDs and numbers.
- `Event | None` for download cancellation.
- `dict[str, Any]` and `list[...]` for JSON-decoded results.

The package does not expose a separate stub package. Import the public symbols from `mangate` and let a type checker infer the annotated method signatures.

## Paths

Constructor and archive path arguments accept strings and path-like objects such as `pathlib.Path`. The client converts them to strings before starting the executable.

Returned output and archive paths are strings from the CLI JSON result. Relative input and output paths are interpreted by the child process from the current working directory. Mangate does not expand shell syntax in a Python string, so expand user directories with `Path.expanduser()` when needed.

~~~python
from pathlib import Path
from mangate import Client

client = Client(output_dir=Path("~/mangate-library").expanduser())
~~~

Filesystem failures are reported by the executable and normally become `MangateError` with category `filesystem`.

## Structured serialization

Search, title, chapter, download, provider, and archive methods return dictionaries and lists that can be passed directly to `json.dumps` when their values are JSON-compatible:

~~~python
import json
from mangate import Client

titles = Client().search("example title", limit=1)
print(json.dumps(titles, indent=2))
~~~

There is no `to_dict` or `to_json` method and no separate schema-versioned Python object. The CLI envelope's `formatVersion` is consumed by the client before the method returns. The returned Python value is the envelope's `data` portion, so top-level operation and status fields are not included in ordinary successful method results. Partial download data is still returned when the CLI reports a usable `partial` payload.

## Concurrency and thread safety

Each client method starts a separate child process and waits for it. The package documentation supports using one `Client` from multiple Python threads because the client stores no shared mutable operation state.

The package does not provide a scheduler or lock for concurrent downloads. Calls that target the same output directory or archive path can still contend through the CLI's existing-file policy. Use separate paths for independent work.

There is no documented guarantee for process forking, sharing a client after fork, or running two writes to the same chapter at once.

## Async usage

The package has no async API. All public methods are synchronous and blocking. Run them in a worker thread or process when an application needs a responsive event loop.

Cancellation through `cancel_event` is the supported way to stop a running download from another Python thread.

## Resource lifecycle

There is no public `close` method and `Client` is not a context manager. Each method owns and waits for its child process. No explicit cleanup is required after a successful call.

The CLI retains completed page files after a failed or cancelled download. Temporary incomplete page files use the `.part` suffix and are not treated as complete pages on a later run.

## Version compatibility

The package version is available as:

~~~python
from mangate import __version__

print(__version__)
~~~

The current package value is `0.1.0`. The executable version is available through:

~~~python
from mangate import Client

print(Client().version())
~~~

`Client.version()` returns the executable's version text. The package documentation describes compatibility with the Mangate 0.1.x CLI family. The repository does not promise semantic-version compatibility beyond the published API and current package metadata.

## Complete API reference

### `mangate`

- `mangate.Client`. Creates a synchronous client and exposes all provider, download, and archive operations.
- `mangate.MangateError`. Reports a structured operation failure.
- `mangate.__version__`. Reports the Python package version.

### `mangate.client.Client`

| Qualified name | Signature summary | Returns |
| --- | --- | --- |
| `Client.__init__` | Provider, executable, paths, timeout, concurrency, format, policy, and source-retention defaults. | None. |
| `Client.version` | `version()` | Executable version string. |
| `Client.providers` | `providers()` | Provider-record list. |
| `Client.provider_info` | `provider_info(provider=None)` | One provider record. |
| `Client.search` | `search(query, provider=None, limit=None)` | Title-summary list. |
| `Client.title` | `title(title_id, provider=None)` | Title record. |
| `Client.chapters` | `chapters(title_id, provider=None, limit=None, language=None)` | Chapter dictionary list; all languages by default. |
| `Client.download` | Chapter selectors, format, retention, dry-run, acknowledgement, and cancellation. | Download data dictionary. |
| `Client.convert` | Local directory, archive format, output, removal, dry-run, and acknowledgement. | Archive result dictionary. |
| `Client.convert_many` | Directory iterable, optional one-to-one output iterable, format, removal, dry-run, and acknowledgement. | Archive result list. |
| `Client.inspect_archive` | `inspect_archive(archive_path)` | Archive inspection dictionary. |
| `Client.verify_archive` | `verify_archive(archive_path)` | Archive inspection dictionary. |

All methods can raise `MangateError` for executable-reported failures. Constructor validation and batch destination-length validation use `ValueError`.

## Common Python workflows

### List providers

~~~python
from mangate import Client

for provider in Client().providers():
    print(provider["info"]["id"])
~~~

### Search and inspect a title

~~~python
from mangate import Client

client = Client()
titles = client.search("example title", limit=5)
if titles:
    record = client.title(titles[0]["id"])
    print(record["title"]["title"])
~~~

### Filter chapters and download several as CBZ

~~~python
chapters = client.chapters("title-id")
selected = [
    chapter["id"]
    for chapter in chapters
    if chapter.get("number") in {"1", "2"}
]
result = client.download(
    "title-id",
    chapter_ids=selected,
    output_format="cbz",
)
~~~

### Cancel a download

~~~python
from threading import Event

cancel = Event()
# Set cancel from another thread while this call is running.
client.download("title-id", all_chapters=True, cancel_event=cancel)
~~~

### Handle provider or filesystem errors

~~~python
from mangate import MangateError

try:
    client.download("title-id", chapter_numbers=["1"])
except MangateError as error:
    if error.category == "filesystem":
        print("Check the output directory")
    elif error.category in {"network", "timeout"}:
        print("Check provider access and retry")
    else:
        raise
~~~

### Convert a directory to ZIP

~~~python
converted = client.convert(
    "./library/Example-123/Chapter-1",
    output_format="zip",
)
print(converted["outputPath"])
~~~

### Verify an archive

~~~python
verification = client.verify_archive("./library/Example-123/Chapter-1.cbz")
if verification["complete"]:
    print("archive is complete")
else:
    print(verification.get("message", "archive is incomplete"))
~~~
