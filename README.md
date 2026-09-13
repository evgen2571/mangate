# Mangate

Search MangaDex, inspect manga and chapters, and download chapters as image directories, PNG, JPEG, CBZ, or ZIP. Use the command line, the interactive terminal interface, or the Python wrapper.

## Install

You need Go 1.26 or newer. To install the current checkout:

```sh
git clone https://github.com/evgen2571/mangate.git
cd mangate
go install ./cmd/mangate
```

Add Go's binary directory to your `PATH` if needed. On Linux and macOS:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
mangate --version
```

Alternatively, `bash scripts/install.sh` installs into `~/.local/bin`. Set `MANGATE_BIN_DIR` to use another absolute directory. No configuration file is required.

## Download a chapter

```sh
mangate search "title"
mangate title TITLE_ID
mangate chapters TITLE_ID
mangate --format cbz --output ./library download TITLE_ID --chapter-id CHAPTER_ID --yes
```

Replace `TITLE_ID` and `CHAPTER_ID` with IDs from the preceding commands. Download only chapters you are authorized to save. Use `--dry-run` to inspect a download plan before writing files.

## Use the terminal interface

```sh
mangate tui
```

Type a search and press Enter. Choose a title with the arrow keys, press Enter to load chapters, toggle chapters with Space, then continue through format, output directory, and review. Press `?` for help outside text inputs, Escape to go back, and `q` to cancel a download. A wide results screen shows the selected title's cover and synopsis.

A chapter selection screen using the test fixture:

```text
MANGATE  /  Chapters
Search › Title › Chapters › Format › Output › Review

Fixture One
2 chapters · 1 selected
› ●  Chapter 1 - Opening          en  2 pages
  ○  Chapter 2 - Unavailable      en  2 pages

space toggle · enter continue · / filter · ? help
```

## Python and reference

Install the wrapper from this checkout with `uv pip install ./python`, then use `Client` with the installed executable. See the [Python guide](docs/python.md) for complete examples and error handling.

- [CLI commands](docs/cli.md)
- [Configuration](docs/configuration.md)
- [Dataset collection](docs/dataset.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Architecture and development](docs/architecture.md)
