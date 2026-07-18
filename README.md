# Mangate

Mangate is a Go command-line application for searching and downloading manga / manhwa from supported providers. It offers a direct CLI for quick commands and scripts, a minimal full-screen TUI for interactive use, extensible provider support, and Python bindings.

## Features

- Search supported manga and manhwa providers.
- View title information and available chapters.
- Select individual chapters, ranges, or multiple chapters to download.
- Download through the CLI or the interactive TUI, with progress and partial-failure reporting.
- Save chapters as ordered image directories, PNG, JPEG, CBZ, or ZIP output.
- Collect bounded, resumable datasets with provenance and validation records.
- Skip and reuse existing downloads, or choose a replacement policy.
- Produce machine-readable JSON output for scripts.
- Use Mangate's main operations from Python.

## Installation

Install with Go 1.26 or newer:

```bash
go install github.com/evgen2571/mangate/cmd/mangate@latest
```

To build the current checkout instead:

```bash
go build -o mangate ./cmd/mangate
```

The repository also includes an installation script that places the executable in `~/.local/bin`:

```bash
./scripts/install.sh
```

## Quick start

Search for a title, then use its ID from the results to inspect chapters or download one:

```bash
mangate search "title"
mangate title <title-id>
mangate chapters <title-id>
mangate --format cbz download <title-id> --chapter 1
```

Open the full-screen TUI with:

```bash
mangate tui
```
or simply
```bash
mangate
```

Use the built-in help for other commands and options:

```bash
mangate --help
mangate <command> --help
```

## Documentation

- [CLI documentation](docs/cli.md)
- [Python bindings](docs/python.md)
- [Dataset collection](docs/dataset.md)

## Output formats

- `directory` - ordered image files in a chapter directory.
- `png` - lossless PNG page output.
- `jpeg` - JPEG page output at quality 95, with transparent pixels flattened to white.
- `cbz` - one comic-book archive per chapter.
- `zip` - one standard ZIP archive per chapter.

## Dataset collection

Build a bounded collection without manually finding title IDs:

```bash
mangate --format directory --output ./datasets/manhwa-raw-v1 dataset collect \
  --original-language ko --chapter-language en --max-titles 1000 \
  --max-chapters-per-title 20 --max-bytes 500GiB --resume --yes
```

Use only provider access and content you are authorized to download. Provider terms and availability can change.

## Python bindings

Mangate's main operations are available from Python. Install the package from this repository with:

```bash
python -m pip install ./python
```

```python
from mangate import Client

client = Client()
titles = client.search("title", limit=5)
chapters = client.chapters(titles[0]["id"])
```

See the [full Python bindings guide](docs/python.md) for configuration, downloads,
archive operations, cancellation, and error handling.


## Project status

Mangate is a pet project.

## Contributing

Bug reports and contributions are welcome. Please open an issue or pull request with a clear description of the change.

## License

[MIT License](LICENSE).
