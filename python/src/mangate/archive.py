"""Typed local archive conversion, inspection, and verification."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from ._decode import decode
from ._transport import Transport


@dataclass(frozen=True, slots=True)
class Validation:
    valid: bool
    complete: bool
    page_count: int
    message: str = ""
    format: str = ""
    title_id: str = ""
    chapter_id: str = ""


@dataclass(frozen=True, slots=True)
class Metadata:
    schema_version: str
    completion: str
    created_at: str
    provider: str = ""
    title_id: str = ""
    title: str = ""
    chapter_id: str = ""
    volume: str = ""
    chapter_number: str = ""
    chapter_title: str = ""
    language: str = ""
    release_group: str = ""
    published_at: str = ""
    expected_pages: int = 0


@dataclass(frozen=True, slots=True)
class Conversion:
    format: str
    output_path: Path
    source_dir: Path
    status: str
    included_pages: int
    validation: Validation
    source_removed: bool
    warnings: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class ConversionPlan:
    source_dir: Path
    output_path: Path
    format: str
    destination_exists: bool
    remove_source: bool


@dataclass(frozen=True, slots=True)
class Inspection(Validation):
    state: str = ""
    path: Path = Path(".")
    entry_count: int = 0
    entries: tuple[str, ...] = ()
    unexpected_entries: tuple[str, ...] = ()
    metadata_found: bool = False
    identity_confirmed: bool = False
    metadata: Metadata | None = None


def convert(
    chapter_directory: str | os.PathLike[str],
    *,
    output_format: str = "cbz",
    output: str | os.PathLike[str] | None = None,
    remove_source: bool = False,
    dry_run: bool = False,
    assume_yes: bool = False,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> Conversion | ConversionPlan:
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
    if assume_yes:
        args.append("--yes")
    args.append(os.fspath(chapter_directory))
    return decode(
        ConversionPlan if dry_run else Conversion,
        Transport(executable, timeout=timeout)._json(args)["data"],
    )


def inspect(
    archive_path: str | os.PathLike[str],
    *,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> Inspection:
    """Return archive entries, metadata state, and page count without extraction."""
    return decode(
        Inspection,
        Transport(executable, timeout=timeout)._json(
            ["archive", "inspect", os.fspath(archive_path)]
        )["data"],
    )


def verify(
    archive_path: str | os.PathLike[str],
    *,
    executable: str | os.PathLike[str] | None = None,
    timeout: float | None = None,
) -> Inspection:
    """Verify archive structure, safe entry paths, and completion metadata."""
    return decode(
        Inspection,
        Transport(executable, timeout=timeout)._json(
            ["archive", "verify", os.fspath(archive_path)]
        )["data"],
    )
