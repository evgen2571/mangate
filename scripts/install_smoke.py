"""Install the CLI and Python package into temporary user-owned directories."""

import os
import subprocess
import sys
import tempfile
from pathlib import Path


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    with tempfile.TemporaryDirectory(prefix="mangate-install-") as temporary:
        directory = Path(temporary)
        bin_dir = directory / "bin"
        bin_dir.mkdir()
        env = dict(os.environ, GOBIN=str(bin_dir))
        env["MANGATE_CONFIG"] = str(directory / "config.json")
        if os.name == "nt":
            subprocess.run(
                ["go", "install", "./cmd/mangate"], cwd=root, env=env, check=True
            )
        else:
            env["MANGATE_BIN_DIR"] = str(bin_dir)
            subprocess.run(
                ["bash", str(root / "scripts/install.sh")], env=env, check=True
            )
        binary = bin_dir / ("mangate.exe" if os.name == "nt" else "mangate")
        subprocess.run([str(binary), "--version"], env=env, check=True)
        environment = directory / "venv"
        subprocess.run(
            ["uv", "venv", "--python", sys.executable, str(environment)], check=True
        )
        python = environment / (
            "Scripts/python.exe" if os.name == "nt" else "bin/python"
        )
        subprocess.run(
            ["uv", "pip", "install", "--python", str(python), str(root / "python")],
            check=True,
        )
        subprocess.run(
            [
                str(python),
                "-I",
                "-c",
                "from mangate import Client; import sys; "
                "c = Client(executable=sys.argv[1]); "
                "assert c.version().startswith('mangate '); "
                "assert c.provider_info()['info']['id'] == 'mangadex'",
                str(binary),
            ],
            cwd=directory,
            env=env,
            check=True,
        )
    print("CLI and Python installation smoke test passed")


if __name__ == "__main__":
    main()
