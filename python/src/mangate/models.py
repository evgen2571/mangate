"""Immutable Python views of the Go machine-readable contracts."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Literal

AcquisitionStatus = Literal["complete", "partial", "failed", "cancelled"]


@dataclass(frozen=True, slots=True)
class Manga:
    id: str
    title: str
    provider: str
    url: str = ""
    alternative_title: str = ""
    language: str = ""
    status: str = ""
    content_type: str = ""
    year: int = 0


@dataclass(frozen=True, slots=True)
class Chapter:
    id: str
    provider: str
    number: str = ""
    title: str = ""
    volume: str = ""
    language: str = ""
    release_group: str = ""
    published_at: str = ""
    page_count: int = 0
    url: str = ""


@dataclass(frozen=True, slots=True)
class Page:
    index: int
    path: Path


@dataclass(frozen=True, slots=True)
class DownloadResult:
    manga: Manga
    chapter: Chapter
    pages: tuple[Page, ...]
    output: Path
    status: AcquisitionStatus
    expected_pages: int
    error: str = ""

    @property
    def image_paths(self) -> tuple[Path, ...]:
        return tuple(page.path for page in self.pages)

    @property
    def complete(self) -> bool:
        return self.status == "complete"


@dataclass(frozen=True, slots=True)
class DownloadOptions:
    existing_files: Literal["skip", "replace", "fail"] = "skip"
    output_format: Literal["directory", "png", "jpeg"] = "directory"
    language: str | None = None
    page_downloads: int | None = None

    def __post_init__(self) -> None:
        if self.existing_files not in {"skip", "replace", "fail"}:
            raise ValueError("existing_files must be skip, replace, or fail")
        if self.output_format not in {"directory", "png", "jpeg"}:
            raise ValueError("typed downloads require directory, png, or jpeg format")
        if self.page_downloads is not None and (
            isinstance(self.page_downloads, bool)
            or not isinstance(self.page_downloads, int)
            or self.page_downloads <= 0
        ):
            raise ValueError("page_downloads must be a positive integer")
