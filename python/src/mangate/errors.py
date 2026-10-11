"""Public error categories and validated partial acquisition results."""

from __future__ import annotations

from dataclasses import dataclass

from .models import DownloadResult, InteractiveResult


@dataclass(slots=True)
class MangateError(RuntimeError):
    """A Mangate operation failure with a stable, programmatic category."""

    category: str
    message: str
    stderr: str = ""
    result: DownloadResult | InteractiveResult | None = None

    def __str__(self) -> str:
        return self.message


class DownloadError(MangateError):
    """An unsuccessful acquisition; result holds validated partial output."""
