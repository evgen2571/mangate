"""Python API for the Mangate command-line executable.

Its data and error categories mirror the JSON CLI.
"""

from importlib.metadata import version

from .api import DownloadError, Session, chapters, download, search
from .client import Client, MangateError
from .models import Chapter, DownloadOptions, DownloadResult, Manga, Page

__all__ = [
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
