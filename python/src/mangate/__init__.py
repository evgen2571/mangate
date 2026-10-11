"""Python API for the Mangate command-line executable.

Its data and error categories mirror the JSON CLI.
"""

from importlib.metadata import version

from . import archive, dataset
from .api import (
    DownloadError,
    Session,
    chapters,
    cli_version,
    download,
    download_many,
    interactive_download,
    provider_info,
    providers,
    search,
    title,
)
from .automation import (
    download_random,
    execute,
    load_manifest,
    plan_random,
    save_manifest,
    verify_manifest,
)
from .errors import MangateError
from .models import (
    AcquisitionManifest,
    Chapter,
    DownloadOptions,
    DownloadPlan,
    DownloadResult,
    InteractiveResult,
    Manga,
    ManifestImage,
    Page,
    ProgressEvent,
    ProviderInfo,
    SelectionPolicy,
)

__all__ = [
    "archive",
    "dataset",
    "interactive_download",
    "download_many",
    "providers",
    "provider_info",
    "cli_version",
    "InteractiveResult",
    "ProviderInfo",
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
    "MangateError",
    "DownloadError",
    "Session",
    "Manga",
    "Chapter",
    "Page",
    "DownloadResult",
    "DownloadOptions",
    "search",
    "title",
    "chapters",
    "download",
    "__version__",
]
__version__ = version("mangate")
