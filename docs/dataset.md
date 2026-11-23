# Dataset collection

Use [ordinary downloads](cli.md) to save chapters for reading. Use `dataset` when you need to collect images within a budget, resume the same selection, and export a manifest.

## Plan, then collect

Try the [sample config](../examples/dataset-config.json) from the repository root. It targets Korean titles with English chapters and caps the collection at 100 pages.

```sh
mangate dataset plan --collection-config examples/dataset-config.json
mangate dataset collect --collection-config examples/dataset-config.json --resume --yes
```

`plan` contacts the provider and saves the selection, but doesn't download pages. Read its output before confirming collection with `--yes`. To resume, keep the same settings and dataset root. If you want a different selection, choose a new root.

Edit the sample to change languages, sampling, image validation, or splits. Keep at least one positive stopping limit, such as `--max-pages` or `--max-bytes`. Explicit flags override the file. Use `directory`, `png`, or `jpeg`; datasets don't use archives. If you enable splits, all chapters from the same title stay in one split.

## Read the output

```text
datasets/manhwa-smoke-test/
├── dataset-state.json       # Keep this to resume the same plan.
├── manifest.jsonl           # One record per exported page.
├── summary.json             # Counts and collection totals.
├── reports/failures.jsonl   # What failed, if anything.
└── data/                   # Saved pages, grouped by title and chapter.
```

```sh
mangate dataset status ./datasets/manhwa-smoke-test
mangate dataset verify ./datasets/manhwa-smoke-test
mangate dataset export ./datasets/manhwa-smoke-test
```

These commands work locally. `verify` checks the saved files; add `--repair` only if you want it to update state and regenerate exports. Use `export --split train` to export just that split. You can regenerate exports, but keep `dataset-state.json` and the saved pages if you plan to resume.
