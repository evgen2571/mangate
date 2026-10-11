"""Private Bubble Tea terminal and versioned result-file handoff."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

from ._transport import managed_process
from .errors import MangateError


def interactive_download(
    runner,
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
    try:
        if os.name == "nt":
            from ._windows import console_modes

            restore_terminal = console_modes()
        else:
            import termios

            saved_terminal = termios.tcgetattr(sys.stdin.fileno())

            def restore_terminal():
                termios.tcsetattr(sys.stdin.fileno(), termios.TCSANOW, saved_terminal)
    except OSError as error:
        raise MangateError(
            "terminal", f"Cannot capture terminal state: {error}"
        ) from error
    try:
        with tempfile.TemporaryDirectory(prefix="mangate-result-") as temporary:
            result_file = Path(temporary) / "result.json"
            args = runner._base_args() + [
                "--provider",
                runner.provider,
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
            category = None
            with managed_process(args) as proc:
                try:
                    code = proc.wait(
                        timeout=runner.timeout if timeout is None else timeout
                    )
                except (subprocess.TimeoutExpired, KeyboardInterrupt) as error:
                    category = (
                        "timeout"
                        if isinstance(error, subprocess.TimeoutExpired)
                        else "cancelled"
                    )
            # Graceful shutdown can write a partial result before tree cleanup.
            if category is not None:
                payload = None
                if result_file.exists():
                    try:
                        payload = json.loads(result_file.read_text(encoding="utf-8"))
                        _validate_interactive(payload, max_chapters)
                    except (OSError, ValueError, TypeError, KeyError) as error:
                        raise MangateError(
                            "invalid_result", f"Invalid interactive result: {error}"
                        ) from error
                raise MangateError(
                    category, f"interactive download {category}", result=payload
                )
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
        try:
            restore_terminal()
        except OSError as error:
            raise MangateError(
                "terminal", f"Cannot restore terminal state: {error}"
            ) from error


def _has_terminal() -> bool:
    return (
        sys.stdin is not None
        and sys.stdout is not None
        and sys.stdin.isatty()
        and sys.stdout.isatty()
        and os.environ.get("TERM", "").lower() != "dumb"
    )


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
