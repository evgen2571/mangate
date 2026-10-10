# CLI commands

Use `mangate --help` or append `--help` to any command for its complete flag list. Examples use `TITLE_ID` and `CHAPTER_ID` as placeholders for IDs returned by the provider.

## Search and inspect

```sh
mangate search "title" --limit 10
mangate --json search "title"
mangate title TITLE_ID
mangate chapters TITLE_ID --chapter-language en
mangate provider mangadex
mangate providers
```

Search accepts a title query. `--limit` limits displayed results; the provider retrieves the matching collection before the CLI applies this limit. `--content-type` filters content ratings. An explicitly supplied `--language` filters search results by original language as well as setting the preferred language for provider text.

Title output includes metadata and descriptions. Chapters include stable IDs, chapter numbers, language, and release information when supplied by the provider. A missing chapter number stays unknown; select such a chapter by ID.

## Download

```sh
mangate --output ./library download TITLE_ID --chapter-id CHAPTER_ID
mangate download TITLE_ID --chapter 1 --dry-run
mangate download TITLE_ID --range 1-5 --chapter-language en
mangate download TITLE_ID --latest
mangate --format cbz download TITLE_ID --first --yes
```

Choose one selection strategy:

| Selector | Behavior |
| --- | --- |
| `--chapter-id ID` | Exact release; repeat for several IDs |
| `--chapter NUMBER` | Exact chapter number; ambiguous releases require an ID |
| `--range START-END` | Inclusive chapter-number range |
| `--before NUMBER`, `--after NUMBER` | Chapters before or after a label |
| `--first`, `--latest` | First or last entry in the provider's ordered chapter list |
| `--all` | All chapters after language filtering |

Do not combine `--first`, `--latest`, or `--all` with another strategy. Explicit IDs/numbers cannot be mixed with range selectors. `--chapter-language` filters releases; it is separate from the preferred language setting.

`--dry-run` prints the plan without downloading pages or creating archives. Archive downloads, replacements, and broad selections may require confirmation. Use `--yes` for an intentional non-interactive operation. `--non-interactive` prevents prompts and the TUI.

## Output and recovery

| `--format` | Output |
| --- | --- |
| `directory` | Ordered pages preserving downloaded encodings |
| `png` | Ordered PNG pages |
| `jpeg` | Ordered JPEG pages, transparency flattened onto white |
| `cbz` | One comic archive per chapter |
| `zip` | One ZIP archive per chapter |

The output root contains a title directory and chapter outputs. Names are sanitized and collisions within a selection are disambiguated. Completed page directories contain `.mangate.json` with chapter state. Archive downloads remove their source directory after archive validation.

The default `--existing-files skip` reuses completed page files and matching valid archives. Use `replace` to replace output or `fail` to reject existing output. A failed chapter retains completed pages for retry. A failed replacement preserves an old page until its replacement is finalized. See [troubleshooting](troubleshooting.md) for partial downloads.

## Convert and inspect local archives

```sh
mangate --format cbz archive convert ./chapter --output ./chapter.cbz
mangate --format zip archive convert ./chapter --output ./chapter.zip --dry-run
mangate archive inspect ./chapter.cbz
mangate archive verify ./chapter.cbz
```

Conversion preserves page bytes. It keeps the source directory unless `--remove-source` is supplied. Source removal and replacement require confirmation; pass `--yes` when scripting either action. Conversion's `--output` names the archive file, whereas download's global `--output` names a root directory.

## TUI and local utilities

```sh
mangate tui
mangate config
mangate diagnostics
mangate completion bash
mangate completion zsh
mangate completion fish
```

Bare `mangate` opens the TUI when attached to an interactive terminal, otherwise it shows help. The results screen shows a selected-title cover and synopsis on wide terminals. Use arrows to navigate, Enter to continue, Space to select chapters, Escape to go back, and `?` for help outside text fields. Press `q` during a download to cancel it.

`config` shows effective settings. `diagnostics` checks local setup without provider requests. `completion` prints a script for the chosen shell. Detailed options and precedence are in [configuration](configuration.md); dataset commands have their own [guide](dataset.md).

## JSON and process status

`--json` emits an envelope with `formatVersion`, `operation`, `status`, and `data`. `--quiet` suppresses ordinary human-readable output. `--verbose` adds diagnostic categories to stderr. Success uses `status: "success"`; a download's data also has its own completion status.

Empty searches return `status: "no_results"` and exit 1. Partial downloads return chapter results with `status: "partial"` and exit 5. Fully unsuccessful downloads use `status: "incomplete"`. Do not discard stdout solely because the process exited nonzero.

Other command failures use an error envelope containing `data.category` and `data.message`. Configuration-loading errors happen before command execution and are written to stderr with exit 3. General mappings include invalid input 2, provider lookup/timeouts 4, filesystem errors 6, cancellation 7, archive errors 8, and unclassified failures 10. Download reporting can use its own aggregate exit code. Treat the JSON category and chapter outcomes as the detailed result rather than inferring everything from a numeric code.

## Interactive integration result

```sh
mangate tui --exit-on-complete --result-file /tmp/mangate-result.json --max-chapters 1 --output ./chapters
```

`--exit-on-complete` enables integration mode; ordinary `mangate tui` retains its normal format selection and completion screen. Supply a new result-file path in an existing writable parent directory. The result is written atomically after the terminal is restored. It contains title metadata and separate per-chapter metadata, output directories, and ordered image paths. Integration output uses image directories. It does not write JSON into the terminal or change direct commands' JSON envelopes.

`--max-chapters 0` (default) permits multiple chapters. A positive value limits the selection. Integration flags must be used together with `--exit-on-complete`; `--json` is incompatible with this mode.

Exit status `0` means a complete validated download, `7` means cancellation, `5` means incomplete/partial download, and `10` means an interactive runtime error. Startup/argument/filesystem errors can occur before a result exists. A failed chapter never includes consumable image paths. See the [Python guide](python.md) for the result schema and exception handling.

### Exact single-chapter directories

```sh
mangate --json download <title-id> --chapter-id <chapter-id> --chapter-output ./chapter
```

`--chapter-output` writes one selected chapter directly into that directory, without title/chapter subdirectories. It supports `directory`, `png`, and `jpeg`; multi-chapter or archive selections are rejected. `--output` and `--download-dir` retain their existing organized output-root behavior. Nonempty exact destinations require matching Mangate chapter/provider metadata, including for partial-download resume. Unrelated files and symlink destinations are rejected.

JSON download records retain chapter selection order. Each chapter includes full `chapter` metadata and ordered absolute `imagePaths` for finalized pages. Top-level status distinguishes `complete`, `partial`, `failed`, and `cancelled`; partially acquired chapters do not count as complete. Archived chapters expose `archivePath` and omit consumable image paths after source removal.

## Random acquisition, planning, and replay

```sh
mangate random --original-language ko --chapter-language en --min-pages 10 --max-attempts 20 --output ./output/chapter --manifest ./chapter.json
mangate --json random --plan --seed 42 --min-pages 10 --output ./output/chapter --manifest ./selection.json
mangate download --from-manifest ./selection.json
mangate download --from-manifest ./chapter.json --output ./replayed-chapter --manifest ./replayed.json
mangate download --from-manifest ./chapter.json --verify-manifest
mangate random --min-pages 10 --max-attempts 20 --output ./output/chapter --jsonl
```

`random` selects exactly one eligible chapter. Its `--output` is the exact chapter directory; `directory`, `png`, and `jpeg` formats are supported. Existing-file behavior uses the ordinary `--existing-files skip|replace|fail` policy. It never invokes dataset collection. Providers must permit downloads and implement the requested optional random or catalog capability; unsupported operations fail explicitly.

Filters include original language, translation language, `--min-pages` (default 1), `--max-pages`, inclusive numeric `--chapter-min`/`--chapter-max`, and `--max-attempts` (default 20). Missing required language or numeric metadata rejects a candidate. Unknown page counts are resolved from the provider page list; contradictory counts or unavailable pages reject the chapter. Duplicate titles consume attempts without repeating chapter inspection. Exhaustion never downloads a rejected candidate. Missing or invalid candidate chapter lists/pages are rejected within the attempt budget (MangaDex HTTP 404/410 are candidate failures). Cancellation, deadlines, authentication, and other provider-wide or unclassified errors stop the operation immediately. Rejection events use fixed reasons rather than raw provider responses. Requests retain cancellation, HTTP timeouts, rate pacing, and bounded retries.

Without `--seed`, MangaDex's native random endpoint selects candidates; results are nondeterministic. With `--seed`, selection uses a bounded catalog snapshot (at most `max-attempts` distinct titles, ordered by creation time for catalog reads). The recorded seed and candidate IDs explain that selection; a future live catalog may differ. Within a title, suitable chapters are considered in stable ID order. This is structural eligibility, without image scoring or image analysis.

`--plan` resolves a stable selection and expected page count without transferring images. Use `--manifest` to save it for later execution. Save manifests outside the chapter directory to keep image files and chapter state safe. Version-2 manifests record the plan, policy, selection strategy, seed/candidates where applicable, relative output and image paths, detected formats, SHA-256 checksums and byte sizes for images, page counts, terminal status, and safe failure reasons. Keep the manifest with its relative output tree when moving it between machines. Page URLs and credentials are not persisted.

`download --from-manifest` replays the selected IDs without random discovery, revalidates current eligibility and page counts, and applies the current existing-file policy. `--output` can relocate replay; provider/format overrides must agree with the plan. Chapter selectors cannot be combined with replay. `--verify-manifest` instead checks existing files locally without contacting the provider; it requires a complete version-2 manifest, matching provider/title/chapter state with `complete=true` and the expected page count, and unchanged regular image files. Version-1 manifests remain loadable and replayable but cannot pass integrity verification because they lack original checksums; replay produces a version-2 manifest. Plans retain version 1. Replay and verification are different operations.

`--json` still writes one result envelope. `--jsonl` streams discovery, selection, validation, downloading, and completion events followed by one terminal result/error envelope; it cannot be combined with `--json` or `--quiet`. Human diagnostics stay on stderr. `--events-file` writes progress JSON Lines to a new file while retaining the ordinary result output. Event and manifest destinations must differ. Progress encoding, write, flush, and close failures make the command fail; acquired images and available manifests remain intact. If acquisition completed but reporting failed, the result is `partial` with the complete acquisition manifest retained. A failed JSONL stdout stream receives no further writes, since its last line may be incomplete. A completion event describes acquisition state; the terminal result/exit status also accounts for reporting finalization failures. These automation flags apply to `random` and manifest replay. Interrupted acquisitions retain finalized ordered pages and, when requested, an incomplete manifest; forced termination can prevent a terminal result from being written.
