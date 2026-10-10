import os
import subprocess
from pathlib import Path

import pytest


@pytest.fixture(scope="module")
def binary(tmp_path_factory: pytest.TempPathFactory) -> Path:
    if bundled := os.environ.get("MANGATE_TEST_EXECUTABLE"):
        path = Path(bundled)
        assert path.is_file(), f"Missing bundled test executable: {path}"
        return path
    root = Path(__file__).resolve().parents[2]
    path = tmp_path_factory.mktemp("binary") / (
        "mangate.exe" if os.name == "nt" else "mangate"
    )
    subprocess.run(
        ["go", "build", "-o", str(path), "./cmd/mangate"],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
        timeout=120,
    )
    return path
