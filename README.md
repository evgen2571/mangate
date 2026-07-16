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
mangate diagnostics

# Find a title, then inspect it and its chapters.
mangate search "example title" --limit 10
mangate search "example title" --interactive
mangate title <title-id>
mangate chapters <title-id>

# Pick exactly what to download. No interactive prompt is required.
mangate --download-dir ./library download <title-id> --chapter-id <chapter-id>
mangate download <title-id> --chapter 1.5
mangate download <title-id> --range 1-10
mangate download <title-id> --first
mangate download <title-id> --latest
mangate download <title-id> --all --chapter-language en

# Broad downloads, replacement, and source-page deletion require an explicit acknowledgement.
mangate --format cbz --existing-files replace download <title-id> --latest --yes

# Choose a per-chapter archive. CBZ is for comic readers, ZIP is a general archive.
mangate --format cbz download <title-id> --chapter 1
mangate --format zip download <title-id> --latest

# Plan first, without downloading or creating files.
mangate --format cbz download <title-id> --range 1-10 --dry-run

# Convert pages that were already downloaded. This never contacts a provider.
mangate --format cbz archive convert ./library/Example-123/Chapter-1
mangate --format cbz archive convert ./library/Example-123/Chapter-1 --remove-source --yes
mangate archive inspect ./library/Example-123/Chapter-1.cbz
mangate archive verify ./library/Example-123/Chapter-1.cbz
```

Human-readable search results include the provider, alternative title, content rating, publication status, original language, and year when the provider supplies them. Use `--language` to filter by original language and `--content-type` to filter content ratings when supplied by a provider. Repeat `--content-type` to match any supplied value; duplicates are ignored and their order does not matter. Add `--interactive` to open matching results in the TUI without repeating the search. Use the displayed `Reference` value with `mangate title`, `mangate chapters`, or `mangate download`; `--json` retains the complete structured metadata.

Use `--quiet` to suppress successful human-readable command output; it does not suppress errors or alter `--json` output. Use `--verbose` to add a safe error category and exit-code diagnostic on failure.

Exit status `1` means a valid search returned no results. In JSON mode, this is represented by a `no_results` status and an empty result list.

`--chapter` rejects ambiguous releases. Use `--chapter-id` in that case. Chapters are listed in ascending provider chapter sequence with their stable IDs and languages. `--range`, `--before`, and `--after` currently compare provider chapter labels, so stable chapter IDs are the safe choice for special labels such as `Prologue`. Archive downloads validate and reuse matching existing archives under the default `skip` policy without downloading page content again.

Run `mangate tui` to opt into the terminal UI. `interactive` remains an alias. With no arguments Mangate opens the TUI only when standard input and output are terminals. Mangate supports standard Linux terminal emulators, SSH sessions, and terminal multiplexers with basic or true-colour capability; it refuses pipes, redirected output, and `TERM=dumb` so interactive rendering cannot corrupt scripted output. Pass `--non-interactive` in scripts to refuse TUI entry explicitly. TUI colors follow terminal defaults; use `--no-color` to disable them or `--color` to force them. Those two flags conflict.

The TUI is a sequential keyboard flow: search, choose a title, select one or more chapters, choose Directory, CBZ, or ZIP, choose or edit the output root, then review the operation before it starts. The chapter selector labels local state as complete pages, incomplete pages, archive present, or not downloaded; `/` can filter metadata and local state with terms such as `local:complete` or `local:archive`. The review shows known page totals, every planned output path, existing-output policy decisions, and whether source pages are retained after archive validation. A final result screen separates completed, reused, incomplete, and archive-failed chapters with their actual output paths; a partial page failure still finalizes any sibling chapters that completed safely. Use arrows or `j` and `k` to move, `space` to toggle chapters, `a` to select all visible chapters, `l` to select the latest visible chapter, `r` to select the filtered range from the previous range anchor to the current row, `d` to clear them, `enter` to continue, and `esc` to go back. `ctrl+g` opens configuration, including output root, default format, existing-file policy, and source retention; applying it while reviewing immediately refreshes the plan. It has no mouse requirement. `ctrl+c` or `q` exits before a download begins; while a download is active it cancels the operation and keeps completed page data intact.

## Files and repeated downloads

The default output root is `~/downloads/mangate`. Existing non-empty pages use the default `skip` behavior. Pass `--existing-files replace` to fetch them again, or `--existing-files fail` to stop on a conflict. Mangate writes to:

```text
<output root>/<sanitized title>-<title id>/<sanitized chapter>/0001.<original image extension>
```

The title ID keeps same-named titles apart. A page first lands in a `.part` file and is renamed only after a complete non-empty image response. Existing non-empty pages are reused. Each chapter directory includes `.mangate.json`, which records format version, remote IDs, expected page count, update time, and whether the chapter is complete. A failed page leaves completed pages intact and marks the chapter incomplete.

## Output formats and archives

`directory` is the default. It stores ordered image files in a chapter directory and writes `.mangate.json` after every chapter attempt. This state file records provider, remote identity, expected page count, and completion state.

`cbz` creates one standard ZIP-compatible comic archive per chapter, with a `.cbz` extension. Page images are at the archive root in download order, followed by `ComicInfo.xml` and `.mangate.json`. Mangate copies image bytes unchanged. When a local page has an `.img` fallback name, Mangate detects its image bytes and uses the real extension inside the archive. `ComicInfo.xml` contains known series, chapter, volume, language, page-count, and provider data; `.mangate.json` records the archive schema, provider and chapter identity, page count, and completion state.

`zip` creates the same one-archive-per-chapter layout with a `.zip` extension. It is a general archive and is not presented as comic-reader output.

Mangate writes every archive to a temporary sibling file, reopens and verifies it, then renames it to its final path. A failed or cancelled archive never appears at its requested final path. The default `--existing-files skip` policy reuses an existing archive only after it validates and its stored title and chapter identity match. `replace` builds and validates a new temporary archive before replacing the old one. `fail` stops on any existing destination.

Archive downloads retain the page directory by default. Use `--retain-source=false` on a download, or `--remove-source` with `archive convert`, to remove the source directory only after a valid archive is finalized. Archive entries never contain absolute paths or parent-directory traversal. Archive timestamps reflect creation time, so byte-for-byte reproducibility is not promised. Every human-readable download completion (including archive reuse and partial failure) reports completed, skipped/reused, failed, and archive-failure counts plus the resulting output paths. Downloads selecting 25 or more chapters, replacing outputs, or deleting archive source pages require `--yes`; use `--dry-run` first to inspect the resolved plan.

`archive convert` accepts a local chapter directory and creates a CBZ or ZIP without provider requests. It requires at least one recognized image page and rejects a present but incomplete `.mangate.json` state file. Directories with pages but no local state can still be converted, with limited metadata; the result carries an explicit warning when title or chapter identity cannot be confirmed. `archive inspect` and `archive verify` read entries in place and report format, pages, metadata, safe paths, unexpected non-page entries, completion state, and an explicit `identityConfirmed` value without extracting anything. Their structured `state` is one of `structurally_invalid`, `metadata_incomplete`, `identity_unconfirmed`, `incomplete`, or `complete`.

Use `mangate --format cbz archive convert <chapter-directory> --dry-run` to inspect a local conversion target without creating an archive or deleting source pages. The plan reports the target path, whether it already exists, and whether source cleanup was requested.

Run `mangate config` to inspect the effective provider, output root, format, existing-file policy, and source-retention setting after configuration and command flags are merged.

## Shell completion

Mangate includes generated completion commands for Bash, Zsh, and Fish. Load one in the current shell with the corresponding command:

```sh
source <(mangate completion bash)
source <(mangate completion zsh)
mangate completion fish | source
```

Completion is local. It does not search providers or download content.

## Testing

```sh
go test ./...
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=python/src python -m unittest discover -s python/tests -v
```

## JSON output and exit status

Every core command accepts `--json`. Standard output then contains exactly one JSON object with `formatVersion`, `operation`, `status`, and `data`. Error output uses the same envelope and carries a stable category such as `invalid_input`, `unknown_provider`, `not_found`, `unsupported_capability`, `timeout`, `filesystem`, or `cancelled`.

`0` means complete success. `1` means a valid search found no results, `2` means invalid command usage, `3` configuration failure, `4` provider or content failure, `5` partial download, `6` filesystem failure, `7` cancellation, `8` archive creation or validation failure, and `10` internal failure. A multi-chapter partial result uses `5` and its structured chapter result identifies the completed and incomplete chapters. In JSON mode an empty search uses the `no_results` status with an empty result list.

Use `--quiet` to suppress successful human-readable output. `--verbose` writes limited diagnostic context to standard error and never prints credentials. JSON output never mixes progress into standard output.

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

`Client.download` accepts `output_format="directory"`, `"cbz"`, or `"zip"`, plus `dry_run=True` for a no-write download plan and `assume_yes=True` for broad or destructive operations that require explicit acknowledgement. It returns the requested format, output paths, and archive validation state. A partial download returns its completed and failed chapter records instead of discarding completed archive paths. `Client.convert` accepts `dry_run=True` for local conversion planning and `assume_yes=True` for source deletion or replacement, and `Client.convert_many` converts directories sequentially in input order (with optional one-to-one output paths). `inspect_archive` and `verify_archive` expose archive inspection. CLI and Python version `0.1.x` are compatible. The Python API exports `Client`, `MangateError`, and `__version__`.
