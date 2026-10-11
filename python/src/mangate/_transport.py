"""Private executable configuration, JSON transport and process lifecycle."""

from __future__ import annotations

import json
import math
import os
import signal
import subprocess
import tempfile
import time
from collections.abc import Callable
from contextlib import contextmanager
from pathlib import Path
from threading import Event
from typing import Any

from ._binary import resolve_executable
from .errors import MangateError
from .models import ProgressEvent


class Transport:
    """One CLI invocation per operation; no persistent resources."""

    def __init__(
        self,
        executable: str | os.PathLike[str] | None = None,
        *,
        provider: str = "mangadex",
        output_dir: str | os.PathLike[str] | None = None,
        timeout: float | None = None,
        page_downloads: int | None = None,
        chapter_downloads: int | None = None,
        existing_files: str | None = "skip",
        output_format: str | None = "directory",
    ) -> None:
        if timeout is not None and (
            isinstance(timeout, bool)
            or not isinstance(timeout, (int, float))
            or not math.isfinite(timeout)
            or timeout <= 0
        ):
            raise ValueError("timeout must be finite and positive")
        try:
            self.executable = (
                resolve_executable() if executable is None else os.fspath(executable)
            )
        except RuntimeError as error:
            raise MangateError("executable", str(error)) from error
        self.provider = provider.strip().lower()
        self.output_dir = None if output_dir is None else os.fspath(output_dir)
        self.timeout = timeout
        self.page_downloads = page_downloads
        self.chapter_downloads = chapter_downloads
        existing_files = (
            existing_files.strip().lower() if existing_files is not None else None
        )
        output_format = (
            output_format.strip().lower() if output_format is not None else None
        )
        if existing_files is not None and existing_files not in {
            "skip",
            "replace",
            "fail",
        }:
            raise ValueError("existing_files must be skip, replace, or fail")
        if output_format is not None and output_format not in {
            "directory",
            "png",
            "jpeg",
            "cbz",
            "zip",
        }:
            raise ValueError("output_format must be directory, png, jpeg, cbz, or zip")
        self.existing_files = existing_files
        self.output_format = output_format

    def _base_args(self) -> list[str]:
        args = [self.executable]
        if self.output_dir:
            args.extend(["--download-dir", self.output_dir])
        if self.page_downloads is not None:
            args.extend(["--page-downloads", str(self.page_downloads)])
        if self.chapter_downloads is not None:
            args.extend(["--chapter-downloads", str(self.chapter_downloads)])
        if self.existing_files is not None:
            args.extend(["--existing-files", self.existing_files])
        if self.output_format is not None:
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
        if (
            payload.get("formatVersion") != "1"
            or not isinstance(payload.get("operation"), str)
            or not payload["operation"]
            or "data" not in payload
        ):
            raise MangateError(
                "invalid_result", "Invalid Mangate JSON envelope", stderr
            )
        expected_code = {"success": 0, "partial": 5, "no_results": 1}
        # Dataset collection records stopping limits in its state and exits normally.
        if payload["operation"] == "dataset.collect":
            expected_code["partial"] = 0
        if (
            payload.get("status") in expected_code
            and code != expected_code[payload["status"]]
        ):
            raise MangateError(
                "invalid_result", "Mangate result disagrees with exit status", stderr
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
            category = None
            with managed_process(command, stdout=output, stderr=diagnostic) as proc:
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


@contextmanager
def managed_process(command: list[str], *, stdout=None, stderr=None):
    proc = None
    job = None
    creationflags = 0
    try:
        if os.name == "nt":
            from ._windows import CREATE_SUSPENDED, ProcessJob

            job = ProcessJob()
            creationflags = subprocess.CREATE_NEW_PROCESS_GROUP | CREATE_SUSPENDED
        proc = subprocess.Popen(
            command,
            stdout=stdout,
            stderr=stderr,
            start_new_session=os.name != "nt",
            creationflags=creationflags,
        )
        if job is not None:
            try:
                job.start(proc)
            except OSError:
                proc.kill()
                try:
                    proc.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    pass
                raise
        yield proc
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
