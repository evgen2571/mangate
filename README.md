# Mangate

Download manga and manhwa chapters from MangaDex. Pick chapters in an interactive terminal, or automate downloads with the CLI or Python. Save them as image folders, CBZ, or ZIP.

## Get started

Download the archive for your system from [GitHub Releases](https://github.com/evgen2571/mangate/releases). Extract it, put `mangate` (`mangate.exe` on Windows) on your `PATH`, and run:

```sh
mangate tui
```

Search for a title, press **Space** to select chapters, then choose where and how to save them. Press **?** outside a text field for key bindings.

Already using Python? Run `uv add mangate`, then `uv run mangate tui`. Both the release archives and the Python wheels include the executable; you don't need Go to use them.

## Download from the command line

```sh
mangate search "Solo Leveling"
mangate chapters TITLE_ID --chapter-language en
mangate --output ./library --format cbz download TITLE_ID --chapter-id CHAPTER_ID --yes
```

Copy `TITLE_ID` from the search results and `CHAPTER_ID` from the chapter list. This saves one CBZ under `./library`. Leave out `--format cbz` to keep the original page images with the default settings.

## Guides

- [CLI](docs/cli.md) — choose chapters, preview downloads, or replay a random pick.
- [Python](docs/python.md) — download chapters and work with the saved page paths.
- [Configuration](docs/configuration.md) and [troubleshooting](docs/troubleshooting.md) — set defaults or recover a download.
- [Dataset collection](docs/dataset.md) — plan, validate, and export a batch of images.
- [Development](docs/architecture.md) and [releasing](docs/releasing.md) — build, test, and publish Mangate.

Only download chapters you're authorized to save.
