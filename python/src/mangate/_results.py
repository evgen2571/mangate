"""Private validation of provider metadata and acquisition result contracts."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from .errors import MangateError
from .models import (
    AcquisitionManifest,
    AcquisitionStatus,
    Chapter,
    DownloadResult,
    InteractiveResult,
    Manga,
    Page,
    ProviderInfo,
)


def _manga(data: dict[str, Any], provider: str) -> Manga:
    from ._decode import decode

    if not isinstance(data, dict) or not isinstance(data.get("metadata", {}), dict):
        raise MangateError("invalid_result", "Invalid manga metadata")
    return decode(
        Manga,
        {
            **data.get("metadata", {}),
            "id": data.get("id"),
            "title": data.get("title"),
            "provider": provider,
            "url": data.get("url", ""),
        },
    )


def _chapter(data: dict[str, Any], provider: str) -> Chapter:
    from ._decode import decode

    if not isinstance(data, dict):
        raise MangateError("invalid_result", "Invalid chapter metadata")
    return decode(Chapter, {**data, "provider": provider})


def _download_result(
    record: dict[str, Any], manifest: AcquisitionManifest | None = None
) -> DownloadResult:
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
            if not path.stem.isascii() or not path.stem.isdecimal():
                raise ValueError("invalid page filename")
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
            manifest,
        )
    except (KeyError, TypeError, ValueError, OSError) as error:
        raise MangateError(
            "invalid_result", f"Invalid download result: {error}"
        ) from error


def _interactive_result(payload: dict[str, Any], provider: str) -> InteractiveResult:
    statuses = {
        "success": "complete",
        "partial": "partial",
        "error": "failed",
        "cancelled": "cancelled",
    }
    downloads = []
    for item in payload["chapters"]:
        status = "complete" if item["status"] == "success" else "failed"
        record = {
            "provider": provider,
            "title": payload["title"],
            "status": status,
            "error": item.get("error", ""),
            "chapters": [
                {
                    "id": item["chapter"]["id"],
                    "chapter": item["chapter"],
                    "outputPath": item["outputDir"],
                    "expectedPages": item["expectedPages"],
                    "imagePaths": item["imagePaths"],
                    "status": status,
                }
            ],
        }
        downloads.append(_download_result(record))
    return InteractiveResult(
        tuple(downloads), statuses[payload["status"]], payload.get("error", "")
    )


def _batch_result(record: dict[str, Any]) -> InteractiveResult:
    try:
        if (
            not isinstance(record, dict)
            or record.get("status")
            not in {"complete", "partial", "failed", "cancelled"}
            or not isinstance(record.get("chapters"), list)
        ):
            raise ValueError("invalid chapter batch")
        if record["status"] == "complete" and not record["chapters"]:
            raise ValueError("complete acquisition has no chapters")
        downloads = []
        for chapter in record["chapters"]:
            status = (
                "complete"
                if chapter["status"] in {"complete", "skipped"}
                else record["status"]
            )
            downloads.append(
                _download_result({**record, "status": status, "chapters": [chapter]})
            )
        return InteractiveResult(
            tuple(downloads), record["status"], record.get("error", "")
        )
    except (KeyError, TypeError, ValueError) as error:
        raise MangateError(
            "invalid_result", f"Invalid chapter batch: {error}"
        ) from error


def _provider(record: dict[str, Any]) -> ProviderInfo:
    from ._decode import decode

    if (
        not isinstance(record, dict)
        or not isinstance(record.get("info"), dict)
        or type(record.get("usable")) is not bool
    ):
        raise MangateError("invalid_result", "Invalid provider metadata")
    return decode(
        ProviderInfo,
        {
            **record["info"],
            "usable": record["usable"],
            "error": record.get("error", ""),
        },
    )
