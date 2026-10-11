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

    def __post_init__(self) -> None:
        if any(
            not isinstance(value, str) or not value.strip()
            for value in (self.id, self.provider)
        ) or not isinstance(self.title, str):
            raise ValueError("manga requires string identity, provider, and title")


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

    def __post_init__(self) -> None:
        if any(
            not isinstance(value, str) or not value.strip()
            for value in (self.id, self.provider)
        ):
            raise ValueError("chapter requires string identity and provider")
        if type(self.page_count) is not int or self.page_count < 0:
            raise ValueError("page_count must be a nonnegative integer")


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
    manifest: AcquisitionManifest | None = None

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


@dataclass(frozen=True, slots=True)
class SelectionPolicy:
    provider: str = "mangadex"
    original_language: str = ""
    chapter_language: str = ""
    min_pages: int = 1
    max_pages: int | None = None
    chapter_min: float | None = None
    chapter_max: float | None = None
    max_attempts: int = 20
    seed: int | None = None

    def __post_init__(self) -> None:
        import math
        import re

        if (
            not isinstance(self.provider, str)
            or not self.provider.strip()
            or self.provider != self.provider.strip()
        ):
            raise ValueError("provider must be a nonempty unpadded string")
        for name, value in (
            ("min_pages", self.min_pages),
            ("max_attempts", self.max_attempts),
            ("max_pages", self.max_pages),
        ):
            if (value is not None or name != "max_pages") and (
                isinstance(value, bool) or not isinstance(value, int) or value < 1
            ):
                raise ValueError(f"{name} must be a positive integer")
        if self.max_pages is not None and self.max_pages < self.min_pages:
            raise ValueError("max_pages must be >= min_pages")
        for language in (self.original_language, self.chapter_language):
            if not isinstance(language, str) or not re.fullmatch(r"[a-z-]*", language):
                raise ValueError("language must be a lowercase language code")
        for value in (self.chapter_min, self.chapter_max):
            if value is not None and (
                isinstance(value, bool)
                or not isinstance(value, (int, float))
                or not math.isfinite(value)
                or value < 0
            ):
                raise ValueError("chapter bounds must be finite and nonnegative")
        if (
            self.chapter_min is not None
            and self.chapter_max is not None
            and self.chapter_min > self.chapter_max
        ):
            raise ValueError("chapter_min must be <= chapter_max")
        if self.seed is not None and (
            isinstance(self.seed, bool)
            or not isinstance(self.seed, int)
            or not -(2**63) <= self.seed < 2**63
        ):
            raise ValueError("seed must be a signed 64-bit integer")


@dataclass(frozen=True, slots=True)
class ProgressEvent:
    stage: Literal["discovery", "selection", "validation", "downloading", "completion"]
    status: str
    attempt: int = 0
    manga_id: str = ""
    chapter_id: str = ""
    completed_pages: int = 0
    expected_pages: int = 0
    reason: str = ""


@dataclass(frozen=True, slots=True)
class DownloadPlan:
    manga: Manga
    chapter: Chapter
    policy: SelectionPolicy
    strategy: Literal["provider_random", "seeded_catalog"]
    expected_pages: int
    output: Path
    output_format: Literal["directory", "png", "jpeg"] = "directory"
    candidate_ids: tuple[str, ...] = ()
    version: int = 1
    validation: str = "eligible"


@dataclass(frozen=True, slots=True)
class ManifestImage:
    index: int
    path: Path
    format: str
    bytes: int = 0
    sha256: str = ""


@dataclass(frozen=True, slots=True)
class AcquisitionManifest:
    plan: DownloadPlan
    images: tuple[ManifestImage, ...] = ()
    status: str = "planned"
    validation: str = "eligible"
    error: str = ""
    version: int = 2

    @property
    def actual_pages(self) -> int:
        return len(self.images)


@dataclass(frozen=True, slots=True)
class InteractiveResult:
    downloads: tuple[DownloadResult, ...]
    status: AcquisitionStatus
    error: str = ""


@dataclass(frozen=True, slots=True)
class ProviderInfo:
    id: str
    name: str
    version: str
    availability: str
    description: str
    authentication: str
    capabilities: tuple[str, ...]
    restrictions: tuple[str, ...]
    download_permitted: bool
    usable: bool
    error: str = ""
