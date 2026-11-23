# CLI guide

Run `mangate tui` to browse and pick chapters interactively. For scripts or repeatable downloads, use the commands below. Add `--help` to any command to see all its options.

## Find a chapter

```sh
mangate search "Solo Leveling" --limit 5
mangate chapters TITLE_ID --chapter-language en
```

Use the title ID from `search` in `chapters`, then copy the chapter ID for the release you want. `mangate title TITLE_ID` shows title details. `--chapter-language` filters translations; an explicit `--language` on `search` filters by the title's original language.

## Download it

```sh
mangate --output ./library download TITLE_ID --chapter-id CHAPTER_ID
mangate download TITLE_ID --range 1-3 --chapter-language en --dry-run
```

`--dry-run` previews the selection without saving pages. Select by ID, number (`--chapter`), or range (`--range`, `--before`, `--after`). `--first`, `--latest`, and `--all` each stand alone. If a chapter number matches more than one release, use the chapter ID instead.

By default, `directory` keeps the original image encodings. Use `--format png` or `--format jpeg` to convert pages, or `--format cbz` or `--format zip` for one archive per chapter. Here's what chapter 1 would look like as a folder or a CBZ:

```text
library/Example-Title-TITLE_ID/Chapter-1/0001.jpg  # directory: one file per page
library/Example-Title-TITLE_ID/Chapter-1.cbz      # cbz: one file per chapter
```

Archive downloads keep only the archive once it passes validation. For scripts, `--non-interactive` disables prompts and `--yes` confirms actions that need approval. The default `--existing-files skip` reuses matching files on retry; use `fail` to reject existing output or `replace` to overwrite it.

Use `--chapter-output ./chapter` to write one chapter directly into a folder without the title/chapter hierarchy. It doesn't support archive formats or unrelated nonempty destinations.

## Convert a chapter you already have

```sh
mangate --format cbz archive convert ./chapter --output ./chapter.cbz
mangate archive verify ./chapter.cbz
```

Here `--output` is the archive file, not a directory. Conversion keeps the source pages unless you pass `--remove-source --yes`. Mangate validates the archive before removing the pages.

## Random acquisition, planning, and replay

```sh
mangate random --original-language ko --chapter-language en --min-pages 10 --output ./chapter
```

This looks for one English chapter of a Korean title with at least 10 pages. `--output` names the exact chapter folder. Set `--max-attempts` to limit the search (default 20). If none qualifies, Mangate fails rather than downloading a fallback.

To review the pick before downloading, save a plan and replay it:

```sh
mangate random --plan --seed 42 --min-pages 10 --output ./planned-chapter --manifest ./selection.json
mangate download --from-manifest ./selection.json --manifest ./acquired.json
mangate download --from-manifest ./acquired.json --verify-manifest
```

A seed repeats the selection only if the candidate catalog stays the same. Replay uses the recorded IDs and checks that the chapter still qualifies. Verification is local: it requires a complete version-2 manifest and checks that the saved images haven't changed. Keep manifests outside the chapter folder, and preserve their relative paths to the images if you move them.

## Script the CLI

- `--json` returns one envelope with `formatVersion`, `operation`, `status`, and `data`. Keep stdout even on a nonzero exit: no results (`1`) and partial downloads (`5`) can still return useful data.
- Random and replay commands support `--jsonl` for progress and a final envelope, or `--events-file PATH` for progress in a separate new file. Don't combine `--jsonl` with `--json` or `--quiet`.
- To let a person pick chapters for your script, use `mangate tui --exit-on-complete --result-file ./result.json --max-chapters 1`. Run it in a real terminal, use a new result path, and don't pass `--json`. The result file contains the selections and saved image paths; `--max-chapters 0` leaves the selection unlimited.
- `mangate diagnostics` checks local setup without contacting MangaDex. `mangate completion bash` prints a completion script; use `zsh` or `fish` for those shells.

See [configuration](configuration.md) to save your defaults, or [troubleshooting](troubleshooting.md) if a download stalls or fails.
