"""Structured, non-interactive access to the Mangate executable."""

from __future__ import annotations

import json
import math
import os
import signal
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Iterable
from dataclasses import dataclass
from pathlib import Path
from threading import Event
from typing import Any

from .binary import resolve_executable
from .models import DownloadResult, ProgressEvent


@dataclass(slots=True)
class MangateError(RuntimeError):
    """A Mangate operation failure with a stable, programmatic category."""

    category: str
    message: str
    stderr: str = ""
    result: dict[str, Any] | DownloadResult | None = None

    def __str__(self) -> str:
        return self.message


class Client:
    """Run Mangate operations without parsing terminal-oriented output.

    Args:
        executable: Path or command name for the Mangate CLI.
        provider: Default provider identifier.
        output_dir: Optional root directory for downloaded chapters.
        timeout: Per-command timeout in seconds.
        page_downloads: Maximum simultaneous page transfers.
        chapter_downloads: Maximum simultaneous chapter transfers.
    """

    def __init__(
        self,
        executable: str | os.PathLike[str] | None = None,
        *,
        provider: str = "mangadex",
        output_dir: str | os.PathLike[str] | None = None,
        timeout: float | None = None,
        page_downloads: int | None = None,
        chapter_downloads: int | None = None,
        existing_files: str = "skip",
        output_format: str = "directory",
    ) -> None:
        if timeout is not None and (not math.isfinite(timeout) or timeout <= 0):
            raise ValueError("timeout must be finite and positive")
        self.executable = (
            resolve_executable() if executable is None else os.fspath(executable)
        )
        self.provider = provider.strip().lower()
        self.output_dir = None if output_dir is None else os.fspath(output_dir)
        self.timeout = timeout
        self.page_downloads = page_downloads
        self.chapter_downloads = chapter_downloads
        existing_files = existing_files.strip().lower()
        output_format = output_format.strip().lower()
        if existing_files not in {"skip", "replace", "fail"}:
            raise ValueError("existing_files must be skip, replace, or fail")
        if output_format not in {"directory", "png", "jpeg", "cbz", "zip"}:
            raise ValueError("output_format must be directory, png, jpeg, cbz, or zip")
        self.existing_files = existing_files
        self.output_format = output_format

    def version(self) -> str:
        return self._text(["--version"]).strip()

    def providers(self) -> list[dict[str, Any]]:
        return self._json(["providers"])["data"]

    def provider_info(self, provider: str | None = None) -> dict[str, Any]:
        return self._json(
            [
                "--provider",
                provider or self.provider,
                "provider",
                provider or self.provider,
            ]
        )["data"]

    def search(
        self,
        query: str,
        *,
        provider: str | None = None,
        limit: int | None = None,
        cancel_event: Event | None = None,
    ) -> list[dict[str, Any]]:
        args = ["--provider", provider or self.provider, "search"]
        if limit is not None:
            args.extend(["--limit", str(limit)])
        args.append(query)
        return self._json(args, cancel_event=cancel_event)["data"]["results"]

    def title(self, title_id: str, *, provider: str | None = None) -> dict[str, Any]:
        return self._json(["--provider", provider or self.provider, "title", title_id])[
            "data"
        ]

    def chapters(
        self,
        title_id: str,
        *,
        provider: str | None = None,
        limit: int | None = None,
        language: str | None = None,
    ) -> list[dict[str, Any]]:
        args = ["--provider", provider or self.provider, "chapters"]
        if limit is not None:
            args.extend(["--limit", str(limit)])
        if language:
            args.extend(["--chapter-language", language])
        args.append(title_id)
        return self._json(args)["data"]["chapters"]

    def download(
        self,
        title_id: str,
        *,
        provider: str | None = None,
        chapter_ids: Iterable[str] = (),
        chapter_numbers: Iterable[str] = (),
        chapter_range: str | None = None,
        first: bool = False,
        latest: bool = False,
        all_chapters: bool = False,
        language: str | None = None,
        output_format: str | None = None,
        dry_run: bool = False,
        assume_yes: bool = False,
        cancel_event: Event | None = None,
        output: str | os.PathLike[str] | None = None,
    ) -> dict[str, Any]:
        args = ["--provider", provider or self.provider, "download"]
        if output is not None:
            args.extend(["--chapter-output", os.fspath(output)])
        for chapter_id in chapter_ids:
            args.extend(["--chapter-id", chapter_id])
        for number in chapter_numbers:
            args.extend(["--chapter", number])
        if chapter_range:
            args.extend(["--range", chapter_range])
        if first:
            args.append("--first")
        if latest:
            args.append("--latest")
        if all_chapters:
            args.append("--all")
        if language:
            args.extend(["--chapter-language", language])
        if output_format is not None:
            if output_format.lower() not in {"directory", "png", "jpeg", "cbz", "zip"}:
                raise ValueError(
                    "output_format must be directory, png, jpeg, cbz, or zip"
                )
            args.extend(["--format", output_format.lower()])
        if dry_run:
            args.append("--dry-run")
        if assume_yes:
            args.append("--yes")
        args.append(title_id)
        return self._json(args, cancel_event=cancel_event)["data"]

    def interactive_download(
        self,
        *,
        output_dir: str | os.PathLike[str] | None = None,
        max_chapters: int = 0,
        language: str | None = None,
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Open the TUI and return complete chapters with ordered image paths.

        Terminal streams are inherited. Cancellation and partial failure raise
        MangateError, whose result retains any validated completed chapters.
        """
        if (
            isinstance(max_chapters, bool)
            or not isinstance(max_chapters, int)
            or max_chapters < 0
        ):
            raise ValueError("max_chapters must be a nonnegative integer")
        if not _has_terminal():
            raise MangateError(
                "terminal",
                "interactive_download requires a terminal on stdin and stdout",
            )
        saved_terminal = None
        if os.name != "nt":
            import termios

            saved_terminal = termios.tcgetattr(sys.stdin.fileno())
        try:
            with tempfile.TemporaryDirectory(prefix="mangate-result-") as temporary:
                result_file = Path(temporary) / "result.json"
                args = self._base_args() + [
                    "--provider",
                    self.provider,
                    "--format",
                    "directory",
                ]
                if output_dir is not None:
                    args += ["--download-dir", os.fspath(output_dir)]
                if language:
                    args += ["--language", language]
                args += [
                    "tui",
                    "--exit-on-complete",
                    "--result-file",
                    str(result_file),
                    "--max-chapters",
                    str(max_chapters),
                ]
                try:
                    proc = subprocess.Popen(args)
                except FileNotFoundError:
                    raise MangateError(
                        "executable", f"Mangate executable not found: {self.executable}"
                    ) from None
                except OSError as error:
                    raise MangateError(
                        "executable", f"Cannot start Mangate: {error}"
                    ) from error
                try:
                    code = proc.wait(
                        timeout=self.timeout if timeout is None else timeout
                    )
                except (subprocess.TimeoutExpired, KeyboardInterrupt) as error:
                    _stop_interactive(proc)
                    category = (
                        "timeout"
                        if isinstance(error, subprocess.TimeoutExpired)
                        else "cancelled"
                    )
                    raise MangateError(
                        category, f"interactive download {category}"
                    ) from None
                try:
                    payload = json.loads(result_file.read_text(encoding="utf-8"))
                    _validate_interactive(payload, max_chapters)
                except (OSError, ValueError, TypeError, KeyError) as error:
                    raise MangateError(
                        "invalid_result", f"Invalid interactive result: {error}"
                    ) from error
                status = payload["status"]
                expected_code = {
                    "success": 0,
                    "cancelled": 7,
                    "partial": 5,
                    "error": 10,
                }[status]
                if code != expected_code:
                    raise MangateError(
                        "invalid_result",
                        f"Interactive result disagrees with exit status {code}",
                    )
                if status != "success":
                    raise MangateError(
                        status,
                        payload.get("error") or f"interactive download {status}",
                        result=payload,
                    )
                return payload
        finally:
            if saved_terminal is not None:
                termios.tcsetattr(sys.stdin.fileno(), termios.TCSANOW, saved_terminal)

    def convert(
        self,
        chapter_directory: str | os.PathLike[str],
        *,
        output_format: str = "cbz",
        output: str | os.PathLike[str] | None = None,
        remove_source: bool = False,
        dry_run: bool = False,
        assume_yes: bool = False,
    ) -> dict[str, Any]:
        """Create a CBZ or ZIP archive from an existing chapter directory."""
        if output_format.lower() not in {"cbz", "zip"}:
            raise ValueError("output_format must be cbz or zip")
        args = ["--format", output_format.lower(), "archive", "convert"]
        if output is not None:
            args.extend(["--output", os.fspath(output)])
        if remove_source:
            args.append("--remove-source")
        if dry_run:
            args.append("--dry-run")
        if assume_yes:
            args.append("--yes")
        args.append(os.fspath(chapter_directory))
        return self._json(args)["data"]

    def convert_many(
        self,
        chapter_directories: Iterable[str | os.PathLike[str]],
        *,
        output_format: str = "cbz",
        outputs: Iterable[str | os.PathLike[str]] | None = None,
        remove_source: bool = False,
        dry_run: bool = False,
        assume_yes: bool = False,
    ) -> list[dict[str, Any]]:
        """Convert chapter directories in input order.

        ``outputs`` is optional. When supplied it must contain exactly one
        destination for every source directory; this prevents accidental
        destination reuse across a batch.
        """
        directories = [os.fspath(directory) for directory in chapter_directories]
        destinations = (
            None if outputs is None else [os.fspath(output) for output in outputs]
        )
        if destinations is not None and len(destinations) != len(directories):
            raise ValueError(
                "outputs must contain exactly one path per chapter directory"
            )
        return [
            self.convert(
                directory,
                output_format=output_format,
                output=None if destinations is None else destinations[index],
                remove_source=remove_source,
                dry_run=dry_run,
                assume_yes=assume_yes,
            )
            for index, directory in enumerate(directories)
        ]

    def inspect_archive(self, archive_path: str | os.PathLike[str]) -> dict[str, Any]:
        """Return archive entries, metadata state, and page count without extraction."""
        return self._json(["archive", "inspect", os.fspath(archive_path)])["data"]

    def verify_archive(self, archive_path: str | os.PathLike[str]) -> dict[str, Any]:
        """Verify archive structure, safe entry paths, and completion metadata."""
        return self._json(["archive", "verify", os.fspath(archive_path)])["data"]

    def dataset_plan(
        self,
        *,
        collection_config: str | os.PathLike[str] | None = None,
        resume: bool = False,
        extra_args: Iterable[str] = (),
    ) -> dict[str, Any]:
        args = ["dataset", "plan"]
        if collection_config is not None:
            args.extend(["--collection-config", os.fspath(collection_config)])
        if resume:
            args.append("--resume")
        args.extend(extra_args)
        return self._json(args)["data"]

    def dataset_collect(
        self,
        *,
        collection_config: str | os.PathLike[str] | None = None,
        resume: bool = False,
        dry_run: bool = False,
        assume_yes: bool = True,
        cancel_event: Event | None = None,
        extra_args: Iterable[str] = (),
    ) -> dict[str, Any]:
        args = ["dataset", "collect"]
        if collection_config is not None:
            args.extend(["--collection-config", os.fspath(collection_config)])
        if resume:
            args.append("--resume")
        if dry_run:
            args.append("--dry-run")
        if assume_yes:
            args.append("--yes")
        args.extend(extra_args)
        return self._json(args, cancel_event=cancel_event)["data"]

    def dataset_status(self, dataset_root: str | os.PathLike[str]) -> dict[str, Any]:
        return self._json(["dataset", "status", os.fspath(dataset_root)])["data"]

    def dataset_verify(
        self, dataset_root: str | os.PathLike[str], *, repair: bool = False
    ) -> dict[str, Any]:
        args = ["dataset", "verify", os.fspath(dataset_root)]
        if repair:
            args.append("--repair")
        return self._json(args)["data"]

    def dataset_export(
        self,
        dataset_root: str | os.PathLike[str],
        *,
        split: str | None = None,
        include_duplicates: bool = False,
        include_rejected: bool = False,
    ) -> dict[str, Any]:
        args = ["dataset", "export", os.fspath(dataset_root)]
        if split:
            args.extend(["--split", split])
        if include_duplicates:
            args.append("--include-duplicates")
        if include_rejected:
            args.append("--include-rejected")
        return self._json(args)["data"]

    def _base_args(self) -> list[str]:
        args = [self.executable]
        if self.output_dir:
            args.extend(["--download-dir", self.output_dir])
        if self.page_downloads is not None:
            args.extend(["--page-downloads", str(self.page_downloads)])
        if self.chapter_downloads is not None:
            args.extend(["--chapter-downloads", str(self.chapter_downloads)])
        args.extend(["--existing-files", self.existing_files])
        args.extend(["--format", self.output_format])
        return args

    def _json(
        self,
        args: list[str],
        *,
        cancel_event: Event | None = None,
        progress: Callable[[ProgressEvent], None] | None = None,
    ) -> dict[str, Any]:
        command = self._base_args() + ["--json", "--quiet"] + args
        if progress is None:
            stdout, stderr, code = self._run(command, cancel_event=cancel_event)
        else:
            with tempfile.TemporaryDirectory(prefix="mangate-events-") as temporary:
                events = Path(temporary) / "events.jsonl"
                offset = 0
                pending = b""

                def poll_events() -> None:
                    nonlocal offset, pending
                    if not events.exists():
                        return
                    with events.open("rb") as stream:
                        stream.seek(offset)
                        chunk = stream.read()
                        offset = stream.tell()
                    pending += chunk
                    lines = pending.split(b"\n")
                    pending = lines.pop()
                    for line in lines:
                        try:
                            data = json.loads(line)
                            event = ProgressEvent(
                                stage=data["stage"],
                                status=data["status"],
                                attempt=data.get("attempt", 0),
                                manga_id=data.get("mangaId", ""),
                                chapter_id=data.get("chapterId", ""),
                                completed_pages=data.get("completedPages", 0),
                                expected_pages=data.get("expectedPages", 0),
                                reason=data.get("reason", ""),
                            )
                            if event.stage not in {
                                "discovery",
                                "selection",
                                "validation",
                                "downloading",
                                "completion",
                            }:
                                raise ValueError("unknown progress stage")
                        except (ValueError, KeyError, TypeError) as error:
                            raise MangateError(
                                "invalid_result", f"Invalid progress event: {error}"
                            ) from error
                        progress(event)

                stdout, stderr, code = self._run(
                    command + ["--events-file", str(events)],
                    cancel_event=cancel_event,
                    on_poll=poll_events,
                )
        try:
            payload = json.loads(stdout)
        except json.JSONDecodeError:
            if code != 0:
                raise MangateError(
                    _category(stderr), stderr.strip() or "Mangate failed", stderr
                ) from None
            raise MangateError(
                "internal", "Mangate returned invalid JSON", stderr
            ) from None
        if not isinstance(payload, dict):
            raise MangateError(
                "invalid_result", "Mangate returned a non-object result", stderr
            )
        # A partial download is a usable result with failed chapters recorded in
        # its data. Preserve it for callers instead of discarding completed
        # archive paths because the CLI correctly returned exit status 5.
        if payload.get("status") in {"partial", "no_results"}:
            return payload
        if code != 0 or payload.get("status") != "success":
            error = payload.get("data", {})
            if isinstance(error, dict):
                category = str(
                    error.get("category")
                    or (
                        "cancelled"
                        if payload.get("status") == "cancelled"
                        else _category(stderr)
                    )
                )
                message = str(
                    error.get("message") or error.get("error") or "Mangate failed"
                )
            else:
                category = _category(stderr)
                message = "Mangate failed"
            raise MangateError(category, message, stderr, payload.get("data"))
        return payload

    def _run(
        self,
        command: list[str],
        *,
        cancel_event: Event | None = None,
        on_poll: Callable[[], None] | None = None,
    ) -> tuple[str, str, int]:
        if cancel_event is not None and cancel_event.is_set():
            raise MangateError("cancelled", "operation cancelled")
        # Files avoid waiting for EOF from descendants that inherit output handles.
        with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as diagnostic:
            proc = None
            job = None
            category = None
            creationflags = 0
            try:
                if os.name == "nt":
                    from ._windows import CREATE_SUSPENDED, ProcessJob

                    job = ProcessJob()
                    creationflags = (
                        subprocess.CREATE_NEW_PROCESS_GROUP | CREATE_SUSPENDED
                    )
                proc = subprocess.Popen(
                    command,
                    stdout=output,
                    stderr=diagnostic,
                    start_new_session=os.name != "nt",
                    creationflags=creationflags,
                )
                if job is not None:
                    try:
                        job.start(proc)
                    except OSError:
                        # Assignment failure leaves an unowned, suspended child.
                        proc.kill()
                        try:
                            proc.wait(timeout=2)
                        except subprocess.TimeoutExpired:
                            pass
                        raise
                deadline = (
                    None if self.timeout is None else time.monotonic() + self.timeout
                )
                try:
                    while True:
                        if on_poll is not None:
                            on_poll()
                        if cancel_event is not None and cancel_event.is_set():
                            category = "cancelled"
                        elif deadline is not None and time.monotonic() >= deadline:
                            category = "timeout"
                        if category is not None:
                            break
                        try:
                            proc.wait(timeout=0.1)
                            break
                        except subprocess.TimeoutExpired:
                            continue
                except KeyboardInterrupt:
                    category = "cancelled"
            except OSError as error:
                raise MangateError(
                    "executable", f"Cannot start or manage Mangate: {error}"
                ) from error
            finally:
                try:
                    if proc is not None:
                        if proc.poll() is None:
                            _stop_process(proc, job)
                        _kill_process_tree(proc, job)
                finally:
                    if job is not None:
                        job.close()
            if on_poll is not None:
                on_poll()
            output.seek(0)
            diagnostic.seek(0)
            stdout = output.read().decode("utf-8", errors="replace")
            stderr = diagnostic.read().decode("utf-8", errors="replace")
            stdout = stdout.replace("\r\n", "\n").replace("\r", "\n")
            stderr = stderr.replace("\r\n", "\n").replace("\r", "\n")
            if category is not None:
                try:
                    result = json.loads(stdout).get("data")
                except (ValueError, AttributeError):
                    result = None
                raise MangateError(category, f"operation {category}", stderr, result)
            return stdout, stderr, proc.returncode

    def _text(self, args: list[str]) -> str:
        stdout, stderr, code = self._run(self._base_args() + args)
        if code:
            raise MangateError(
                _category(stderr), stderr.strip() or "Mangate failed", stderr
            )
        return stdout


def _kill_process_tree(proc: subprocess.Popen, job=None) -> None:
    if job is not None:
        job.terminate()
    elif os.name != "nt":
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    elif proc.poll() is None:
        try:
            proc.kill()
        except OSError:
            if proc.poll() is None:
                raise


def _stop_process(proc: subprocess.Popen, job=None) -> None:
    if proc.poll() is None:
        try:
            if os.name == "nt":
                proc.send_signal(signal.CTRL_BREAK_EVENT)
            else:
                os.killpg(proc.pid, signal.SIGINT)
        except (OSError, ValueError):
            try:
                proc.terminate()
            except OSError:
                if proc.poll() is None:
                    raise
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        _kill_process_tree(proc, job)
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            pass


def _category(message: str) -> str:
    lower = message.lower()
    if "unknown provider" in lower:
        return "unknown_provider"
    if "not found" in lower:
        return "not_found"
    if "unsupported" in lower or "does not permit" in lower:
        return "unsupported_operation"
    if "timeout" in lower or "deadline exceeded" in lower:
        return "timeout"
    if "archive" in lower:
        return "archive"
    if "permission" in lower or "create" in lower or "write" in lower:
        return "filesystem"
    if "cancel" in lower or "interrupt" in lower:
        return "cancelled"
    if "select chapters" in lower or "cannot be empty" in lower:
        return "invalid_input"
    if "network" in lower or "connection" in lower or "request" in lower:
        return "network"
    return "internal"


def _has_terminal() -> bool:
    return (
        sys.stdin.isatty()
        and sys.stdout.isatty()
        and os.environ.get("TERM", "").lower() != "dumb"
    )


def _stop_interactive(proc: subprocess.Popen) -> None:
    if proc.poll() is not None:
        return
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait()


def _validate_interactive(payload: Any, max_chapters: int) -> None:
    if not isinstance(payload, dict) or payload.get("formatVersion") != "1":
        raise ValueError("unsupported format version")
    status = payload.get("status")
    if status not in {"success", "cancelled", "partial", "error"}:
        raise ValueError("unknown status")
    root = Path(payload["outputDir"])
    if not root.is_absolute():
        raise ValueError("output directory must be absolute")
    root = root.resolve()
    chapters = payload["chapters"]
    if not isinstance(chapters, list) or (
        max_chapters and len(chapters) > max_chapters
    ):
        raise ValueError("invalid chapter selection")
    if status == "success" and not chapters:
        raise ValueError("successful result has no chapters")
    if chapters and (
        not isinstance(payload.get("title"), dict) or not payload["title"].get("id")
    ):
        raise ValueError("missing title metadata")
    ids = set()
    for item in chapters:
        chapter_id = item["chapter"]["id"]
        if not isinstance(chapter_id, str) or not chapter_id or chapter_id in ids:
            raise ValueError("invalid chapter metadata")
        ids.add(chapter_id)
        chapter_status = item["status"]
        if chapter_status not in {"success", "failed"} or (
            status == "success" and chapter_status != "success"
        ):
            raise ValueError("incomplete successful result")
        directory = Path(item["outputDir"])
        if not directory.is_absolute() or not directory.resolve().is_relative_to(root):
            raise ValueError("chapter path escapes output directory")
        paths = item["imagePaths"]
        if (
            not isinstance(paths, list)
            or (chapter_status == "success" and not paths)
            or (chapter_status == "failed" and paths)
        ):
            raise ValueError("invalid chapter image sequence")
        expected = item["expectedPages"]
        if not isinstance(expected, int) or isinstance(expected, bool) or expected < 0:
            raise ValueError("invalid expected page count")
        if chapter_status == "success" and (expected <= 0 or len(paths) != expected):
            raise ValueError("incomplete image sequence")
        for index, value in enumerate(paths, 1):
            path = Path(value)
            if (
                not path.is_absolute()
                or path.resolve().parent != directory.resolve()
                or not path.is_file()
                or path.stat().st_size == 0
                or not path.stem.isascii()
                or not path.stem.isdecimal()
                or int(path.stem) != index
            ):
                raise ValueError("missing, unsafe, or unordered image path")
