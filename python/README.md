# Mangate Python package

Python 3.10+ API for the Mangate CLI and TUI. Platform wheels include the Go executable, so you don't need Go installed.

```sh
uv add mangate
# or: pip install mangate
uv run mangate tui
```

```python
import mangate

result = mangate.download(title="Solo Leveling", chapter=1, output="./chapter")
print(result.manga.title, result.chapter.number)
print(result.image_paths)
```

`search`, `chapters`, and `download` return typed models. `DownloadError.result` preserves available partial output; only complete downloads return normally. `Session` optionally reuses defaults, and the existing dictionary-based `Client` API remains available.

```python
from mangate import Client

client = Client()
result = client.interactive_download(output_dir="./chapters", max_chapters=1)
print(result["chapters"][0]["imagePaths"])
```

Linux x86_64/ARM64 (glibc or musl), macOS 12+ x86_64/ARM64, and Windows x86_64 are supported. `Client(executable="/path/to/mangate")` overrides the bundled executable. Source installs from the Git checkout require Go 1.26+; releases publish wheels only.

See the [Python API guide](https://github.com/evgen2571/mangate/blob/main/docs/python.md) for metadata, errors, non-interactive downloads, and archive operations.
