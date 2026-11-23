# Python guide

Mangate needs Python 3.10+. The wheels bundle the CLI, so you don't need Go. Install with `uv add mangate` or `pip install mangate`.

Wheels support Linux and macOS (x86-64 and ARM64) and Windows (x86-64). On macOS, use version 12 or later. For source and editable installs, see [development](architecture.md).

## Find and download a chapter

```python
import mangate

titles = mangate.search("Solo Leveling", limit=5)
if not titles:
    raise SystemExit("No matching titles")

releases = mangate.chapters(titles[0], language="en")
if not releases:
    raise SystemExit("No English chapters for this title")

result = mangate.download(titles[0], releases[0], output="./chapter")
print(result.image_paths)
```

This example takes the first result and the first English release. For a specific release, pass `title_id=` and `chapter_id=`. You can also pass a title name and chapter number, but Mangate rejects ambiguous matches instead of guessing.

With `download()`, `output=` is the exact chapter directory. Use an empty directory or reuse one Mangate created for the same chapter. Without `output=`, Mangate uses its usual title/chapter hierarchy. `result.image_paths` contains absolute `Path` objects in page order.

`DownloadOptions` sets `existing_files` (`skip`, `replace`, `fail`), `output_format` (`directory`, `png`, `jpeg`), chapter `language`, and `page_downloads`. Use `Session` to reuse provider, timeout, executable, and output defaults across calls.

## Pick chapters in the terminal

```python
import mangate

result = mangate.interactive_download(output_dir="./library", max_chapters=1)
for chapter in result.downloads:
    print(chapter.manga.title, chapter.chapter.number, chapter.image_paths)
```

Run this in an interactive terminal. `max_chapters=0` lets the user select any number of chapters. For several known releases without a prompt, use `download_many()` with `chapter_ids=` and `output_dir=`. Pass `assume_yes=True` for selections of 25 chapters or more.

Python downloads return page files. To make a CBZ afterward, call `mangate.archive.convert(chapter.output, output_format="cbz")` with a chapter result. Conversion keeps the source files by default.

## Pick a random chapter and save the selection

```python
import mangate

plan = mangate.plan_random(
    "./random-chapter", original_language="ko", chapter_language="en", min_pages=10
)
result = mangate.execute(plan, timeout=120, manifest="./acquired.json")
print(result.image_paths)
```

`plan_random()` picks a chapter without downloading its images. Use `download_random()` when you want both steps in one call. `execute()` also accepts a manifest path and rechecks the recorded chapter without choosing a new one. `verify_manifest()` checks saved files locally; keep manifests outside the chapter directory. See [random downloads and replay](cli.md#random-acquisition-planning-and-replay) for the difference between replaying and verifying.

For bounded collections instead of single downloads, use `mangate.dataset.plan()`, `collect()`, `status()`, `verify()`, and `export()` with the [dataset guide](dataset.md).

## Handle failures

Invalid Python arguments can raise `ValueError`. CLI failures raise `MangateError`, which exposes `category`, `message`, and `stderr`. Acquisition failures (including cancellation and timeout) raise `DownloadError`, a subclass of `MangateError`.

Look at `error.result` for partial output. It can be `None` if the process stopped before writing a usable result. Retry the same chapter and destination with `existing_files="skip"` to reuse matching pages. For headless single-chapter and random downloads, `timeout=` bounds the subprocess and `cancel_event=` accepts a `threading.Event`.
