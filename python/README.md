# Mangate for Python

Download MangaDex chapters from Python and get the saved page paths back in order. Mangate needs Python 3.10+; the wheels include the CLI, so you don't need Go.

```sh
uv add mangate
```

To choose one chapter in an interactive terminal:

```python
import mangate

result = mangate.interactive_download(output_dir="./library", max_chapters=1)
for chapter in result.downloads:
    print(chapter.image_paths)
```

For scripts that run without a terminal, use `search()`, `chapters()`, and `download()`. See the [Python guide](https://github.com/evgen2571/mangate/blob/main/docs/python.md) for examples, random selection, and handling partial downloads.
