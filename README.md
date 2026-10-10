# Mangate

Mangate is a small command-line and terminal app for downloading manga and manhwa chapters from MangaDex. It can save chapters as image folders, PNG, JPEG, CBZ, or ZIP, and it has a Python wrapper for scripts.

## Why

I needed to download manhwa chapters from Python for another project, with some options specific to my use case. I looked at existing downloaders, but none really fit what I needed, so I decided to make my own.

## Install

You'll need Go 1.26 or newer:

```sh
git clone https://github.com/evgen2571/mangate.git
cd mangate
go install ./cmd/mangate
```

Make sure Go's binary directory is in your `PATH`. Alternatively, run `bash scripts/install.sh` to install to `~/.local/bin`.

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

Mangate also has a small Python wrapper around the CLI. Install it from the checkout:

```sh
uv pip install ./python
```

The `mangate` executable must also be installed and available on your `PATH`.

```python
from mangate import Client

client = Client(output_dir="./chapters")
result = client.download(
    "TITLE_ID",
    chapter_ids=["CHAPTER_ID"],
    output_format="directory",
    assume_yes=True,
)

print(result["chapters"][0]["outputPath"])
```

See the [Python guide](docs/python.md) for searching, errors, and other options.

## Docs

- [CLI commands](docs/cli.md)
- [Configuration](docs/configuration.md)
- [Dataset collection](docs/dataset.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Architecture and development](docs/architecture.md)

Only download chapters that you're authorized to save.
