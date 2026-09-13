"""Python API for the Mangate command-line executable.

Its data and error categories mirror the JSON CLI.
"""

from importlib.metadata import version

from .client import Client, MangateError

__all__ = ["Client", "MangateError", "__version__"]
__version__ = version("mangate")
