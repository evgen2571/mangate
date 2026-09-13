"""Check documented command syntax, Python snippets, and local Markdown links."""

import ast
import os
import re
import shlex
import subprocess
import tempfile
from pathlib import Path


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    documents = [
        root / "README.md",
        root / "python/README.md",
        *sorted((root / "docs").glob("*.md")),
    ]
    commands = snippets = links = 0
    with tempfile.TemporaryDirectory(prefix="mangate-docs-") as temporary:
        binary = Path(temporary) / ("mangate.exe" if os.name == "nt" else "mangate")
        subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd/mangate"],
            cwd=root,
            check=True,
            timeout=120,
        )
        env = dict(os.environ, MANGATE_CONFIG=str(Path(temporary) / "config.json"))
        for document in documents:
            text = document.read_text()
            for language, block in re.findall(
                r"```([^\n]*)\n(.*?)```", text, re.DOTALL
            ):
                if language == "python":
                    ast.parse(block, filename=str(document))
                    snippets += 1
                if language not in {"sh", "bash"}:
                    continue
                for line in block.splitlines():
                    if not line.startswith("mangate "):
                        continue
                    args = shlex.split(line)
                    result = subprocess.run(
                        [str(binary), *args[1:], "--help"],
                        cwd=temporary,
                        env=env,
                        capture_output=True,
                        text=True,
                        timeout=10,
                    )
                    if result.returncode:
                        raise RuntimeError(f"{document}: {line}\n{result.stderr}")
                    commands += 1
            for target in re.findall(r"\]\(([^)]+)\)", text):
                if "://" in target or target.startswith("#"):
                    continue
                path = document.parent / target.split("#", 1)[0]
                if not path.exists():
                    raise RuntimeError(f"{document}: broken link {target}")
                links += 1
    print(
        f"Verified {commands} command/flag combinations, {snippets} Python snippets, and {links} local links"
    )
    print(
        "Help checks do not execute live-provider operations; runtime coverage is in the integration tests."
    )


if __name__ == "__main__":
    main()
