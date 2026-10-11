"""Function-first chapter acquisition using the bundled executable."""

from __future__ import annotations

import math
import os
from collections.abc import Callable
from threading import Event
from typing import Any

from . import _commands
from ._results import (
    _batch_result,
    _chapter,
    _download_result,
    _interactive_result,
    _manga,
    _provider,
)
from ._transport import Transport
from .errors import DownloadError, MangateError
from .models import (
    AcquisitionManifest,
    Chapter,
    DownloadOptions,
    DownloadPlan,
    DownloadResult,
    InteractiveResult,
    Manga,
    ProgressEvent,
    ProviderInfo,
)


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
        self._transport = Transport(
            executable=executable,
            provider=provider,
            timeout=timeout,
            output_dir=output_dir,
        )

    def search(self, query: str, *, limit: int | None = None) -> list[Manga]:
        return [
            _manga(item, self._transport.provider)
            for item in _commands.search(self._transport, query, limit=limit)
        ]

    def chapters(
        self,
        manga: Manga | str,
        *,
        language: str | None = None,
        limit: int | None = None,
    ) -> list[Chapter]:
        provider = (
            manga.provider if isinstance(manga, Manga) else self._transport.provider
        )
        title_id = manga.id if isinstance(manga, Manga) else manga
        return [
            _chapter(item, provider)
            for item in _commands.chapters(
                self._transport,
                title_id,
                provider=provider,
                language=language,
                limit=limit,
            )
        ]

    def plan_random(
        self, output: str | os.PathLike[str] | None = None, **kwargs: Any
    ) -> DownloadPlan:
        from .automation import plan_random

        return plan_random(**self._automation_defaults(output, kwargs))

    def download_random(
        self, output: str | os.PathLike[str] | None = None, **kwargs: Any
    ) -> DownloadResult:
        from .automation import download_random

        return download_random(**self._automation_defaults(output, kwargs))

    def execute(
        self,
        plan: DownloadPlan | AcquisitionManifest | str | os.PathLike[str],
        **kwargs: Any,
    ) -> DownloadResult:
        from .automation import execute

        kwargs.setdefault("executable", self._transport.executable)
        kwargs.setdefault("timeout", self._transport.timeout)
        return execute(plan, **kwargs)

    def _automation_defaults(self, output, kwargs):
        kwargs.setdefault("executable", self._transport.executable)
        kwargs.setdefault("timeout", self._transport.timeout)
        if kwargs.get("policy") is None:
            kwargs.setdefault("provider", self._transport.provider)
        kwargs["output"] = output if output is not None else self._transport.output_dir
        if kwargs["output"] is None:
            raise ValueError("output is required")
        return kwargs

    def interactive_download(
        self,
        *,
        max_chapters: int = 0,
        language: str | None = None,
        output_dir: str | os.PathLike[str] | None = None,
        timeout: float | None = None,
    ) -> InteractiveResult:
        return interactive_download(
            executable=self._transport.executable,
            provider=self._transport.provider,
            output_dir=self._transport.output_dir if output_dir is None else output_dir,
            timeout=self._transport.timeout if timeout is None else timeout,
            max_chapters=max_chapters,
            language=language,
        )

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
        progress: Callable[[ProgressEvent], None] | None = None,
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
        if title is not None and not isinstance(title, (Manga, str)):
            raise ValueError("title must be a Manga or title string")
        if chapter is not None and (
            isinstance(chapter, bool)
            or not isinstance(chapter, (Chapter, str, int, float))
            or isinstance(chapter, float)
            and not math.isfinite(chapter)
        ):
            raise ValueError("chapter must be a Chapter or chapter number")
        for name, value in (("title_id", title_id), ("chapter_id", chapter_id)):
            if value is not None:
                _commands._text_argument(value, name)
        if output is not None and not os.fspath(output).strip():
            raise ValueError("output cannot be empty")
        provider = (
            title.provider if isinstance(title, Manga) else self._transport.provider
        )
        if isinstance(chapter, Chapter):
            if chapter.provider != provider:
                raise ValueError("chapter and manga providers must match")
            chapter_id = chapter.id
        if isinstance(title, Manga):
            title_id = title.id
        elif title is not None:
            try:
                results = _commands.search(
                    self._transport, title, cancel_event=cancel_event
                )
            except MangateError as error:
                raise DownloadError(
                    error.category, error.message, error.stderr
                ) from error
            matches = [
                item
                for item in (_manga(raw, provider) for raw in results)
                if item.title.casefold() == title.casefold()
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
        client = Transport(
            executable=self._transport.executable,
            provider=provider,
            timeout=self._transport.timeout,
            output_dir=self._transport.output_dir,
            existing_files=options.existing_files,
            output_format=options.output_format,
            page_downloads=options.page_downloads,
        )
        try:
            record = _commands.download(
                client,
                title_id,
                chapter_ids=() if chapter_id is None else (chapter_id,),
                chapter_numbers=() if chapter_id is not None else (str(chapter),),
                language=options.language,
                output=output,
                assume_yes=options.existing_files == "replace",
                cancel_event=cancel_event,
                progress=progress,
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


def title(
    title_id: str,
    *,
    provider: str = "mangadex",
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> Manga:
    """Look up a title by stable ID without title matching."""
    _commands._text_argument(title_id, "title_id")
    runner = Transport(executable, provider=provider, timeout=timeout)
    return _manga(_commands.title(runner, title_id), runner.provider)


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
    progress: Callable[[ProgressEvent], None] | None = None,
) -> DownloadResult:
    return Session(executable, provider=provider, timeout=timeout).download(
        title,
        chapter,
        output,
        title_id=title_id,
        chapter_id=chapter_id,
        options=options,
        cancel_event=cancel_event,
        progress=progress,
    )


def interactive_download(
    *,
    output_dir: str | os.PathLike[str] | None = None,
    max_chapters: int = 0,
    language: str | None = None,
    provider: str = "mangadex",
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> InteractiveResult:
    """Run the existing Go TUI in the caller's terminal.

    Cancellation and failed acquisition raise DownloadError with typed results
    when the TUI has written a valid handoff. Terminal errors use MangateError.
    """
    from ._terminal import interactive_download as run_interactive

    runner = Transport(
        executable, provider=provider, timeout=timeout, output_dir=output_dir
    )
    try:
        payload = run_interactive(runner, max_chapters=max_chapters, language=language)
    except MangateError as error:
        result = (
            _interactive_result(error.result, runner.provider)
            if isinstance(error.result, dict)
            else None
        )
        if error.category in {"cancelled", "timeout", "partial", "error"}:
            raise DownloadError(
                "failed" if error.category == "error" else error.category,
                error.message,
                error.stderr,
                result,
            ) from error
        raise
    return _interactive_result(payload, runner.provider)


def download_many(
    title_id: str,
    *,
    chapter_ids: tuple[str, ...] = (),
    chapter_numbers: tuple[str, ...] = (),
    chapter_range: str | None = None,
    first: bool = False,
    latest: bool = False,
    all_chapters: bool = False,
    assume_yes: bool = False,
    output_dir: str | os.PathLike[str] | None = None,
    provider: str = "mangadex",
    options: DownloadOptions | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    cancel_event: Event | None = None,
    progress: Callable[[ProgressEvent], None] | None = None,
) -> InteractiveResult:
    """Download several explicitly selected chapters into the CLI hierarchy."""
    options = options or DownloadOptions()
    runner = Transport(
        executable,
        provider=provider,
        timeout=timeout,
        output_dir=output_dir,
        existing_files=options.existing_files,
        output_format=options.output_format,
        page_downloads=options.page_downloads,
    )
    try:
        record = _commands.download(
            runner,
            title_id,
            chapter_ids=chapter_ids,
            chapter_numbers=chapter_numbers,
            chapter_range=chapter_range,
            first=first,
            latest=latest,
            all_chapters=all_chapters,
            language=options.language,
            assume_yes=assume_yes or options.existing_files == "replace",
            cancel_event=cancel_event,
            progress=progress,
        )
    except MangateError as error:
        result = (
            _batch_result(error.result)
            if isinstance(error.result, dict) and "chapters" in error.result
            else None
        )
        raise DownloadError(
            error.category, error.message, error.stderr, result
        ) from error
    result = _batch_result(record)
    if result.status != "complete":
        raise DownloadError(
            result.status, result.error or f"download {result.status}", result=result
        )
    return result


def cli_version(
    *, executable: str | os.PathLike[str] | None = None, timeout: float | None = None
) -> str:
    return _commands.version(Transport(executable, timeout=timeout))


def providers(
    *, executable: str | os.PathLike[str] | None = None, timeout: float | None = None
) -> tuple[ProviderInfo, ...]:
    return tuple(
        _provider(item)
        for item in _commands.providers(Transport(executable, timeout=timeout))
    )


def provider_info(
    provider: str = "mangadex",
    *,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> ProviderInfo:
    return _provider(
        _commands.provider_info(
            Transport(executable, provider=provider, timeout=timeout)
        )
    )
