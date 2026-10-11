"""Private CLI argument construction; Go owns acquisition business logic."""

from __future__ import annotations

import os
from collections.abc import Callable, Iterable
from threading import Event
from typing import Any

from .errors import MangateError
from .models import ProgressEvent


def version(runner) -> str:
    return runner._text(["--version"]).strip()


def providers(runner) -> list[dict[str, Any]]:
    return runner._json(["providers"])["data"]


def provider_info(runner, provider: str | None = None) -> dict[str, Any]:
    return runner._json(
        [
            "--provider",
            provider or runner.provider,
            "provider",
            provider or runner.provider,
        ]
    )["data"]


def search(
    runner,
    query: str,
    *,
    provider: str | None = None,
    limit: int | None = None,
    cancel_event: Event | None = None,
) -> list[dict[str, Any]]:
    _text_argument(query, "query")
    _limit(limit)
    args = ["--provider", provider or runner.provider, "search"]
    if limit is not None:
        args.extend(["--limit", str(limit)])
    args.append(query)
    return _records(runner._json(args, cancel_event=cancel_event)["data"], "results")


def title(runner, title_id: str, *, provider: str | None = None) -> dict[str, Any]:
    record = runner._json(
        ["--provider", provider or runner.provider, "title", title_id]
    )["data"]
    if not isinstance(record, dict) or not isinstance(record.get("title"), dict):
        raise MangateError("invalid_result", "Invalid title result")
    return record["title"]


def chapters(
    runner,
    title_id: str,
    *,
    provider: str | None = None,
    limit: int | None = None,
    language: str | None = None,
) -> list[dict[str, Any]]:
    _text_argument(title_id, "title_id")
    _limit(limit)
    args = ["--provider", provider or runner.provider, "chapters"]
    if limit is not None:
        args.extend(["--limit", str(limit)])
    if language:
        args.extend(["--chapter-language", language])
    args.append(title_id)
    return _records(runner._json(args)["data"], "chapters")


def download(
    runner,
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
    assume_yes: bool = False,
    cancel_event: Event | None = None,
    progress: Callable[[ProgressEvent], None] | None = None,
    output: str | os.PathLike[str] | None = None,
) -> dict[str, Any]:
    _text_argument(title_id, "title_id")
    args = ["--provider", provider or runner.provider, "download"]
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
    if assume_yes:
        args.append("--yes")
    args.append(title_id)
    return runner._json(args, cancel_event=cancel_event, progress=progress)["data"]


def _records(data: object, key: str) -> list[dict[str, Any]]:
    if (
        not isinstance(data, dict)
        or not isinstance(data.get(key), list)
        or any(not isinstance(item, dict) for item in data[key])
    ):
        raise MangateError("invalid_result", f"Invalid {key} result")
    return data[key]


def _text_argument(value: str, name: str) -> None:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} must be a nonempty string")


def _limit(value: int | None) -> None:
    if value is not None and (type(value) is not int or value <= 0):
        raise ValueError("limit must be a positive integer")
