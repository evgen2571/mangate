# Dataset collection

Use ordinary [download commands](cli.md) to save chapters for reading. `mangate dataset` is a separate workflow for bounded image collection with a persisted plan, validation, provenance, and exports. It writes ordered page directories rather than chapter archives.

## Plan a small collection

```sh
mangate --output ./datasets/sample dataset plan --original-language ko --chapter-language en --candidate-pool-size 20 --max-titles 2 --max-chapters-per-title 1 --max-pages 100
```

Planning contacts the provider and writes `dataset-state.json`; it is not a read-only preview. Review the plan before collecting pages. Keep the same configuration when resuming:

```sh
mangate --output ./datasets/sample dataset collect --original-language ko --chapter-language en --candidate-pool-size 20 --max-titles 2 --max-chapters-per-title 1 --max-pages 100 --resume --yes
```

`collect --dry-run` creates or shows the plan without downloading pages. Collection requires explicit confirmation. `--resume` reuses persisted state and rejects configuration changes rather than silently changing the plan. Choose a new dataset root for different settings.

## Bounds and selection

Set at least one positive stopping condition such as a title, page, or byte limit. Useful flags include:

| Flag | Purpose |
| --- | --- |
| `--candidate-pool-size` | Maximum catalog candidates considered |
| `--max-titles` | Maximum selected titles |
| `--max-chapters-per-title` | Maximum chapters from each title |
| `--max-pages` | Maximum final pages |
| `--max-bytes` | Final-byte budget, such as `1GiB` |
| `--max-failures` | Failed-chapter stopping threshold |
| `--seed` | Deterministic sampling seed |
| `--title-strategy` | `sequential`, `random`, or `stratified` |
| `--chapter-strategy` | `all`, `first`, `latest`, `random`, or `uniform` |

Zero disables supported count/byte limits; do not disable every stopping condition. Discovery filters include `--original-language`, `--chapter-language`, `--status`, `--content-rating`, `--include-tag`, and `--exclude-tag`. `--keep-duplicate-releases` retains multiple releases of a logical chapter.

Defaults target Korean-origin titles with English chapters. Set filters deliberately for your collection. Use `mangate dataset plan --help` for the complete flag list.

## Formats and configuration

`--format directory` preserves downloaded encodings that pass validation. `png` and `jpeg` convert final page files. CBZ and ZIP are rejected for datasets.

`--collection-config PATH` reads a separate JSON collection configuration. Explicit collection flags override that file. The normalized configuration and its hash are persisted with the plan. Its sections are `output`, `discovery`, `sampling`, `limits`, `validation`, `splits`, and `runtime`, plus `version`, `datasetId`, and `provider`. The exact fields and validation rules are defined by `internal/dataset/config.go`.

Validation records image dimensions, byte counts, optional SHA-256, and perceptual hashes. Duplicate and rejected records remain distinguishable from accepted pages. Dataset splits are assigned by title so a title's chapters stay in one split.

## Inspect, verify, and export

```sh
mangate dataset status ./datasets/sample
mangate dataset verify ./datasets/sample
mangate dataset export ./datasets/sample
```

These commands operate on local state without provider downloads. `verify --repair` changes state and regenerates exports; omit `--repair` for inspection. `export` supports `--split`, `--include-duplicates`, and `--include-rejected`.

The dataset root contains:

```text
dataset-state.json
manifest.jsonl
summary.json
reports/failures.jsonl
data/<title>/chapter-<number>/0001.png
```

The extension depends on output format. Treat `dataset-state.json` as the resumable source of truth; exports can be regenerated. Keep completed page directories when recovering a failed collection. Resume with the original settings, then inspect the failure report and verification output before using the dataset.

Python equivalents are described in the [Python guide](python.md). Dataset runtime limits and validation are separate from ordinary downloader settings; do not assume the two configuration files are interchangeable.
