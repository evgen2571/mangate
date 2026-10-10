"""Python API for the Mangate command-line executable.

Its data and error categories mirror the JSON CLI.
"""

from importlib.metadata import version

from .api import DownloadError, Session, chapters, download, search
from .automation import (
    download_random,
    execute,
    load_manifest,
    plan_random,
    save_manifest,
    verify_manifest,
)
from .client import Client, MangateError
from .models import (
    AcquisitionManifest,
    Chapter,
    DownloadOptions,
    DownloadPlan,
    DownloadResult,
    Manga,
    ManifestImage,
    Page,
    ProgressEvent,
    SelectionPolicy,
)

__all__ = [
    "download_random",
    "plan_random",
    "execute",
    "load_manifest",
    "save_manifest",
    "verify_manifest",
    "SelectionPolicy",
    "DownloadPlan",
    "AcquisitionManifest",
    "ProgressEvent",
    "ManifestImage",
    "Client",
    "MangateError",
    "DownloadError",
    "Session",
    "Manga",
    "Chapter",
    "Page",
    "DownloadResult",
    "DownloadOptions",
    "search",
    "chapters",
    "download",
    "__version__",
]
__version__ = version("mangate")
