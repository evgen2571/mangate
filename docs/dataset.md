# Dataset collection

`mangate dataset` collects a bounded image dataset through the same provider, downloader, and archive paths used by ordinary downloads. It keeps SQLite state at `<dataset-root>/dataset.sqlite`, stores data under stable provider, title, and chapter IDs, and writes `manifest.jsonl` plus `summary.json`.

Plan before a large run:

```bash
mangate --output ./datasets/manhwa-raw-v1 dataset plan \
  --original-language ko --chapter-language en --max-titles 1000 \
  --max-chapters-per-title 20 --max-bytes 500GiB
```

Then collect with an explicit confirmation:

```bash
mangate --format png --output ./datasets/manhwa-raw-v1 dataset collect \
  --original-language ko --chapter-language en --max-titles 1000 \
  --max-chapters-per-title 20 --max-bytes 500GiB --resume --yes
```

The collector browses provider catalog pages with Korean-origin and English-release filters, then persists a deterministic plan. Title strategies are `sequential`, `random`, and `stratified`. Stratified sampling uses a stable year bucket and publication status. Chapter strategies are `all`, `first`, `latest`, `random`, and `uniform`. Uniform selection spreads picks across the chapter sequence. When duplicate releases are disabled, Mangate prefers a release with pages, then the earliest publication time, then its stable ID.

Use `--collection-config examples/dataset-config.json` for a reproducible run. Explicit command flags take precedence over this file. The saved normalized configuration and its hash are immutable for a dataset. A resume with different output format or collection settings fails instead of changing the persisted plan.

Formats have exact behavior. `directory` preserves the downloaded encoding when it can be validated. `png` writes every final page as PNG. `jpeg` writes every final page as JPEG at quality 95 and flattens transparency to white. `cbz` and `zip` make one archive per chapter after page validation. Archive page metadata remains in the manifest even after staging files are removed.

Every accepted page has dimensions, final byte count, SHA-256 when enabled, and a small perceptual hash. Exact duplicate pages remain in state with an explicit canonical reference. Splits are deterministic by title, so chapters from one title do not cross `train`, `validation`, and `test`.

The main commands are:

```text
mangate dataset plan
mangate dataset collect --yes
mangate dataset status <dataset-root>
mangate dataset verify <dataset-root>
mangate dataset verify <dataset-root> --repair
mangate dataset export <dataset-root>
```

`status`, `verify`, and `export` do not make provider requests. `verify` is read-only unless `--repair` is supplied. A resumed collection reuses completed pages when their stored state matches local output, and it leaves partial files clearly marked. Run `dataset export` to regenerate the manifest, summary, and failure report from SQLite.

Dataset layout:

```text
dataset-root/
  dataset.sqlite
  manifest.jsonl
  summary.json
  reports/failures.jsonl
  data/<provider>/<title-id>/<chapter-id>/0001.png
```

Archive datasets replace the chapter directory with `<chapter-id>.cbz` or `<chapter-id>.zip`. Dataset files can grow quickly. Set page, byte, title, chapter, and failure limits before a production run, and retain enough space for temporary archive staging.
