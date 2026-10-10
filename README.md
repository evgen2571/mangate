# Mangate

Mangate is a small command-line and terminal app for downloading manga and manhwa chapters from MangaDex. It can save chapters as image folders, PNG, JPEG, CBZ, or ZIP, and it has a Python wrapper for scripts.

## Why

I needed to download manhwa chapters from Python for another project, with some options specific to my use case. I looked at existing downloaders, but none really fit what I needed, so I decided to make my own.

## Install

Download the archive for your system from [GitHub Releases](https://github.com/evgen2571/mangate/releases), extract it, and put `mangate` (`mangate.exe` on Windows) on your `PATH`. No Go installation needed.

For Python projects:

```sh
uv add mangate
# or: pip install mangate
uv run mangate tui
```

The Python wheel includes the executable. Releases support Linux and macOS on x86_64/ARM64, and Windows on x86_64. Building from this checkout still needs Go 1.26+: `go install ./cmd/mangate` or `bash scripts/install.sh`.

## Usage

Find a title, list its chapters, and download one:

```sh
mangate search "title"
mangate chapters TITLE_ID
mangate --output ./chapters download TITLE_ID --chapter-id CHAPTER_ID
```

Use the IDs shown by the search and chapter commands. The default output is an image directory. For a CBZ archive, use `--format cbz` and `--yes`.

For an interactive interface, run:

```sh
mangate tui
```

Search for a title, choose the chapters you want, and select an output format and directory.

To automatically select and download one eligible chapter:

```sh
mangate random --original-language ko --chapter-language en --min-pages 10 --output ./chapter
```

## Python

For scripts and automated workflows, download an eligible chapter directly:

```python
import mangate

result = mangate.download_random(
    output="./chapter", original_language="ko", chapter_language="en", min_pages=10
)
print(result.image_paths)
```

For manual chapter selection from Python, use `Client().interactive_download(output_dir="./chapters", max_chapters=1)`. The terminal picker closes automatically after downloading. The function-first `search()`, `chapters()`, and `download()` APIs also support explicit selections.

See the [Python guide](docs/python.md) for selection policies, planning, manifests, progress, cancellation, and errors.

## Docs

- [CLI commands](docs/cli.md)
- [Configuration](docs/configuration.md)
- [Dataset collection](docs/dataset.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Architecture and development](docs/architecture.md)
- [Releasing](docs/releasing.md)

Only download chapters that you're authorized to save.
