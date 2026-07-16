"""Structured, non-interactive access to the Mangate executable."""

from __future__ import annotations

import json
import os
import signal
import subprocess
import time
from dataclasses import dataclass
from threading import Event
from typing import Any, Iterable


@dataclass(slots=True)
class MangateError(RuntimeError):
    """A Mangate operation failure with a stable, programmatic category."""

    category: str
    message: str
    stderr: str = ""

    def __str__(self) -> str:
        return self.message


class Client:
    """Run Mangate operations without parsing terminal-oriented output.

    Args:
        executable: Path or command name for a compatible Mangate 0.1 CLI.
        provider: Default provider identifier.
        output_dir: Optional root directory for downloaded chapters.
        timeout: Per-command timeout in seconds.
        page_downloads: Maximum simultaneous page transfers.
        chapter_downloads: Maximum simultaneous chapter transfers.
    """

    def __init__(
        self,
        executable: str | os.PathLike[str] = "mangate",
        *,
        provider: str = "mangadex",
        output_dir: str | os.PathLike[str] | None = None,
        timeout: float | None = None,
        page_downloads: int | None = None,
        chapter_downloads: int | None = None,
        existing_files: str = "skip",
        output_format: str = "directory",
        retain_source: bool = True,
    ) -> None:
        self.executable = os.fspath(executable)
        self.provider = provider
        self.output_dir = None if output_dir is None else os.fspath(output_dir)
        self.timeout = timeout
        self.page_downloads = page_downloads
        self.chapter_downloads = chapter_downloads
        if existing_files not in {"skip", "replace", "fail"}:
            raise ValueError("existing_files must be skip, replace, or fail")
        if output_format.lower() not in {"directory", "cbz", "zip"}:
            raise ValueError("output_format must be directory, cbz, or zip")
        self.existing_files = existing_files
        self.output_format = output_format.lower()
        self.retain_source = retain_source

    def version(self) -> str:
        return self._text(["--version"]).strip()

    def providers(self) -> list[dict[str, Any]]:
        return self._json(["providers"])["data"]

    def provider_info(self, provider: str | None = None) -> dict[str, Any]:
        return self._json(["--provider", provider or self.provider, "provider", provider or self.provider])["data"]

    def search(self, query: str, *, provider: str | None = None, limit: int | None = None) -> list[dict[str, Any]]:
        args = ["--provider", provider or self.provider, "search"]
        if limit is not None:
            args.extend(["--limit", str(limit)])
        args.append(query)
        return self._json(args)["data"]["results"]

    def title(self, title_id: str, *, provider: str | None = None) -> dict[str, Any]:
        return self._json(["--provider", provider or self.provider, "title", title_id])["data"]

    def chapters(self, title_id: str, *, provider: str | None = None, limit: int | None = None) -> list[dict[str, Any]]:
        args = ["--provider", provider or self.provider, "chapters"]
        if limit is not None:
            args.extend(["--limit", str(limit)])
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
        retain_source: bool | None = None,
        cancel_event: Event | None = None,
    ) -> dict[str, Any]:
        args = ["--provider", provider or self.provider, "download"]
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
            if output_format.lower() not in {"directory", "cbz", "zip"}:
                raise ValueError("output_format must be directory, cbz, or zip")
            args.extend(["--format", output_format.lower()])
        if retain_source is not None:
            args.append("--retain-source" if retain_source else "--retain-source=false")
        args.append(title_id)
        return self._json(args, cancel_event=cancel_event)["data"]

    def convert(
        self,
        chapter_directory: str | os.PathLike[str],
        *,
        output_format: str = "cbz",
        output: str | os.PathLike[str] | None = None,
        remove_source: bool = False,
        dry_run: bool = False,
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
        args.append(os.fspath(chapter_directory))
        return self._json(args)["data"]

    def inspect_archive(self, archive_path: str | os.PathLike[str]) -> dict[str, Any]:
        """Return archive entries, metadata state, and page count without extraction."""
        return self._json(["archive", "inspect", os.fspath(archive_path)])["data"]

    def verify_archive(self, archive_path: str | os.PathLike[str]) -> dict[str, Any]:
        """Verify archive structure, safe entry paths, and completion metadata."""
        return self._json(["archive", "verify", os.fspath(archive_path)])["data"]

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
        if not self.retain_source:
            args.append("--retain-source=false")
        return args

    def _json(self, args: list[str], *, cancel_event: Event | None = None) -> dict[str, Any]:
        command = self._base_args() + ["--json", "--quiet"] + args
        proc = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        deadline = None if self.timeout is None else time.monotonic() + self.timeout
        while proc.poll() is None:
            if cancel_event is not None and cancel_event.is_set():
                proc.send_signal(signal.SIGINT)
                stdout, stderr = proc.communicate()
                raise MangateError("cancelled", "operation cancelled", stderr)
            if deadline is not None and time.monotonic() >= deadline:
                proc.kill()
                _, stderr = proc.communicate()
                raise MangateError("timeout", "operation timed out", stderr)
            time.sleep(0.02)
        stdout, stderr = proc.communicate()
        try:
            payload = json.loads(stdout)
        except json.JSONDecodeError:
            if proc.returncode != 0:
                raise MangateError(_category(stderr), stderr.strip() or "Mangate failed", stderr) from None
            raise MangateError("internal", "Mangate returned invalid JSON", stderr) from None
        # A partial download is a usable result with failed chapters recorded in
        # its data. Preserve it for callers instead of discarding completed
        # archive paths because the CLI correctly returned exit status 5.
        if payload.get("status") == "partial":
            return payload
        if proc.returncode != 0 or payload.get("status") != "success":
            error = payload.get("data", {})
            if isinstance(error, dict):
                category = str(error.get("category") or _category(stderr))
                message = str(error.get("message") or "Mangate failed")
            else:
                category = _category(stderr)
                message = "Mangate failed"
            raise MangateError(category, message, stderr)
        return payload

    def _text(self, args: list[str]) -> str:
        try:
            return subprocess.check_output(self._base_args() + args, text=True, stderr=subprocess.PIPE, timeout=self.timeout)
        except subprocess.CalledProcessError as error:
            raise MangateError(_category(error.stderr), error.stderr.strip() or "Mangate failed", error.stderr) from None


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
