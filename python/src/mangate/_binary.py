"""Find and launch the executable shipped in the platform wheel."""

import os
import platform
import shutil
import subprocess
import sys
from pathlib import Path


def resolve_executable() -> str:
    system, machine = platform.system().lower(), platform.machine().lower()
    supported = machine in {"x86_64", "amd64", "arm64", "aarch64"} and system in {
        "linux",
        "darwin",
        "windows",
    }
    if not supported or (system == "windows" and machine not in {"x86_64", "amd64"}):
        raise RuntimeError(
            f"Unsupported Mangate platform: {system}-{machine}; provide an explicit executable"
        )
    binary = (
        Path(__file__).parent
        / "bin"
        / ("mangate.exe" if os.name == "nt" else "mangate")
    )
    if binary.is_file():
        return str(binary)
    # Editable source checkouts use the separately built developer executable.
    return shutil.which("mangate") or "mangate"


def main() -> None:
    try:
        executable = resolve_executable()
        resolved = shutil.which(executable) or executable
        if Path(resolved).resolve() == Path(sys.argv[0]).resolve():
            raise RuntimeError(
                "Go executable missing; build it and provide an explicit path for this source checkout"
            )
        code = subprocess.call([executable, *sys.argv[1:]])
    except (OSError, RuntimeError) as error:
        print(f"mangate: {error}", file=sys.stderr)
        raise SystemExit(3) from None
    raise SystemExit(code)
