"""Typed dataset planning, collection, inspection, verification and export.

Collection configuration is the existing versioned Go JSON file.
"""

from __future__ import annotations

import os
from collections.abc import Iterable, Mapping
from dataclasses import dataclass, field
from pathlib import Path
from threading import Event

from ._decode import decode
from ._transport import Transport


@dataclass(frozen=True, slots=True)
class Output:
    directory: str
    format: str
    existing_files: str


@dataclass(frozen=True, slots=True)
class Discovery:
    original_languages: tuple[str, ...]
    chapter_languages: tuple[str, ...]
    statuses: tuple[str, ...]
    content_ratings: tuple[str, ...]
    included_tags: tuple[str, ...]
    excluded_tags: tuple[str, ...]
    order_by: str
    order_direction: str
    candidate_pool_size: int


@dataclass(frozen=True, slots=True)
class Sampling:
    seed: int
    title_strategy: str
    chapter_strategy: str
    max_titles: int
    max_chapters_per_title: int
    keep_duplicate_chapter_releases: bool


@dataclass(frozen=True, slots=True)
class Limits:
    max_pages: int
    max_bytes: int
    max_failures: int


@dataclass(frozen=True, slots=True)
class Validation:
    minimum_width: int
    minimum_height: int
    maximum_width: int
    maximum_height: int
    maximum_decoded_pixels: int
    full_decode: bool
    calculate_sha256: bool = field(metadata={"json": "calculateSHA256"})
    calculate_perceptual_hash: bool


@dataclass(frozen=True, slots=True)
class Splits:
    enabled: bool
    train: float
    validation: float
    test: float


@dataclass(frozen=True, slots=True)
class Runtime:
    title_workers: int
    chapter_workers: int
    page_workers: int
    validation_workers: int
    retry_limit: int


@dataclass(frozen=True, slots=True)
class Config:
    version: int
    dataset_id: str
    provider: str
    output: Output
    discovery: Discovery
    sampling: Sampling
    limits: Limits
    validation: Validation
    splits: Splits
    runtime: Runtime


@dataclass(frozen=True, slots=True)
class Counters:
    discovered_titles: int
    planned_titles: int
    completed_titles: int
    failed_titles: int
    planned_chapters: int
    completed_chapters: int
    failed_chapters: int
    planned_pages: int
    valid_pages: int
    duplicate_pages: int
    rejected_pages: int
    failed_pages: int
    archives: int
    stored_bytes: int


@dataclass(frozen=True, slots=True)
class DatasetInfo:
    config: Config
    config_hash: str = field(metadata={"json": "configurationHash"})
    state: str
    stopping_reason: str
    created_at: str
    updated_at: str
    counters: Counters
    split_counts: Mapping[str, int]


@dataclass(frozen=True, slots=True)
class Plan:
    candidates: int
    titles: int
    chapters: int
    estimated_pages: int
    split_counts: Mapping[str, int]
    warnings: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class CollectResult:
    dataset_root: str
    dataset_id: str
    provider: str
    format: str
    state: str
    stopping_reason: str
    counters: Counters
    manifest_path: str
    summary_path: str
    resumed: bool


@dataclass(frozen=True, slots=True)
class PlanResult:
    dataset_root: Path
    dataset_id: str
    provider: str
    format: str
    plan: Plan
    confirmation_required: bool = False
    dry_run: bool = False


@dataclass(frozen=True, slots=True)
class Verification:
    dataset_root: Path
    checked_pages: int
    invalid_pages: int
    adopted_archives: int
    state_inconsistencies: int
    invalid_duplicate_references: int
    manifest_inconsistencies: int
    temporary_files: int
    staging_directories: int
    unexpected_files: int
    split_leakage: int
    repair: bool
    valid: bool


@dataclass(frozen=True, slots=True)
class ExportResult:
    manifest_path: Path
    summary_path: Path


def plan(
    *,
    collection_config: str | os.PathLike[str] | None = None,
    resume: bool = False,
    extra_args: Iterable[str] = (),
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    provider: str | None = None,
    output_dir: str | os.PathLike[str] | None = None,
) -> PlanResult:
    args = ["dataset", "plan"]
    if provider is not None:
        args = ["--provider", provider] + args
    if collection_config is not None:
        args.extend(["--collection-config", os.fspath(collection_config)])
    if resume:
        args.append("--resume")
    args.extend(extra_args)
    return decode(
        PlanResult,
        Transport(
            executable,
            timeout=timeout,
            provider=provider or "mangadex",
            output_dir=output_dir,
            output_format=None,
            existing_files=None,
        )._json(args)["data"],
    )


def collect(
    *,
    collection_config: str | os.PathLike[str] | None = None,
    resume: bool = False,
    dry_run: bool = False,
    assume_yes: bool = True,
    cancel_event: Event | None = None,
    extra_args: Iterable[str] = (),
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
    provider: str | None = None,
    output_dir: str | os.PathLike[str] | None = None,
) -> PlanResult | CollectResult:
    args = ["dataset", "collect"]
    if provider is not None:
        args = ["--provider", provider] + args
    if collection_config is not None:
        args.extend(["--collection-config", os.fspath(collection_config)])
    if resume:
        args.append("--resume")
    if dry_run:
        args.append("--dry-run")
    if assume_yes:
        args.append("--yes")
    args.extend(extra_args)
    return decode(
        PlanResult if dry_run else CollectResult,
        Transport(
            executable,
            timeout=timeout,
            provider=provider or "mangadex",
            output_dir=output_dir,
            output_format=None,
            existing_files=None,
        )._json(args, cancel_event=cancel_event)["data"],
    )


def status(
    dataset_root: str | os.PathLike[str],
    *,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> DatasetInfo:
    return decode(
        DatasetInfo,
        Transport(executable, timeout=timeout)._json(
            ["dataset", "status", os.fspath(dataset_root)]
        )["data"],
    )


def verify(
    dataset_root: str | os.PathLike[str],
    *,
    repair: bool = False,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> Verification:
    args = ["dataset", "verify", os.fspath(dataset_root)]
    if repair:
        args.append("--repair")
    return decode(
        Verification, Transport(executable, timeout=timeout)._json(args)["data"]
    )


def export(
    dataset_root: str | os.PathLike[str],
    *,
    split: str | None = None,
    include_duplicates: bool = False,
    include_rejected: bool = False,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> ExportResult:
    args = ["dataset", "export", os.fspath(dataset_root)]
    if split:
        args.extend(["--split", split])
    if include_duplicates:
        args.append("--include-duplicates")
    if include_rejected:
        args.append("--include-rejected")
    return decode(
        ExportResult, Transport(executable, timeout=timeout)._json(args)["data"]
    )
