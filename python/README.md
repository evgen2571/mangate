# Mangate Python package

Python 3.10+ typed API for the Mangate CLI and TUI. Platform wheels include the Go executable.

```sh
uv add mangate
```

```python
import mangate

titles = mangate.search("Solo Leveling")
chapters = mangate.chapters(titles[0], language="en")
result = mangate.download(titles[0], chapters[0], output="./chapter")
print(result.image_paths)
```

Acquire an eligible chapter automatically:

```python
import mangate

result = mangate.download_random(
    output="./chapter",
    original_language="ko",
    chapter_language="en",
    min_pages=10,
    max_attempts=20,
)
print(result.image_paths)
```

Select chapters in the existing terminal interface:

```python
import mangate

result = mangate.interactive_download(output_dir="./chapters", max_chapters=1)
for download in result.downloads:
    print(download.manga.title, download.chapter.number, download.image_paths)
```

Results are immutable typed models. Failed acquisitions raise `DownloadError`, with validated partial output in `error.result` when available. `Session` reuses provider, executable, timeout, and output defaults.

Linux x86_64/ARM64, macOS 12+ x86_64/ARM64, and Windows x86_64 are supported. Source builds require Go 1.26+.

See the [Python guide](https://github.com/evgen2571/mangate/blob/main/docs/python.md) for advanced configuration, planning, manifests, archives, datasets, and errors.
