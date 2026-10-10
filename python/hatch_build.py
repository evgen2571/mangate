"""Bundle a static Go executable, never an unusable universal wheel or sdist."""

import os
import platform
import subprocess
from pathlib import Path

from hatchling.builders.hooks.plugin.interface import BuildHookInterface

TARGETS = {
    "linux-amd64": "manylinux_2_17_x86_64.musllinux_1_2_x86_64",
    "linux-arm64": "manylinux_2_17_aarch64.musllinux_1_2_aarch64",
    "darwin-amd64": "macosx_12_0_x86_64",
    "darwin-arm64": "macosx_12_0_arm64",
    "windows-amd64": "win_amd64",
}


def target_platform(system=None, machine=None):
    system = system or platform.system().lower()
    machine = machine or platform.machine().lower()
    arch = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }.get(machine, machine)
    target = f"{system}-{arch}"
    if target not in TARGETS:
        raise ValueError(f"Unsupported Mangate platform: {target}")
    return target


class CustomBuildHook(BuildHookInterface):
    def initialize(self, version, build_data):
        if self.target_name == "sdist":
            raise RuntimeError(
                "Mangate distributes executable wheels only; build from the Git repository with Go installed"
            )
        if version == "editable":
            return
        target = os.environ.get("MANGATE_WHEEL_TARGET") or target_platform()
        if target not in TARGETS:
            raise ValueError(f"Unsupported Mangate wheel target: {target}")
        goos, goarch = target.split("-")
        root = Path(self.root).parent
        binary = (
            root
            / "python"
            / ".build"
            / target
            / ("mangate.exe" if goos == "windows" else "mangate")
        )
        binary.parent.mkdir(parents=True, exist_ok=True)
        package_version = self.metadata.version
        env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED="0")
        subprocess.run(
            [
                "go",
                "build",
                "-trimpath",
                "-buildvcs=false",
                "-ldflags",
                f"-s -w -X github.com/evgen2571/mangate/internal/constant.Version={package_version}",
                "-o",
                str(binary),
                "./cmd/mangate",
            ],
            cwd=root,
            env=env,
            check=True,
        )
        binary.chmod(0o755)
        build_data["pure_python"] = False
        build_data["tag"] = f"py3-none-{TARGETS[target]}"
        build_data["force_include"][str(binary)] = f"mangate/bin/{binary.name}"
