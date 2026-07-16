# Mangate

Mangate is a Go command-line downloader for manga and other sequential-image publications from supported providers. It is for material you are legally allowed to access and save. It does not bypass paywalls, DRM, CAPTCHAs, login controls, geographic restrictions, or provider limits.

Linux is the primary supported platform. Paths and filenames are sanitized on every platform supported by Go. The Python package supports Python 3.10 through 3.13 on Linux and drives the matching Mangate executable.

## Install

Build from this checkout with Go 1.26 or newer:

```sh
go build -o mangate ./cmd/mangate
./mangate --help
```

The default configuration is `~/.config/mangate/config.json`, or `MANGATE_CONFIG` when set. Command flags override configuration values. Provider defaults apply only when neither is supplied.

## CLI

```sh
# Discover what is installed without making a provider request.
mangate providers
mangate provider mangadex

# Find a title, then inspect it and its chapters.
mangate search "example title" --limit 10
mangate title <title-id>
mangate chapters <title-id>

# Pick exactly what to download. No interactive prompt is required.
mangate --download-dir ./library download <title-id> --chapter-id <chapter-id>
mangate download <title-id> --chapter 1.5
mangate download <title-id> --range 1-10
mangate download <title-id> --first
mangate download <title-id> --latest
mangate download <title-id> --all --chapter-language en

# Choose a per-chapter archive. CBZ is for comic readers, ZIP is a general archive.
mangate --format cbz download <title-id> --chapter 1
mangate --format zip download <title-id> --latest

# Plan first, without downloading or creating files.
mangate --format cbz download <title-id> --range 1-10 --dry-run

# Convert pages that were already downloaded. This never contacts a provider.
mangate --format cbz archive convert ./library/Example-123/Chapter-1
mangate archive inspect ./library/Example-123/Chapter-1.cbz
mangate archive verify ./library/Example-123/Chapter-1.cbz
```

`--chapter` rejects ambiguous releases. Use `--chapter-id` in that case. Chapters are listed in ascending provider chapter sequence. `--range`, `--before`, and `--after` currently compare provider chapter labels, so stable chapter IDs are the safe choice for special labels such as `Prologue`.

Run `mangate tui` to opt into the terminal UI. `interactive` remains an alias. With no arguments Mangate opens the TUI only when standard input and output are terminals. In a pipe or redirected shell it prints help instead.

## Files and repeated downloads

The default output root is `~/downloads/mangate`. Existing non-empty pages use the default `skip` behavior. Pass `--existing-files replace` to fetch them again, or `--existing-files fail` to stop on a conflict. Mangate writes to:

```text
<output root>/<sanitized title>-<title id>/<sanitized chapter>/0001.<original image extension>
```

The title ID keeps same-named titles apart. A page first lands in a `.part` file and is renamed only after a complete non-empty image response. Existing non-empty pages are reused. Each chapter directory includes `.mangate.json`, which records format version, remote IDs, expected page count, update time, and whether the chapter is complete. A failed page leaves completed pages intact and marks the chapter incomplete.

## Output formats and archives

`directory` is the default. It stores ordered image files in a chapter directory and writes `.mangate.json` after every chapter attempt. This state file records the remote identity, expected page count, and completion state.

`cbz` creates one standard ZIP-compatible comic archive per chapter, with a `.cbz` extension. Page images are at the archive root in download order, followed by `ComicInfo.xml` and `.mangate.json`. Mangate copies image bytes unchanged. `ComicInfo.xml` contains only known chapter data, and `.mangate.json` records the archive schema, provider and chapter identity, page count, and completion state.

`zip` creates the same one-archive-per-chapter layout with a `.zip` extension. It is a general archive and is not presented as comic-reader output.

Mangate writes every archive to a temporary sibling file, reopens and verifies it, then renames it to its final path. A failed or cancelled archive never appears at its requested final path. The default `--existing-files skip` policy reuses an existing archive only after it validates and its stored title and chapter identity match. `replace` builds and validates a new temporary archive before replacing the old one. `fail` stops on any existing destination.

Archive downloads retain the page directory by default. Use `--retain-source=false` on a download, or `--remove-source` with `archive convert`, to remove the source directory only after a valid archive is finalized. Archive entries never contain absolute paths or parent-directory traversal. Archive timestamps reflect creation time, so byte-for-byte reproducibility is not promised.

`archive convert` accepts a local chapter directory and creates a CBZ or ZIP without provider requests. It requires at least one recognized image page and rejects a present but incomplete `.mangate.json` state file. Directories with pages but no local state can still be converted, with limited metadata. `archive inspect` and `archive verify` read entries in place and report format, pages, metadata, safe paths, and completion state without extracting anything.

## JSON output and exit status

Every core command accepts `--json`. Standard output then contains exactly one JSON object with `formatVersion`, `operation`, `status`, and `data`. Error output uses the same envelope and carries a stable category such as `invalid_input`, `unknown_provider`, `not_found`, `unsupported_capability`, `timeout`, `filesystem`, or `cancelled`.

`0` means complete success. `2` means invalid command usage, `3` configuration failure, `4` provider or content failure, `5` partial download, `6` filesystem or archive failure, `7` cancellation, and `10` internal failure. A multi-chapter partial result uses `5` and its structured chapter result identifies the completed and incomplete chapters. A successful search with no results returns `0`.

Use `--quiet` to suppress progress. `--verbose` writes limited diagnostic context to standard error and never prints credentials. JSON output never mixes progress into standard output.

## Provider support

| ID | Provider | Search | Title | Chapters | Pages | Download |
| --- | --- | --- | --- | --- | --- | --- |
| `mangadex` | MangaDex public API v5 | yes | yes | selected language | yes | yes, subject to MangaDex access and terms |

MangaDex uses the configured language, defaulting to `en`. The service may alter its API, availability, rate limits, or content access independently of Mangate. Its public API adapter has no authentication setup. Provider capability and restriction data is available from `mangate provider mangadex`.

## Python

The package lives in [`python`](python). Install it with:

```sh
python -m pip install ./python
```

```python
from mangate import Client, MangateError

client = Client(executable="./mangate", output_dir="./library", timeout=60, existing_files="skip", output_format="cbz")
titles = client.search("example title", limit=5)
chapters = client.chapters(titles[0]["id"])

try:
    result = client.download(titles[0]["id"], chapter_ids=[chapters[0]["id"]])
    verified = client.verify_archive(result["chapters"][0]["archivePath"])
except MangateError as error:
    print(error.category, error)
```

`Client` methods block and return structured dictionaries. Each call runs independently and may be used from different Python threads. Pass a `threading.Event` as `cancel_event` to `download` to interrupt the process. The same durable-page guarantees apply.

`Client.download` accepts `output_format="directory"`, `"cbz"`, or `"zip"` and returns the requested format, output paths, and archive validation state. `Client.convert`, `inspect_archive`, and `verify_archive` expose local archive operations. CLI and Python version `0.1.x` are compatible. The Python API exports `Client`, `MangateError`, and `__version__`.
