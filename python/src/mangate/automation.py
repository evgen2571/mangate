"""Bounded discovery, stable planning, and portable selection replay."""

from __future__ import annotations

import json
import os
import tempfile
from collections.abc import Callable
from dataclasses import asdict
from pathlib import Path
from threading import Event
from typing import Any

from ._results import _chapter, _download_result, _manga
from ._transport import Transport
from .errors import DownloadError, MangateError
from .models import (
    AcquisitionManifest,
    DownloadOptions,
    DownloadPlan,
    DownloadResult,
    ManifestImage,
    ProgressEvent,
    SelectionPolicy,
)

_POLICY_KEYS = {
    "provider": "provider",
    "originalLanguage": "original_language",
    "chapterLanguage": "chapter_language",
    "minPages": "min_pages",
    "maxPages": "max_pages",
    "chapterMin": "chapter_min",
    "chapterMax": "chapter_max",
    "maxAttempts": "max_attempts",
    "seed": "seed",
}


def _policy(policy: SelectionPolicy | None, filters: dict[str, Any]) -> SelectionPolicy:
    supplied = {key: value for key, value in filters.items() if value is not None}
    if policy is not None:
        if supplied:
            raise ValueError("use either policy or individual policy fields")
        return policy
    return SelectionPolicy(**supplied)


def _random(
    output: str | os.PathLike[str],
    *,
    planning: bool,
    policy: SelectionPolicy,
    options: DownloadOptions | None,
    executable: str | os.PathLike[str] | None,
    timeout: float | None,
    cancel_event: Event | None,
    progress: Callable[[ProgressEvent], None] | None,
    manifest: str | os.PathLike[str] | None,
) -> DownloadPlan | DownloadResult:
    if not os.fspath(output).strip():
        raise ValueError("output cannot be empty")
    options = options or DownloadOptions()
    if options.language is not None and options.language != policy.chapter_language:
        raise ValueError("options.language must match policy.chapter_language")
    args = ["--provider", policy.provider, "random", "--output", os.fspath(output)]
    for name, key in _POLICY_KEYS.items():
        if name == "provider":
            continue
        value = getattr(policy, key)
        if value is not None and value != "":
            args += ["--" + key.replace("_", "-"), str(value)]
    if planning:
        args.append("--plan")
    if manifest is not None:
        args += ["--manifest", os.fspath(manifest)]
    client = Transport(
        executable,
        provider=policy.provider,
        timeout=timeout,
        existing_files=options.existing_files,
        output_format=options.output_format,
        page_downloads=options.page_downloads,
    )
    try:
        data = client._json(args, cancel_event=cancel_event, progress=progress)["data"]
    except MangateError as error:
        _raise_download(error)
    if planning:
        return _plan(data)
    return _result(data)


def plan_random(
    output: str | os.PathLike[str],
    *,
    policy: SelectionPolicy | None = None,
    provider: str | None = None,
    original_language: str | None = None,
    chapter_language: str | None = None,
    min_pages: int | None = None,
    max_pages: int | None = None,
    chapter_min: float | None = None,
    chapter_max: float | None = None,
    max_attempts: int | None = None,
    seed: int | None = None,
    options: DownloadOptions | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    cancel_event: Event | None = None,
    progress: Callable[[ProgressEvent], None] | None = None,
    manifest: str | os.PathLike[str] | None = None,
) -> DownloadPlan:
    """Select and validate one chapter without downloading images.

    Seeded catalog selection records its bounded candidate snapshot. Provider
    random selection makes no determinism promise. Missing required metadata
    rejects a candidate; missing page counts are resolved from the page list.
    """
    selection = _policy(
        policy,
        dict(
            provider=provider,
            original_language=original_language,
            chapter_language=chapter_language,
            min_pages=min_pages,
            max_pages=max_pages,
            chapter_min=chapter_min,
            chapter_max=chapter_max,
            max_attempts=max_attempts,
            seed=seed,
        ),
    )
    return _random(
        output,
        planning=True,
        policy=selection,
        options=options,
        executable=executable,
        timeout=timeout,
        cancel_event=cancel_event,
        progress=progress,
        manifest=manifest,
    )


def download_random(
    output: str | os.PathLike[str],
    *,
    policy: SelectionPolicy | None = None,
    provider: str | None = None,
    original_language: str | None = None,
    chapter_language: str | None = None,
    min_pages: int | None = None,
    max_pages: int | None = None,
    chapter_min: float | None = None,
    chapter_max: float | None = None,
    max_attempts: int | None = None,
    seed: int | None = None,
    options: DownloadOptions | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    cancel_event: Event | None = None,
    progress: Callable[[ProgressEvent], None] | None = None,
    manifest: str | os.PathLike[str] | None = None,
) -> DownloadResult:
    """Discover and acquire exactly one eligible chapter in one subprocess."""
    selection = _policy(
        policy,
        dict(
            provider=provider,
            original_language=original_language,
            chapter_language=chapter_language,
            min_pages=min_pages,
            max_pages=max_pages,
            chapter_min=chapter_min,
            chapter_max=chapter_max,
            max_attempts=max_attempts,
            seed=seed,
        ),
    )
    return _random(
        output,
        planning=False,
        policy=selection,
        options=options,
        executable=executable,
        timeout=timeout,
        cancel_event=cancel_event,
        progress=progress,
        manifest=manifest,
    )


def execute(
    plan: DownloadPlan | AcquisitionManifest | str | os.PathLike[str],
    *,
    output: str | os.PathLike[str] | None = None,
    options: DownloadOptions | None = None,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    cancel_event: Event | None = None,
    progress: Callable[[ProgressEvent], None] | None = None,
    manifest: str | os.PathLike[str] | None = None,
) -> DownloadResult:
    """Replay the stable selection, validating it anew without random discovery.

    Existing files follow DownloadOptions. Use verify_manifest for local-only
    verification; replay always resolves the original selected chapter.
    """
    source = load_manifest(plan) if isinstance(plan, (str, os.PathLike)) else plan
    source = AcquisitionManifest(source) if isinstance(source, DownloadPlan) else source
    options = options or DownloadOptions(output_format=source.plan.output_format)
    if options.output_format != source.plan.output_format:
        raise ValueError("options.output_format must match the plan")
    if (
        options.language is not None
        and options.language != source.plan.chapter.language
    ):
        raise ValueError("options.language must match the selected chapter")
    if output is not None and not os.fspath(output).strip():
        raise ValueError("output cannot be empty")
    client = Transport(
        executable,
        timeout=timeout,
        existing_files=options.existing_files,
        output_format=options.output_format,
        page_downloads=options.page_downloads,
    )
    with tempfile.TemporaryDirectory(prefix="mangate-plan-") as temporary:
        path = Path(temporary) / "manifest.json"
        save_manifest(source, path)
        args = ["download", "--from-manifest", str(path)]
        if output is not None:
            args += ["--output", os.fspath(output)]
        if manifest is not None:
            args += ["--manifest", os.fspath(manifest)]
        try:
            record = client._json(args, cancel_event=cancel_event, progress=progress)[
                "data"
            ]
        except MangateError as error:
            _raise_download(error)
    return _result(record)


def _raise_download(error: MangateError) -> None:
    result = None
    if isinstance(error.result, dict) and "chapters" in error.result:
        result = _download_result(
            error.result,
            _manifest(error.result["manifest"])
            if error.result.get("manifest") is not None
            else None,
        )
    raise DownloadError(error.category, error.message, error.stderr, result) from error


def _result(record: dict[str, Any]) -> DownloadResult:
    result = _download_result(
        record,
        _manifest(record["manifest"]) if record.get("manifest") is not None else None,
    )
    if not result.complete:
        raise DownloadError(
            result.status, result.error or f"download {result.status}", result=result
        )
    return result


def _plan(data: dict[str, Any], base: Path | None = None) -> DownloadPlan:
    try:
        if (
            set(data)
            - {
                "version",
                "provider",
                "manga",
                "chapter",
                "policy",
                "strategy",
                "candidateIds",
                "expectedPages",
                "output",
                "format",
                "validation",
            }
            or type(data["version"]) is not int
            or data["version"] != 1
            or data["validation"] != "eligible"
        ):
            raise ValueError("unsupported version or validation state")
        policy_data = data["policy"]
        if set(policy_data) - _POLICY_KEYS.keys():
            raise ValueError("unknown policy fields")
        policy = SelectionPolicy(**{_POLICY_KEYS[k]: v for k, v in policy_data.items()})
        provider = data["provider"]
        manga, chapter = (
            _manga(data["manga"], provider),
            _chapter(data["chapter"], provider),
        )
        expected = data["expectedPages"]
        if (
            provider != policy.provider
            or not isinstance(manga.id, str)
            or not manga.id
            or not isinstance(chapter.id, str)
            or not chapter.id
        ):
            raise ValueError("inconsistent identity")
        if (
            isinstance(expected, bool)
            or not isinstance(expected, int)
            or expected < policy.min_pages
            or policy.max_pages is not None
            and expected > policy.max_pages
            or chapter.page_count != expected
        ):
            raise ValueError("inconsistent page count")
        if (
            policy.original_language
            and manga.language != policy.original_language
            or policy.chapter_language
            and chapter.language != policy.chapter_language
        ):
            raise ValueError("metadata does not match policy")
        if policy.chapter_min is not None or policy.chapter_max is not None:
            import math

            number = float(chapter.number)
            if (
                not math.isfinite(number)
                or policy.chapter_min is not None
                and number < policy.chapter_min
                or policy.chapter_max is not None
                and number > policy.chapter_max
            ):
                raise ValueError("chapter number does not match policy")
        strategy = data["strategy"]
        raw_ids = data.get("candidateIds", [])
        if not isinstance(raw_ids, list) or any(
            not isinstance(item, str) or not item for item in raw_ids
        ):
            raise ValueError("invalid candidate IDs")
        ids = tuple(raw_ids)
        if strategy == "provider_random":
            if policy.seed is not None or ids:
                raise ValueError("provider random cannot have a seed/catalog")
        elif strategy == "seeded_catalog":
            if (
                policy.seed is None
                or not ids
                or manga.id not in ids
                or len(ids) != len(set(ids))
                or len(ids) > policy.max_attempts
            ):
                raise ValueError("invalid seeded candidate set")
        else:
            raise ValueError("unknown strategy")
        if (
            data["format"] not in {"directory", "png", "jpeg"}
            or not data["output"].strip()
        ):
            raise ValueError("unsupported format or empty output")
        if (
            data["manga"].get("chapters")
            or data["chapter"].get("pages")
            or data["manga"].get("url")
            or data["chapter"].get("url")
            or data["manga"].get("cover", {}).get("url")
        ):
            raise ValueError("embedded chapters or page URLs")
        output = Path(data["output"])
        if not output.is_absolute():
            if base is None:
                raise ValueError("plan output must be absolute")
            output = base / output
        return DownloadPlan(
            manga,
            chapter,
            policy,
            strategy,
            expected,
            output.resolve(),
            data["format"],
            ids,
        )
    except (KeyError, TypeError, ValueError, AttributeError) as error:
        raise MangateError("invalid_result", f"Invalid plan: {error}") from error


def _manifest(data: dict[str, Any], base: Path | None = None) -> AcquisitionManifest:
    try:
        if (
            type(data["version"]) is not int
            or data["version"] not in {1, 2}
            or set(data)
            - {
                "version",
                "plan",
                "images",
                "actualPages",
                "status",
                "validation",
                "error",
            }
        ):
            raise ValueError("unsupported version or fields")
        plan = _plan(data["plan"], base)
        images = []
        previous = 0
        for item in data["images"]:
            path = Path(item["path"])
            index = item["index"]
            if (
                isinstance(index, bool)
                or not isinstance(index, int)
                or not previous < index <= plan.expected_pages
                or path.name != item["path"]
                or "\\" in item["path"]
                or "\0" in item["path"]
                or int(path.stem) != index
                or item["format"] not in {"png", "jpeg", "gif", "webp", "bmp", "avif"}
                and not (
                    item["format"] == "unknown" and data["validation"] == "incomplete"
                )
            ):
                raise ValueError("unsafe or unordered image")
            size, digest = item.get("bytes", 0), item.get("sha256", "")
            if set(item) - {"index", "path", "format", "bytes", "sha256"}:
                raise ValueError("unknown image fields")
            if (
                type(size) is not int
                or not 0 <= size < 2**63
                or not isinstance(digest, str)
            ):
                raise ValueError("invalid image integrity types")
            if data["version"] == 1:
                if size != 0 or digest != "":
                    raise ValueError("version 1 cannot contain integrity records")
            elif not (
                item["format"] == "unknown"
                and data["validation"] == "incomplete"
                and size == 0
                and digest == ""
            ):
                if (
                    size <= 0
                    or len(digest) != 64
                    or any(c not in "0123456789abcdef" for c in digest)
                ):
                    raise ValueError("invalid image integrity record")
            images.append(ManifestImage(index, path, item["format"], size, digest))
            previous = index
        if type(data["actualPages"]) is not int or data["actualPages"] != len(images):
            raise ValueError("inconsistent actual pages")
        status, validation, error = (
            data["status"],
            data["validation"],
            data.get("error", ""),
        )
        if status == "complete":
            if len(images) != plan.expected_pages or validation != "complete" or error:
                raise ValueError("inconsistent completion")
        elif status == "planned":
            if images or validation != "eligible" or error:
                raise ValueError("inconsistent plan")
        elif status in {"failed", "partial", "cancelled"}:
            if (
                validation != "incomplete"
                or not error
                or status == "partial"
                and not images
                or status == "failed"
                and images
            ):
                raise ValueError("inconsistent failure")
        else:
            raise ValueError("unknown terminal status")
        return AcquisitionManifest(
            plan, tuple(images), status, validation, error, data["version"]
        )
    except (KeyError, TypeError, ValueError, AttributeError) as error:
        raise MangateError("invalid_result", f"Invalid manifest: {error}") from error


def load_manifest(path: str | os.PathLike[str]) -> AcquisitionManifest:
    """Load selection and relative image paths; this does not verify local files."""
    path = Path(path).resolve()
    try:
        if path.stat().st_size > 4 << 20:
            raise ValueError("manifest exceeds 4 MiB")
        return _manifest(json.loads(path.read_text(encoding="utf-8")), path.parent)
    except (OSError, ValueError) as error:
        raise MangateError("invalid_input", f"Cannot load manifest: {error}") from error


def _plan_data(plan: DownloadPlan) -> dict[str, Any]:
    manga = plan.manga
    chapter = plan.chapter
    if (
        manga.provider != plan.policy.provider
        or chapter.provider != plan.policy.provider
    ):
        raise ValueError("plan manga/chapter provider must match policy")
    return dict(
        version=plan.version,
        provider=plan.policy.provider,
        manga=dict(
            id=manga.id,
            title=manga.title,
            metadata=dict(
                language=manga.language,
                status=manga.status,
                alternativeTitle=manga.alternative_title,
                contentType=manga.content_type,
                year=manga.year,
            ),
        ),
        chapter=dict(
            id=chapter.id,
            number=chapter.number,
            title=chapter.title,
            volume=chapter.volume,
            language=chapter.language,
            releaseGroup=chapter.release_group,
            publishedAt=chapter.published_at,
            pageCount=chapter.page_count,
        ),
        policy={
            k: getattr(plan.policy, v)
            for k, v in _POLICY_KEYS.items()
            if getattr(plan.policy, v) is not None
        },
        strategy=plan.strategy,
        candidateIds=list(plan.candidate_ids),
        expectedPages=plan.expected_pages,
        output=str(plan.output),
        format=plan.output_format,
        validation=plan.validation,
    )


def save_manifest(
    source: AcquisitionManifest | DownloadPlan, path: str | os.PathLike[str]
) -> None:
    """Atomically save a plan or result manifest, relative to its file location."""
    source = AcquisitionManifest(source) if isinstance(source, DownloadPlan) else source
    destination = Path(path).resolve()
    if destination.is_relative_to(source.plan.output.resolve()):
        raise ValueError("save manifest outside the chapter output directory")
    data = dict(
        version=source.version,
        plan=_plan_data(source.plan),
        images=[
            {
                key: value
                for key, value in dict(asdict(item), path=item.path.as_posix()).items()
                if source.version != 1 or key not in {"bytes", "sha256"}
            }
            for item in source.images
        ],
        actualPages=source.actual_pages,
        status=source.status,
        validation=source.validation,
        error=source.error,
    )
    _manifest(data)
    data["plan"]["output"] = Path(
        os.path.relpath(source.plan.output, destination.parent)
    ).as_posix()
    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", dir=destination.parent, delete=False
    ) as stream:
        temporary = Path(stream.name)
        try:
            json.dump(data, stream, indent=2)
            stream.write("\n")
            stream.close()
            os.replace(temporary, destination)
        finally:
            temporary.unlink(missing_ok=True)


def verify_manifest(
    path: str | os.PathLike[str],
    *,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> AcquisitionManifest:
    """Verify existing downloaded files locally without rediscovering or replaying."""
    source = load_manifest(path)
    Transport(executable, timeout=timeout)._json(
        ["download", "--from-manifest", os.fspath(path), "--verify-manifest"]
    )
    return source
