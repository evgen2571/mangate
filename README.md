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

## Python

Launch the chapter picker from your own application, then continue with its downloaded images:

```python
from mangate import Client

result = Client().interactive_download(output_dir="./chapters", max_chapters=1)
images = result["chapters"][0]["imagePaths"]
print(images)
```

The terminal closes automatically after downloading. Each chapter has its own ordered image list; cancellation and failed downloads raise `MangateError`. The existing `search()`, `chapters()`, and `download()` methods are still available.

See the [Python guide](docs/python.md) for searching, errors, and other options.

## Docs

- [CLI commands](docs/cli.md)
- [Configuration](docs/configuration.md)
- [Dataset collection](docs/dataset.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Architecture and development](docs/architecture.md)
- [Releasing](docs/releasing.md)

Only download chapters that you're authorized to save.
