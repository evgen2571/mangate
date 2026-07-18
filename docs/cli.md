# CLI documentation

Mangate has two ways to work. Direct commands are suited to scripts, explicit searches, and downloads. The full-screen TUI is suited to browsing titles and selecting chapters interactively. Both modes use the same provider, download directory, output format, and existing-file settings.

Direct commands print human-readable results by default. Add `--json` when another program needs structured output. Downloads support `directory`, `cbz`, and `zip` output.

## Command notation

The examples in this guide use these conventions:

- `<value>` is a required value that you replace.
- `[value]` is optional.
- A flag described as repeatable may be supplied more than once.
- A title reference is the stable provider ID shown as `Reference` by `mangate search`. It is not the title's display name.
- A chapter ID is the stable provider ID shown by `mangate chapters`. Use it when a chapter number identifies more than one release.
- A provider identifier is the short ID shown by `mangate providers`, such as `mangadex`.

Mangate passes title and chapter references to the configured provider. A title reference from one provider should not be assumed to work with another provider.

## Built-in help

The installed executable can describe the available commands and flags:

~~~bash
mangate --help
mangate <command> --help
~~~

You can also use the built-in `help` command:

~~~bash
mangate help download
mangate help archive convert
~~~

Built-in help is the authoritative concise summary for the installed version. Use it when a newer executable may have different flags from this guide.

## CLI modes

### Direct CLI mode

Direct mode is the right choice for:

- Explicit searches, title lookups, chapter lists, and downloads.
- Shell scripts and scheduled jobs.
- JSON output.
- Archive conversion and verification.

Direct commands do not open a selection prompt. Download operations that are broad or destructive require an explicit `--yes` flag.

### Interactive TUI mode

Launch the full-screen terminal interface with:

~~~bash
mangate tui
~~~

`mangate interactive` is an alias for the same command.

With no arguments, Mangate opens the TUI only when standard input and standard output are interactive terminals, `TERM` is not `dumb`, and `--json` and `--non-interactive` are not in use. Otherwise, it prints the root help. Pipes and redirected output therefore stay in direct mode.

The TUI searches for a title, loads its chapters, lets you select several chapters, asks for a format and output directory, shows a review screen, and then downloads. See [TUI reference](#tui-reference) for the keyboard controls.

## Global options

These options are available on the root command and inherited by its subcommands. A command can still reject an option that does not make sense for its operation. For example, archive conversion requires an archive format.

| Option | Accepted value and default | Effect |
| --- | --- | --- |
| `--provider string` | Provider ID. Default: `mangadex`. | Selects the provider for searches, title lookups, chapter lists, and downloads. |
| `--language string` | Provider language code. Default: `en`. | Sets the configured provider language. When explicitly supplied to `search`, it also filters results by the title's original language. |
| `--download-dir string` | Directory path. Default: `~/downloads/mangate` on a normal Linux installation. | Sets the root for downloaded titles and chapters. |
| `--output string` | Same value as `--download-dir`. | Alias for the download root. |
| `--format string` | `directory`, `cbz`, or `zip`. Default: `directory`. | Chooses the download format. Archive conversion requires `cbz` or `zip`. |
| `--existing-files string` | `skip`, `replace`, or `fail`. Default: `skip`. | Controls conflicts with existing pages and archives. |
| `--retain-source` | Legacy Boolean setting. | Archive downloads always remove temporary page directories after validation. Standalone archive conversion still uses its explicit `--remove-source` option. |
| `--page-downloads int` | Positive integer. Default: `8`. | Limits simultaneous page downloads. |
| `--chapter-downloads int` | Positive integer. Default: `6`. | Limits simultaneous chapter downloads. |
| `--search-history-max int` | Non-negative integer. Default: `100`. | Sets the number of search queries remembered by the TUI. `0` disables search history. |
| `--cache-dir string` | Directory path. Default: the user cache directory plus `mangate`, normally `~/.cache/mangate` on Linux. | Stores local cache data such as search history and covers. |
| `--json` | Boolean. Default: false. | Writes one JSON document to standard output for supported data commands. It suppresses download progress. |
| `--quiet` | Boolean. Default: false. | Suppresses successful human-readable output. It does not suppress errors or JSON output. |
| `--verbose` | Boolean. Default: false. | Adds a safe error category and exit-code diagnostic to standard error when a command fails. |
| `--non-interactive` | Boolean. Default: false. | Refuses the TUI and interactive terminal entry. It is useful as a guard in scripts. |
| `--color` | Boolean. Default: false. | Forces true-colour output in the TUI. |
| `--no-color` | Boolean. Default: false. | Uses an ASCII colour profile in the TUI. |
| `--version`, `-v` | No value. | Prints the executable version and target operating system. |

`--color` and `--no-color` cannot be combined. The colour flags affect the TUI. Normal direct output does not add colour.

There is no timeout flag. The HTTP timeout is configured with the `http.timeout` JSON setting described in [Configuration](#configuration).

The standard `--help` and `-h` flags are available for the root command and each command.

## Command overview

| Command | Purpose |
| --- | --- |
| `help` | Show help for a command. |
| `providers` | List registered providers and their capabilities. |
| `provider` | Inspect one provider. |
| `search` | Search titles. |
| `title` | Show title metadata. |
| `chapters` | List chapters for a title. |
| `download` | Download selected chapters. |
| `archive` | Convert, inspect, or verify local archives. |
| `config` | Show effective configuration. |
| `diagnostics` | Check local setup without provider requests. |
| `completion` | Generate Bash, Zsh, or Fish completion. |
| `tui` | Open the full-screen terminal interface. |

## Command reference

### `help`

Show help for the root command or a command path.

~~~text
mangate help [command] [flags]
~~~

The optional command may be a top-level command or a nested path such as `archive convert`.

~~~bash
mangate help
mangate help search
mangate help archive convert
~~~

### `providers`

List all registered providers. The command does not make title or chapter requests.

~~~text
mangate providers
~~~

It takes no positional arguments and has no command-specific options. Human-readable output contains the provider ID, name, availability, and capabilities. JSON output returns a list of provider records.

~~~bash
mangate providers
mangate --json providers
~~~

### `provider`

Inspect one provider's public metadata and declared capabilities.

~~~text
mangate provider <provider-id>
~~~

The required provider ID is the ID shown by `mangate providers`. The command takes no command-specific options.

~~~bash
mangate provider mangadex
mangate --json provider mangadex
~~~

The output includes the provider name, ID, availability, capabilities, authentication description, whether downloads are permitted, restrictions, and description. An unknown ID fails with an `unknown_provider` error category and exit status 4.

### `search`

Search the configured provider by title.

~~~text
mangate search <title> [flags]
~~~

The title is required. Multiple positional words are joined with spaces, so quoting the complete query is usually clearest.

| Option | Default | Effect |
| --- | --- | --- |
| `--limit int` | No limit when zero. | Keeps at most this many results after provider and content filters. |
| `--content-type strings` | No filter. | Repeatable. Keeps results whose content type matches any supplied value. Duplicate values are ignored. |
| `--interactive` | false | Opens the TUI on the returned results. It cannot be combined with `--json`. |

The global `--provider` and `--language` options apply. A language filter is applied to search results when `--language` is explicitly supplied. The provider also uses its configured language when mapping title metadata.

~~~bash
mangate search "example title"
mangate --provider mangadex --language en search "example title" --limit 10
mangate search "example title" --content-type safe --content-type suggestive
mangate search "example title" --interactive
mangate --json search "example title"
~~~

Human-readable results show the title, provider, optional alternative title, content type, status, original language, year, stable reference, and provider URL. Missing fields are omitted.

An empty query is invalid. A valid search with no results prints a no-results message and exits with status 1. With `--json` it returns a `no_results` envelope with an empty `results` list and still exits with status 1.

The stable `Reference` returned by this command is the value to pass to `title`, `chapters`, and `download`:

~~~bash
mangate search "example title"
mangate title <reference-from-search>
mangate chapters <reference-from-search>
~~~

### `title`

Show metadata for one title.

~~~text
mangate title <title-id>
~~~

The required title ID must be a stable reference returned by `search` for the selected provider. The command has no command-specific options.

~~~bash
mangate title <title-id>
mangate --json title <title-id>
~~~

Human-readable output may include the title, ID, provider, URL, status, alternative title, content type, language, year, and a description. Missing metadata is omitted. JSON output returns a title record with the provider and a title object.

For MangaDex, an unknown ID is reported as not found. Other provider failures are provider-dependent.

### `chapters`

List the chapters available for one title.

~~~text
mangate chapters <manga-id> [flags]
~~~

The required positional value is the title reference from `search`. The help text calls it `manga-id`, but it is the same stable title ID used by `title` and `download`.

| Option | Default | Effect |
| --- | --- | --- |
| `--limit int` | No limit when zero. | Keeps at most this many chapters after provider ordering. |
| `--chapter-language string` | All languages. | Keeps only chapters with this exact provider language. |

~~~bash
mangate chapters <title-id>
mangate chapters <title-id> --chapter-language en --limit 20
mangate --json chapters <title-id>
~~~

The CLI reports all available chapters in the provider's ascending chapter sequence by default. The MangaDex provider sorts numeric chapter labels numerically, puts non-numeric labels after numeric ones, and uses release metadata and the stable chapter ID to order ties. Use `--chapter-language` to limit the list to one language.

Human-readable rows can include the display title, stable chapter ID, page count, volume, language, release group, publication time, and URL. A chapter with no number is represented by the provider's value. For MangaDex, an unnumbered chapter is mapped to number `0`.

No chapters is a successful result. Human output says no chapters were found. JSON output returns a successful `chapters.list` envelope with an empty list.

### `download`

Download selected chapters for one title.

~~~text
mangate download <title-id> [flags]
~~~

The title ID is required and must be a stable reference from `search`. At least one chapter-selection option is required.

#### Chapter-selection options

| Option | Accepted value | Repeatable | Effect |
| --- | --- | --- | --- |
| `--chapter-id string` | Stable chapter ID. | Yes. | Selects the exact release. |
| `--chapter string` | Chapter number or label. | Yes. | Selects one chapter when the number is unique. |
| `--range string` | Inclusive `START-END`. | No. | Selects chapters whose labels fall within the range. Numeric labels use numeric comparison. |
| `--before string` | Chapter number or label. | No. | Selects chapters with labels less than the value. |
| `--after string` | Chapter number or label. | No. | Selects chapters with labels greater than the value. |
| `--first` | No value. | No. | Selects the first chapter in the provider's ordered list. |
| `--latest` | No value. | No. | Selects the last chapter in the provider's ordered list. This is a direct CLI option, not a TUI action. |
| `--all` | No value. | No. | Selects all accessible chapters after the language filter. |
| `--chapter-language string` | Provider language code. | No. | Keeps only chapters with this exact language value before selection. |

You must choose one of the selection methods. `--first`, `--latest`, and `--all` are mutually exclusive and cannot be combined with explicit chapters or ranges. Explicit chapter selectors cannot be combined with `--range`, `--before`, or `--after`. `--range`, `--before`, and `--after` form the range-selection group.

Chapter numbers are matched as provider strings. Numeric range comparison handles integers and decimals such as `1` and `1.5`. Special or non-numeric labels use string comparison for range, before, and after operations.

An exact chapter number must match one accessible release. If several releases have the same number, the command fails with an ambiguity error and tells you to use `--chapter-id`. Repeating the same chapter ID or selecting it through more than one compatible selector downloads it once.

Malformed ranges, unknown chapter IDs, unknown chapter numbers, empty selections, and conflicting selectors are invalid input. They do not silently broaden the selection.

#### Download-specific options

| Option | Default | Effect |
| --- | --- | --- |
| `--dry-run` | false | Resolves the title, chapter selection, output paths, and format without downloading or creating files. |
| `--yes` | false | Acknowledges a broad or destructive operation. See [Confirmation and dry runs](#confirmation-and-dry-runs). |

The global `--provider`, `--download-dir`, `--format`, `--existing-files`, `--retain-source`, `--page-downloads`, `--chapter-downloads`, `--language`, `--json`, `--quiet`, and `--verbose` options also apply.

#### Examples

~~~bash
mangate download <title-id> --chapter 1
mangate download <title-id> --chapter 1 --chapter 2.5
mangate download <title-id> --range 1-10
mangate --format cbz download <title-id> --chapter-id <chapter-id>
mangate --format zip --output ./library download <title-id> --all --yes
mangate --non-interactive --json download <title-id> --chapter 1
~~~

The direct command does not ask an interactive question. For a broad or destructive operation, first inspect the plan and then acknowledge it:

~~~bash
mangate --format cbz download <title-id> --all --dry-run
mangate --format cbz download <title-id> --all --yes
~~~

The command requires `--yes` when it selects 25 or more chapters, uses `--existing-files replace`, or creates CBZ/ZIP archives, because archive downloads remove their temporary page directories after validation.

The default output root is the configured download directory. Mangate creates a title directory using a sanitized title and title ID, then a sanitized chapter directory. Directory output points to the chapter directory. CBZ and ZIP output points to the archive path beside that directory.

While downloading, human mode writes a preflight plan and page/chapter progress to standard error. The final summary goes to standard output. `--quiet` suppresses the successful preflight, progress, and final summary. `--json` suppresses progress and writes the final structured result to standard output.

Mangate writes completed pages directly and keeps incomplete page transfers in temporary `.part` files. A failed chapter keeps completed pages and records an incomplete chapter state. Re-running the same selection retries missing work and reuses completed non-empty pages according to the existing-file policy.

Page requests retry rate-limit responses. A provider page lookup may also be refreshed once after a forbidden page response. These retries do not make a provider available when the provider itself is down.

Press Ctrl-C, or send an interrupt or termination signal, to cancel a direct download. Completed page files remain. A cancellation normally exits with status 7.

#### Download result states

Human output ends with counts for completed, skipped or reused, failed or incomplete, archive failures, expected pages, and reused pages. It also lists each resulting output path.

JSON download data contains:

- `provider`, `title`, `format`, and `outputRoot`.
- `startedAt` and `completedAt` timestamps.
- An overall `status`.
- A `chapters` list.
- An optional `error` message.

Each chapter record contains its stable `id`, optional `number` and `title`, a `status`, `outputPath`, an optional `archivePath`, optional expected page count, and optional archive validation data.

The overall JSON status is `success` for a complete download, `partial` when at least one chapter completed but another failed, and `incomplete` when the operation failed without a completed chapter. A dry run returns data with `status` set to `planned` and uses operation `download.plan`.

### `archive`

Archive commands work with local chapter directories and archives. They do not need a provider request.

~~~text
mangate archive [command]
~~~

Available subcommands are `convert`, `inspect`, and `verify`.

#### `archive convert`

Create a CBZ or ZIP archive from a local chapter directory.

~~~text
mangate archive convert <chapter-directory> [flags]
~~~

The required path must name a directory containing ordered image files. The format is selected with the global `--format` option and must be `cbz` or `zip`.

| Option | Default | Effect |
| --- | --- | --- |
| `--output string` | Source path plus `.cbz` or `.zip`. | Sets the destination archive path. The extension must match the selected format. |
| `--remove-source` | false | Removes the source directory only after the archive validates. |
| `--dry-run` | false | Shows the source, format, destination, destination-exists flag, and removal choice without changing files. |
| `--yes` | false | Acknowledges source deletion or replacement when required. |

~~~bash
mangate --format cbz archive convert ./library/Example-123/Chapter-1
mangate --format zip archive convert ./library/Example-123/Chapter-1 --output ./archives/chapter-1.zip
mangate --format cbz archive convert ./library/Example-123/Chapter-1 --remove-source --yes
mangate --format cbz archive convert ./library/Example-123/Chapter-1 --dry-run
~~~

Conversion is offline. It reads image files from the source directory. Recognized page extensions are `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.avif`, `.bmp`, and `.img`. Files ending in `.part` are ignored. Page names must contain positive numeric order, such as `0001.jpg`. Pages are ordered by that number and duplicate positions fail.

If the source contains `.mangate.json`, it must describe a complete chapter with the expected page count. A directory without local state can still be converted, but the result warns that archive identity cannot be confirmed. CBZ includes page files, `ComicInfo.xml`, and `.mangate.json`. ZIP includes page files and `.mangate.json`.

The default existing-file policy is `skip`. A complete existing archive is reused only when it validates and its stored identity matches the requested title and chapter when those identities are available. `replace` builds and validates a new archive before replacing the old one. `fail` rejects an existing destination. Replacing an archive or removing source pages requires `--yes`.

The converter writes and validates a temporary archive before giving it the final name. A failed conversion does not leave a new final archive. If source removal fails after archive creation, the archive remains and the command reports the cleanup error.

#### `archive inspect`

Inspect a local CBZ or ZIP without extracting it.

~~~text
mangate archive inspect <archive-path>
~~~

~~~bash
mangate archive inspect ./library/Example-123/Chapter-1.cbz
mangate --json archive inspect ./library/Example-123/Chapter-1.cbz
~~~

The result reports the format, page count, entry count, archive state, completion, identity confirmation, unexpected entries, and stored metadata when present.

#### `archive verify`

Check the same archive structure and completion metadata using a verification-oriented command name.

~~~text
mangate archive verify <archive-path>
~~~

~~~bash
mangate archive verify ./library/Example-123/Chapter-1.cbz
mangate --json archive verify ./library/Example-123/Chapter-1.cbz
~~~

Both archive commands reject unsafe entry paths, duplicate names, unordered pages, invalid image data, missing pages, and invalid metadata. The reported state can be `structurally_invalid`, `metadata_incomplete`, `identity_unconfirmed`, `incomplete`, or `complete`.

### `config`

Show the effective configuration after defaults, the configuration file, and CLI flags have been applied.

~~~text
mangate config
~~~

The command takes no positional arguments or command-specific options. Human output shows the config path, provider, language, output directory, format, existing-file policy, and source-retention setting. JSON output includes the path and the complete effective configuration.

~~~bash
mangate config
mangate --format cbz --output ./library config --json
~~~

`configuration` is an alias for `config`.

### `diagnostics`

Check the local setup without contacting a provider or creating files.

~~~text
mangate diagnostics
~~~

It reports the platform, whether the current input and output are interactive terminals, the configured provider and its capabilities, the download and cache paths, and the supported formats.

~~~bash
mangate diagnostics
mangate --json diagnostics
~~~

`doctor` is an alias for `diagnostics`.

### `completion`

Generate a shell-completion script. Completion generation is local and does not search or download.

~~~text
mangate completion <bash|zsh|fish>
~~~

The required shell is one of `bash`, `zsh`, or `fish`.

~~~bash
source <(mangate completion bash)
source <(mangate completion zsh)
mangate completion fish | source
~~~

The command writes the completion script to standard output. Redirect it to a file for persistent installation according to your shell's normal startup-file conventions.

### `tui`

Open the interactive full-screen interface.

~~~text
mangate tui
~~~

The command requires an interactive terminal and cannot run with `--non-interactive`. The `interactive` alias is equivalent.

## Providers

Mangate currently bundles one usable provider, MangaDex, with provider ID `mangadex`. It declares search, title, chapters, pages, and download capabilities, and reports downloads as permitted subject to authorization and provider terms.

The configured provider is `mangadex` by default. Set `--provider` or the JSON `provider` setting to select a provider ID. Use `providers` to list registered providers and `provider <provider-id>` to inspect one.

The provider reports authentication as optional. Mangate's current MangaDex integration uses its public endpoints and has no credential or login option. Provider availability, access restrictions, rate limits, terms, and capabilities may change independently of Mangate.

There is no multi-provider search command. Each search, title lookup, chapter list, and download uses one configured provider.

## Search, titles, and chapters

### Search results and metadata

Search results expose a stable ID, display title, provider URL when available, and provider metadata. MangaDex may provide an alternative title, status, content rating, original language, year, and localized descriptions.

Search does not search by a local library. An empty result is a valid provider response with exit status 1. A provider or network problem is an error, not an empty result.

### Title information

Use the stable ID returned from search. The command does not accept a display title as a lookup key. Missing optional metadata is omitted from human output and from JSON objects with optional fields.

### Chapter listing

Chapter records can contain:

- `id`, the stable release ID.
- `number`, the provider chapter label.
- `title`, an optional chapter title.
- `volume`, an optional volume label.
- `language`.
- `releaseGroup`.
- `publishedAt`.
- `pageCount`.
- `url`.

The list order is provider order. MangaDex returns all available languages and sorts numeric chapter labels numerically. Duplicate numbers, including releases in different languages, are separate releases and retain separate stable IDs.

The download command uses these IDs and labels for selection. It does not guess which duplicate release you intended.

## Chapter selection

The direct CLI supports exact chapter numbers, stable IDs, multiple explicit selections, inclusive ranges, before and after filters, first, latest, all, and a chapter-language filter.

~~~bash
mangate download <title-id> --chapter 1
mangate download <title-id> --chapter 1.5
mangate download <title-id> --chapter 1 --chapter 2
mangate download <title-id> --chapter-id <chapter-id>
mangate download <title-id> --range 1-10
mangate download <title-id> --before 10
mangate download <title-id> --after 1
mangate download <title-id> --latest
~~~

An exact number is safe only when the provider returns one matching release. If it returns two, use the ID from the chapter list:

~~~bash
mangate chapters <title-id>
mangate download <title-id> --chapter-id <one-release-id>
~~~

An unnumbered MangaDex release is represented as number `0`. Other providers may return non-numeric labels. Exact selection compares the provider's label as returned. Range, before, and after compare numeric labels numerically when both values parse as numbers and otherwise compare their strings.

The TUI does not have a latest-chapter action. Use direct `download --latest` or select a chapter in the TUI.

## Output formats

The global `--format` option selects the download format:

### `directory`

The default format stores each chapter as ordered image files in a chapter directory. A chapter state file named `.mangate.json` records provider and chapter identity, expected page count, and completion.

### `cbz`

CBZ produces one comic-book archive per chapter with a `.cbz` extension. Pages are stored in order. The archive also contains `ComicInfo.xml` and Mangate metadata.

### `zip`

ZIP produces one ordinary ZIP archive per chapter with a `.zip` extension. It uses the same ordered page layout and Mangate metadata, but does not add comic-book-specific `ComicInfo.xml`.

Choose formats before the command or in configuration:

~~~bash
mangate --format cbz download <title-id> --chapter 1
mangate --format zip download <title-id> --chapter 1
mangate --format cbz archive convert ./library/Example-123/Chapter-1
~~~

## Existing-file behaviour

The default policy is `skip`.

| Policy | Directory downloads | CBZ or ZIP downloads and conversion |
| --- | --- | --- |
| `skip` | Reuses each existing non-empty page. Missing pages are downloaded. | Reuses a complete archive after validation and identity matching. An invalid or mismatched existing archive is an error. |
| `replace` | Removes matching existing page files before downloading replacements. | Builds and validates a replacement archive before replacing the destination. |
| `fail` | Fails when a page destination already exists. | Fails when the destination archive already exists. |

Existing non-empty pages can be reused even when a chapter is incomplete. A rerun therefore keeps completed page data and retries missing or failed pages.

For an existing archive, `skip` checks archive structure, completion metadata, and stored title and chapter identity when available. It does not reuse an archive belonging to another chapter.

Use `--existing-files replace` only when overwriting is intended. Replacement requires `--yes` for downloads and archive conversion.

## Confirmation and dry runs

Mangate does not prompt for broad or destructive direct commands. It stops with an instruction to review a plan and rerun with `--yes`.

For downloads, `--yes` is required for:

- Selecting at least 25 chapters.
- `--existing-files replace`.
- Removing temporary source page directories after CBZ or ZIP creation.

For archive conversion, `--yes` is required for source removal and replacement.

Use `--dry-run` before a broad, replacement, or destructive operation. The dry run resolves the title, chapters, paths, and format without writing files or contacting a provider for archive conversion.

## TUI reference

The TUI workflow is:

1. Enter a title search.
2. Choose a title from the results.
3. Select one or more chapters.
4. Choose `directory`, `cbz`, or `zip`.
5. Enter or accept the output directory.
6. Review the title, provider, selection, format, output, and existing-file policy.
7. Confirm to download.

Press `?` to open the current screen's help. Press `Esc` to close it.

| Key | Use |
| --- | --- |
| Up / `k` | Move up. |
| Down / `j` | Move down. |
| Enter | Confirm, continue, or start the reviewed download. |
| `Esc` | Go back. On the result screen, `Backspace` also goes back. |
| `q` | Quit. During a download, request cancellation. |
| Ctrl-C | Quit or request cancellation during a download. |
| `?` | Open or close screen help. |
| `space` | Toggle the highlighted chapter. |
| `a` | Select all visible chapters. |
| `d` | Clear the chapter selection. |
| `r` | Extend the selection from the range anchor to the highlighted visible chapter. |
| `/` | Enter a chapter filter. |
| `f` | From search results, select all chapters for the selected title and continue. |
| Ctrl-G | Open settings outside focused text fields. |

The chapter filter matches chapter display text, language, and stable chapter ID. Selection shortcuts act on visible chapters. A review is required before the download starts.

The format screen cycles through the three formats. The output screen rejects an empty path and an existing regular file. The settings screen lets you edit provider, language, output, format, existing-file policy, source retention, HTTP timeout, page and chapter concurrency, search-history size, and cache directory. In settings, `a` applies the draft for the current session, `s` applies and saves it to the configuration file, and `Esc` returns without applying a new edit.

The download screen shows page and chapter progress. The completion screen reports complete, reused, and failed counts and keeps completed output paths visible. `Enter` or `Esc` returns to chapter selection. Completed files remain after cancellation.

## Human-readable output

Normal results go to standard output. Errors go to standard error. Download preflight and progress also use standard error so a final result can be redirected or piped without progress characters.

`--quiet` suppresses successful human-readable output. It does not suppress errors. It does not change JSON output.

`--verbose` adds a safe diagnostic line containing an error category and exit code. It does not print credentials, provider response bodies, or request details.

The TUI uses the terminal's colour support by default. Use `--no-color` for an ASCII colour profile or `--color` to force true colour. These flags conflict.

## Machine-readable output

Use `--json` on data commands. Mangate writes one JSON object, not a stream of progress events, to standard output:

~~~json
{
  "formatVersion": "1",
  "operation": "search",
  "status": "success",
  "data": {
    "provider": "mangadex",
    "query": "example title",
    "results": [
      {
        "id": "title-id",
        "title": "Example title",
        "cover": {},
        "metadata": {
          "language": "en"
        }
      }
    ]
  }
}
~~~

The stable top-level fields are:

- `formatVersion`, currently `"1"`.
- `operation`, such as `search`, `title.get`, `chapters.list`, `download`, or `archive.verify`.
- `status`.
- `data`, whose shape depends on the operation.

Successful command data commonly includes lower-camel-case fields such as `provider`, `title`, `chapters`, `format`, and output paths. Optional fields are omitted when empty.

Important statuses include:

- `success` for a completed operation.
- `no_results` for a valid search with no matches.
- `partial` for a download with both completed and failed chapters.
- `planned` inside a dry-run download or archive-conversion result.
- `error` for an error envelope.

Error output has the same top-level shape:

~~~json
{
  "formatVersion": "1",
  "operation": "command",
  "status": "error",
  "data": {
    "category": "invalid_input",
    "message": "select chapters: choose a chapter selector"
  }
}
~~~

Supported data operations use these operation names:

| Command | Operation |
| --- | --- |
| `providers` | `providers.list` |
| `provider` | `provider.inspect` |
| `search` | `search` |
| `title` | `title.get` |
| `chapters` | `chapters.list` |
| `download` | `download` or `download.plan` |
| `archive convert` | `archive.convert` or `archive.convert.plan` |
| `archive inspect` | `archive.inspect` |
| `archive verify` | `archive.verify` |
| `config` | `config.inspect` |
| `diagnostics` | `diagnostics` |

Search with no results still writes a JSON document and exits with status 1. Download progress is not mixed into JSON standard output. Use exit status and the structured category rather than parsing English terminal text.

`--json` cannot be combined with `search --interactive`. Help and completion commands remain help or completion output even if a global flag is present.

## Configuration

Mangate starts with built-in defaults, loads a JSON configuration file, and then applies CLI flags. The precedence is:

1. Built-in defaults.
2. The configuration file.
3. CLI options.

The default path comes from the operating system's user configuration directory plus `mangate/config.json`. On Linux this is normally:

~~~text
~/.config/mangate/config.json
~~~

Set `MANGATE_CONFIG` to use another file. A missing file is treated as an empty configuration and leaves the defaults in place. Invalid JSON, invalid durations, invalid values, and invalid provider URLs stop startup with a configuration error.

The supported JSON shape is:

~~~json
{
  "provider": "mangadex",
  "language": "en",
  "providers": {
    "mangadex": {
      "siteUrl": "https://mangadex.org",
      "baseUrl": "https://api.mangadex.org",
      "uploadsUrl": "https://uploads.mangadex.org"
    }
  },
  "http": {
    "timeout": "30s"
  },
  "download": {
    "dir": "./library",
    "format": "directory",
    "existingFileMode": "skip",
    "retainSource": true
  },
  "concurrency": {
    "pageDownloads": 8,
    "chapterDownloads": 6
  },
  "search": {
    "historyMax": 100
  },
  "dirs": {
    "cache": "./.cache/mangate"
  }
}
~~~

The HTTP timeout uses Go duration syntax such as `30s` or `2m`. Download format must be `directory`, `cbz`, or `zip`. Existing-file policy must be `skip`, `replace`, or `fail`. Page and chapter download counts must be positive. Search history must be zero or greater.

`mangate config` displays the effective values. The TUI settings screen can apply values for the current session with `a` or apply and save them with `s`.

Mangate does not read credentials from configuration. The current MangaDex integration uses public endpoints and has no secret or login setting.

## Environment variables

| Variable | Use | Default or override |
| --- | --- | --- |
| `MANGATE_CONFIG` | Selects the JSON configuration file. | Overrides the default configuration path. It is not sensitive. |
| `TERM` | Determines whether the TUI is allowed. | `TERM=dumb` disables TUI entry. Other values are checked together with terminal file descriptors. |
| `HOME` | Supplies the normal home directory used for default download paths. | Used by the operating system's user-directory lookup when available. |
| `XDG_CONFIG_HOME` | On Linux, influences the standard user configuration directory lookup. | If set, the default config path is under this directory unless `MANGATE_CONFIG` is set. |
| `XDG_CACHE_HOME` | On Linux, influences the standard user cache directory lookup. | If set, the default cache path is under this directory. |

The installation script also uses standard home and XDG directory variables when it creates the executable and initial configuration.

## Exit statuses

Mangate's stable process statuses are:

| Status | Meaning |
| ---: | --- |
| 0 | Complete success, including a successful no-write plan. |
| 1 | No results (structured status @@no_results@@). |
| 2 | Invalid input (structured category @@invalid_input@@), such as an empty value, malformed range, or conflicting chapter selectors. |
| 3 | Configuration could not be loaded, validated, or applied at startup. |
| 4 | Known provider or content error, including unknown provider, not found, unsupported capability, or timeout. |
| 5 | Download failure or partial or incomplete download result (structured status @@partial@@ or @@incomplete@@). |
| 6 | Recognized filesystem failure. |
| 7 | Cancellation or interruption. |
| 8 | Archive creation or archive validation failure. |
| 10 | Internal or otherwise unclassified failure. Network errors that do not match a more specific message may use this status. |

Some command-line parsing errors are returned by Cobra before Mangate can classify them and may therefore use status 10. Scripts should use JSON categories and exit statuses together, and should not depend on English error text.

## Error categories

JSON errors and verbose diagnostics use these categories:

| Category | Meaning and usual action |
| --- | --- |
| `no_results` | The search completed but found no titles. Change the query or filters. |
| `invalid_input` | A required value, range, or chapter selection is invalid. Check the command syntax and use a stable chapter ID for duplicates. |
| `unknown_provider` | The provider ID is not registered. Run `mangate providers`. |
| `not_found` | The provider could not find the requested title or resource. Check the stable reference. |
| `unsupported_capability` | The selected provider does not permit the requested operation. |
| `timeout` | A request exceeded the configured HTTP timeout. Retrying may help if the service is slow. |
| `filesystem` | Mangate could not create, write, replace, or inspect a local path. Check permissions and the destination. |
| `archive` | An archive could not be created or validated. Check its source pages and extension. |
| `cancelled` | The operation was interrupted. Completed pages remain available for a later retry. |
| `provider_or_internal` | The error did not match a narrower category. Check provider availability, network access, and verbose diagnostics. |

The Python binding uses a related but not identical set of categories. See [Python exceptions](python.md#exceptions).

## Shell completion

Mangate generates completion scripts for Bash, Zsh, and Fish:

~~~bash
source <(mangate completion bash)
source <(mangate completion zsh)
mangate completion fish | source
~~~

The first two commands activate completion in the current Bash or Zsh session. The Fish command pipes the generated script into Fish's source command. For persistent setup, save the generated script according to the shell's normal completion directory and startup rules.

## Common workflows

### Search and download one chapter

~~~bash
mangate search "example title"
mangate chapters <title-id>
mangate download <title-id> --chapter 1
~~~

### Download several chapters as CBZ

~~~bash
mangate --format cbz download <title-id> --chapter 1 --chapter 2
~~~

If the selection is broad or an existing archive must be replaced, add `--yes` after reviewing a dry run.

### Use the TUI

~~~bash
mangate tui
~~~

### Run a scripted JSON download

~~~bash
mangate --non-interactive --json --format cbz download <title-id> --chapter-id <chapter-id>
~~~

The command writes one JSON result to standard output. Keep standard error available for diagnostics.

### Convert a directory to ZIP and verify it

~~~bash
mangate --format zip archive convert ./library/Example-123/Chapter-1
mangate archive verify ./library/Example-123/Chapter-1.zip
~~~

### Resolve an ambiguous chapter number

~~~bash
mangate chapters <title-id>
mangate download <title-id> --chapter-id <release-id>
~~~

Use the exact stable chapter ID printed by the chapter list.
