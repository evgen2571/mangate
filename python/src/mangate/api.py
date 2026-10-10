"""Function-first chapter acquisition using the bundled executable."""

from __future__ import annotations

import os
from pathlib import Path
from threading import Event
from typing import Any

from .client import Client, MangateError
from .models import (
    AcquisitionStatus,
    Chapter,
    DownloadOptions,
    DownloadPlan,
    DownloadResult,
    Manga,
    Page,
)


class DownloadError(MangateError):
    """An unsuccessful acquisition; result holds any typed partial output."""

    result: DownloadResult | None


class Session:
    """Reusable executable, provider, output-root, and timeout defaults.

    No persistent process or server is started. Each operation invokes the CLI.
    """

    def __init__(
        self,
        executable: str | os.PathLike[str] | None = None,
        *,
        provider: str = "mangadex",
        timeout: float | None = None,
        output_dir: str | os.PathLike[str] | None = None,
    ) -> None:
        self._client = Client(
            executable=executable,
            provider=provider,
            timeout=timeout,
            output_dir=output_dir,
        )

    def search(self, query: str, *, limit: int | None = None) -> list[Manga]:
        return [
            _manga(item, self._client.provider)
            for item in self._client.search(query, limit=limit)
        ]

    def chapters(
        self,
        manga: Manga | str,
        *,
        language: str | None = None,
        limit: int | None = None,
    ) -> list[Chapter]:
        provider = manga.provider if isinstance(manga, Manga) else self._client.provider
        title_id = manga.id if isinstance(manga, Manga) else manga
        return [
            _chapter(item, provider)
            for item in self._client.chapters(
                title_id, provider=provider, language=language, limit=limit
            )
        ]

    def plan_random(self, output=None, **kwargs) -> DownloadPlan:
        from .automation import plan_random

        return plan_random(**self._automation_defaults(output, kwargs))

    def download_random(self, output=None, **kwargs) -> DownloadResult:
        from .automation import download_random

        return download_random(**self._automation_defaults(output, kwargs))

    def execute(self, plan, **kwargs) -> DownloadResult:
        from .automation import execute

        kwargs.setdefault("executable", self._client.executable)
        kwargs.setdefault("timeout", self._client.timeout)
        return execute(plan, **kwargs)

    def _automation_defaults(self, output, kwargs):
        kwargs.setdefault("executable", self._client.executable)
        kwargs.setdefault("timeout", self._client.timeout)
        if kwargs.get("policy") is None:
            kwargs.setdefault("provider", self._client.provider)
        kwargs["output"] = output if output is not None else self._client.output_dir
        if kwargs["output"] is None:
            raise ValueError("output is required")
        return kwargs

    def download(
        self,
        title: Manga | str | None = None,
        chapter: Chapter | str | int | float | None = None,
        output: str | os.PathLike[str] | None = None,
        *,
        title_id: str | None = None,
        chapter_id: str | None = None,
        options: DownloadOptions | None = None,
        cancel_event: Event | None = None,
    ) -> DownloadResult:
        """Download one explicit chapter; output is its exact directory.

        A title string must uniquely match a search result. Stable IDs avoid
        name and release ambiguity. Partial operations raise DownloadError.
        """
        if cancel_event is not None and cancel_event.is_set():
            raise DownloadError("cancelled", "operation cancelled")
        if (title is None) == (title_id is None):
            raise ValueError("provide exactly one of title or title_id")
        if (chapter is None) == (chapter_id is None):
            raise ValueError("provide exactly one of chapter or chapter_id")
        if output is not None and not os.fspath(output).strip():
            raise ValueError("output cannot be empty")
        provider = title.provider if isinstance(title, Manga) else self._client.provider
        if isinstance(chapter, Chapter):
            if chapter.provider != provider:
                raise ValueError("chapter and manga providers must match")
            chapter_id = chapter.id
        if isinstance(title, Manga):
            title_id = title.id
        elif title is not None:
            try:
                results = self._client.search(title, cancel_event=cancel_event)
            except MangateError as error:
                raise DownloadError(
                    error.category, error.message, error.stderr
                ) from error
            matches = [
                _manga(item, provider)
                for item in results
                if item["title"].casefold() == title.casefold()
            ]
            if not matches:
                raise DownloadError(
                    "not_found", f"title {title!r} was not found; use title_id"
                )
            if len(matches) != 1:
                raise DownloadError(
                    "invalid_input", f"title {title!r} is ambiguous; use title_id"
                )
            title_id = matches[0].id
        options = options or DownloadOptions()
        client = Client(
            executable=self._client.executable,
            provider=provider,
            timeout=self._client.timeout,
            output_dir=self._client.output_dir,
            existing_files=options.existing_files,
            output_format=options.output_format,
            page_downloads=options.page_downloads,
        )
        try:
            record = client.download(
                title_id,
                chapter_ids=() if chapter_id is None else (chapter_id,),
                chapter_numbers=() if chapter_id is not None else (str(chapter),),
                language=options.language,
                output=output,
                assume_yes=options.existing_files == "replace",
                cancel_event=cancel_event,
            )
        except MangateError as error:
            result = None
            if isinstance(error.result, dict) and "chapters" in error.result:
                result = _download_result(error.result)
            raise DownloadError(
                error.category, error.message, error.stderr, result
            ) from error
        result = _download_result(record)
        if not result.complete:
            raise DownloadError(
                result.status,
                result.error or f"download {result.status}",
                result=result,
            )
        return result


def search(
    query: str,
    *,
    provider: str = "mangadex",
    limit: int | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> list[Manga]:
    return Session(executable, provider=provider, timeout=timeout).search(
        query, limit=limit
    )


def chapters(
    manga: Manga | str,
    *,
    provider: str = "mangadex",
    language: str | None = None,
    limit: int | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> list[Chapter]:
    return Session(executable, provider=provider, timeout=timeout).chapters(
        manga, language=language, limit=limit
    )


def download(
    title: Manga | str | None = None,
    chapter: Chapter | str | int | float | None = None,
    output: str | os.PathLike[str] | None = None,
    *,
    title_id: str | None = None,
    chapter_id: str | None = None,
    provider: str = "mangadex",
    options: DownloadOptions | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    cancel_event: Event | None = None,
) -> DownloadResult:
    return Session(executable, provider=provider, timeout=timeout).download(
        title,
        chapter,
        output,
        title_id=title_id,
        chapter_id=chapter_id,
        options=options,
        cancel_event=cancel_event,
    )


def _manga(data: dict[str, Any], provider: str) -> Manga:
    try:
        metadata = data.get("metadata") or {}
        return Manga(
            id=data["id"],
            title=data["title"],
            provider=provider,
            url=data.get("url", ""),
            alternative_title=metadata.get("alternativeTitle", ""),
            language=metadata.get("language", ""),
            status=metadata.get("status", ""),
            content_type=metadata.get("contentType", ""),
            year=metadata.get("year", 0),
        )
    except (KeyError, TypeError, AttributeError) as error:
        raise MangateError(
            "invalid_result", f"Invalid manga result: {error}"
        ) from error


def _chapter(data: dict[str, Any], provider: str) -> Chapter:
    try:
        return Chapter(
            id=data["id"],
            provider=provider,
            number=data.get("number", ""),
            title=data.get("title", ""),
            volume=data.get("volume", ""),
            language=data.get("language", ""),
            release_group=data.get("releaseGroup", ""),
            published_at=data.get("publishedAt", ""),
            page_count=data.get("pageCount", 0),
            url=data.get("url", ""),
        )
    except (KeyError, TypeError, AttributeError) as error:
        raise MangateError(
            "invalid_result", f"Invalid chapter result: {error}"
        ) from error


def _download_result(record: dict[str, Any]) -> DownloadResult:
    try:
        if len(record["chapters"]) != 1:
            raise ValueError("expected one chapter")
        item = record["chapters"][0]
        provider = record["provider"]
        manga = _manga(record["title"], provider)
        chapter = _chapter(item["chapter"], provider)
        if chapter.id != item["id"]:
            raise ValueError("chapter identity disagrees with result")
        output = Path(item["outputPath"])
        if not output.is_absolute():
            raise ValueError("output path must be absolute")
        expected = item.get("expectedPages", 0)
        if isinstance(expected, bool) or not isinstance(expected, int) or expected < 0:
            raise ValueError("invalid expected page count")
        pages = []
        for raw_path in item["imagePaths"]:
            path = Path(raw_path)
            if not path.is_absolute() or path.resolve().parent != output.resolve():
                raise ValueError("image path escapes chapter directory")
            if path.is_symlink() or not path.is_file() or path.stat().st_size == 0:
                raise ValueError("missing or empty image")
            index = int(path.stem)
            if index < 1 or index > expected or (pages and index <= pages[-1].index):
                raise ValueError("invalid page ordering")
            pages.append(Page(index=index, path=path))
        raw_status = record["status"]
        if raw_status not in {"complete", "partial", "failed", "cancelled"}:
            raise ValueError("unknown acquisition status")
        if item["status"] not in {
            "complete",
            "skipped",
            "incomplete",
            "failed",
            "partial",
            "cancelled",
        }:
            raise ValueError("unknown chapter status")
        status: AcquisitionStatus
        if raw_status == "cancelled":
            status = "cancelled"
        elif raw_status == "complete":
            if item["status"] not in {"complete", "skipped"}:
                raise ValueError("complete result has an unsuccessful chapter")
            if expected <= 0 or len(pages) != expected:
                raise ValueError("complete download has missing pages")
            status = "complete"
        elif raw_status == "failed":
            status = "failed"
        elif pages:
            status = "partial"
        else:
            status = "failed"
        return DownloadResult(
            manga,
            chapter,
            tuple(pages),
            output,
            status,
            expected,
            record.get("error", ""),
            _result_manifest(record),
        )
    except (KeyError, TypeError, ValueError, OSError) as error:
        raise MangateError(
            "invalid_result", f"Invalid download result: {error}"
        ) from error


def _result_manifest(record):
    if record.get("manifest") is None:
        return None
    from .automation import _manifest

    return _manifest(record["manifest"])
