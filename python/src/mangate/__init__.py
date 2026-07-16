"""Python API for Mangate 0.1.x.

The package supports Python 3.10+ on Linux and uses the matching Mangate CLI
as its execution engine. Its data and error categories mirror the JSON CLI.
"""

from .client import Client, MangateError

__all__ = ["Client", "MangateError", "__version__"]
__version__ = "0.1.0"
