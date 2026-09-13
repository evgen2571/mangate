# Python API

The Python package runs the Go CLI as a subprocess. Install both components; the wrapper does not download an executable.

## Install from the checkout

With Go 1.26+, Python 3.10+, and uv available:

```sh
go install ./cmd/mangate
uv venv
uv pip install ./python
```

Run Python in that environment with `uv run python`, or activate the environment using the command printed by `uv venv`. Add Go's binary directory to `PATH`, or pass an absolute executable path to `Client`.

## Search, inspect, and download

```python
from mangate import Client, MangateError

client = Client(executable="mangate", output_dir="./library", timeout=120)

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
| `executable` | `"mangate"` | Command name or filesystem path |
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

`download(title_id, ...)` supports `chapter_ids`, `chapter_numbers`, `chapter_range`, `first`, `latest`, `all_chapters`, and `language`. It also accepts `provider`, `output_format`, `dry_run`, `assume_yes`, and `cancel_event`.

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

`MangateError` is a `RuntimeError` with `category`, `message`, and `stderr`. JSON command failures, malformed JSON output, and wrapper cancellation/timeouts are reported through this exception. `version()` uses the text subprocess path, whose timeout can raise `subprocess.TimeoutExpired`. Missing executables and process creation failures can raise OS-level exceptions such as `FileNotFoundError`.

Partial download results are returned even though the CLI exits nonzero. Inspect `result["status"]` and each chapter's `status`, `outputPath`, and optional `archivePath`. A fully unsuccessful operation raises `MangateError`. Unsupported output formats and mismatched batch destinations raise `ValueError` before starting a subprocess.

## Dataset operations

The wrapper also exposes `dataset_plan`, `dataset_collect`, `dataset_status`, `dataset_verify`, and `dataset_export`. Plan and collect accept `collection_config`, `resume`, and additional CLI arguments through `extra_args`. Collect supports confirmation, dry-run, and cancellation. Verification can request repair; export can filter a split and include duplicates or rejected records.

Dataset collection has its own immutable plan and configuration. Follow the [dataset guide](dataset.md) before using these methods.
