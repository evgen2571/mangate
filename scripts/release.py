"""Build, validate, and smoke-test release artifacts. Never publishes anything."""

import argparse
import base64
import csv
import hashlib
import importlib.util
import io
import os
import re
import struct
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from email.parser import BytesParser
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
# Python 3.10 remains supported; read the single version field without tomllib.
VERSION = re.search(
    r'^version = "([^"]+)"$', (ROOT / "python/pyproject.toml").read_text(), re.M
)[1]
_spec = importlib.util.spec_from_file_location(
    "mangate_build", ROOT / "python/hatch_build.py"
)
_hook = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_hook)
TARGETS = _hook.TARGETS


def run(args, **kwargs):
    return subprocess.run(args, cwd=ROOT, check=True, **kwargs)


def verify_tag(tag):
    if (
        not re.fullmatch(
            r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", tag
        )
        or tag != f"v{VERSION}"
    ):
        raise ValueError(f"Release tag must be v{VERSION}, matching Python metadata")
    if run(["git", "status", "--porcelain"], capture_output=True, text=True).stdout:
        raise ValueError("Release requires a clean worktree")
    tagged = run(
        ["git", "rev-parse", f"{tag}^{{commit}}"], capture_output=True, text=True
    ).stdout.strip()
    head = run(
        ["git", "rev-parse", "HEAD"], capture_output=True, text=True
    ).stdout.strip()
    if tagged != head:
        raise ValueError("Release tag does not point to HEAD")
    run(["git", "merge-base", "--is-ancestor", "HEAD", "origin/main"])


def verify_binary(data, target):
    system, arch = target.split("-")
    if system == "linux":
        if (
            data[:4] != b"\x7fELF"
            or struct.unpack_from("<H", data, 18)[0]
            != {"amd64": 62, "arm64": 183}[arch]
        ):
            raise ValueError(f"Wrong ELF binary for {target}")
        # CGO_DISABLED must produce a static executable (no PT_INTERP).
        offset = struct.unpack_from("<Q", data, 32)[0]
        size, count = struct.unpack_from("<HH", data, 54)
        if any(
            struct.unpack_from("<I", data, offset + i * size)[0] == 3
            for i in range(count)
        ):
            raise ValueError("Linux wheel binary has a dynamic interpreter")
    elif system == "darwin":
        if (
            data[:4] != b"\xcf\xfa\xed\xfe"
            or struct.unpack_from("<I", data, 4)[0]
            != {"amd64": 0x01000007, "arm64": 0x0100000C}[arch]
        ):
            raise ValueError(f"Wrong Mach-O binary for {target}")
    else:
        offset = struct.unpack_from("<I", data, 60)[0]
        if (
            data[:2] != b"MZ"
            or data[offset : offset + 4] != b"PE\0\0"
            or struct.unpack_from("<H", data, offset + 4)[0] != 0x8664
        ):
            raise ValueError("Wrong Windows executable")
    with tempfile.TemporaryDirectory() as temporary:
        path = Path(temporary) / "binary"
        path.write_bytes(data)
        info = run(
            ["go", "version", "-m", str(path)], capture_output=True, text=True
        ).stdout
        if (
            "CGO_ENABLED=0" not in info
            or f"GOOS={system}" not in info
            or f"GOARCH={arch}" not in info
        ):
            raise ValueError("Executable build settings disagree with its platform")
        if target == _hook.target_platform():
            path.chmod(0o755)
            output = run(
                [str(path), "--version"],
                capture_output=True,
                text=True,
                env=dict(
                    os.environ, MANGATE_CONFIG=str(Path(temporary) / "config.json")
                ),
            ).stdout
            if output.split() != ["mangate", VERSION, f"({system})"]:
                raise ValueError("Executable version disagrees with package")


def validate_wheel(path, target):
    expected = f"mangate-{VERSION}-py3-none-{TARGETS[target]}.whl"
    if path.name != expected:
        raise ValueError(f"Expected {expected}, got {path.name}")
    with zipfile.ZipFile(path) as wheel:
        prefix = f"mangate-{VERSION}.dist-info/"
        metadata = BytesParser().parsebytes(wheel.read(prefix + "METADATA"))
        if (
            metadata["Name"] != "mangate"
            or metadata["Version"] != VERSION
            or metadata["Requires-Python"] != ">=3.10"
        ):
            raise ValueError("Invalid Python package metadata")
        wheel_metadata = BytesParser().parsebytes(wheel.read(prefix + "WHEEL"))
        tags = {f"py3-none-{platform}" for platform in TARGETS[target].split(".")}
        if (
            set(wheel_metadata.get_all("Tag")) != tags
            or wheel_metadata["Root-Is-Purelib"] != "false"
        ):
            raise ValueError("Wheel advertises incorrect platforms")
        records = list(csv.reader(io.StringIO(wheel.read(prefix + "RECORD").decode())))
        if {row[0] for row in records} != set(wheel.namelist()):
            raise ValueError("Incomplete wheel RECORD")
        for name, digest, size in records:
            if name == prefix + "RECORD":
                continue
            data = wheel.read(name)
            expected_hash = (
                base64.urlsafe_b64encode(hashlib.sha256(data).digest())
                .rstrip(b"=")
                .decode()
            )
            if digest != "sha256=" + expected_hash or int(size) != len(data):
                raise ValueError(f"Corrupt wheel record: {name}")
        binary = "mangate/bin/" + (
            "mangate.exe" if target.startswith("windows") else "mangate"
        )
        if [name for name in wheel.namelist() if name.startswith("mangate/bin/")] != [
            binary
        ]:
            raise ValueError("Wheel must contain exactly one platform executable")
        if (wheel.getinfo(binary).external_attr >> 16) & 0o111 == 0:
            raise ValueError("Bundled executable is not executable")
        verify_binary(wheel.read(binary), target)
        if (
            "mangate = mangate._binary:main"
            not in wheel.read(prefix + "entry_points.txt").decode()
        ):
            raise ValueError("Missing console entry point")


def build(directory, snapshot):
    directory.mkdir(parents=True, exist_ok=True)
    if list(directory.glob("*.whl")):
        raise ValueError("Use an empty wheel output directory to avoid stale artifacts")
    env = dict(os.environ, MANGATE_VERSION=VERSION)
    run(["goreleaser", "check"], env=env)
    args = ["goreleaser", "release", "--clean", "--skip=publish"]
    if snapshot:
        args.append("--snapshot")
    run(args, env=env)
    for target in TARGETS:
        run(
            [
                "uv",
                "build",
                "--wheel",
                "--out-dir",
                str(directory),
                str(ROOT / "python"),
            ],
            env=dict(env, MANGATE_WHEEL_TARGET=target),
        )
    assets = (
        list(directory.glob("*.whl"))
        + list((ROOT / "dist").glob("*.tar.gz"))
        + list((ROOT / "dist").glob("*.zip"))
    )
    (directory / "SHA256SUMS").write_text(
        "".join(
            f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n"
            for path in sorted(assets)
        )
    )
    validate(directory, ROOT / "dist")


def validate(directory, archives):
    wheels = list(directory.glob("*.whl"))
    if len(wheels) != len(TARGETS) or list(directory.glob("*.tar.gz")):
        raise ValueError("Exactly five wheels and no source distribution are required")
    for target in TARGETS:
        validate_wheel(
            directory / f"mangate-{VERSION}-py3-none-{TARGETS[target]}.whl", target
        )
    if archives is not None:
        for target in TARGETS:
            system, arch = target.split("-")
            path = (
                archives
                / f"mangate_{VERSION}_{system}_{arch}.{'zip' if system == 'windows' else 'tar.gz'}"
            )
            if system == "windows":
                with zipfile.ZipFile(path) as archive:
                    data = archive.read("mangate.exe")
            else:
                with tarfile.open(path) as archive:
                    data = archive.extractfile("mangate").read()
            verify_binary(data, target)
            with zipfile.ZipFile(
                directory / f"mangate-{VERSION}-py3-none-{TARGETS[target]}.whl"
            ) as wheel:
                binary_name = "mangate/bin/" + (
                    "mangate.exe" if system == "windows" else "mangate"
                )
                if data != wheel.read(binary_name):
                    raise ValueError(
                        f"Go archive and Python executable disagree: {target}"
                    )
        checksums = {}
        for line in (archives / "go-checksums.txt").read_text().splitlines():
            digest, name = line.split(maxsplit=1)
            checksums[name.strip()] = digest
        for target in TARGETS:
            system, arch = target.split("-")
            name = f"mangate_{VERSION}_{system}_{arch}.{'zip' if system == 'windows' else 'tar.gz'}"
            if (
                checksums.get(name)
                != hashlib.sha256((archives / name).read_bytes()).hexdigest()
            ):
                raise ValueError(f"Invalid archive checksum: {name}")
    print(f"Validated all five release targets at {VERSION}")


def smoke(directory, integration=False):
    target = _hook.target_platform()
    wheel = directory / f"mangate-{VERSION}-py3-none-{TARGETS[target]}.whl"
    validate_wheel(wheel, target)
    with tempfile.TemporaryDirectory(prefix="mangate-wheel-") as temporary:
        environment = Path(temporary) / "venv"
        run(["uv", "venv", "--python", sys.executable, str(environment)])
        bindir = environment / ("Scripts" if os.name == "nt" else "bin")
        python = bindir / ("python.exe" if os.name == "nt" else "python")
        run(["uv", "pip", "install", "--python", str(python), "--no-deps", str(wheel)])
        env = dict(os.environ, MANGATE_CONFIG=str(Path(temporary) / "config.json"))
        # No Go or separately installed Mangate is used by this subprocess.
        run(
            [
                str(python),
                "-I",
                "-c",
                "from mangate import cli_version,provider_info,__version__; from mangate._binary import resolve_executable; from pathlib import Path; "
                "assert Path(resolve_executable()).is_file(); "
                "assert cli_version().split()[1]==__version__; "
                "assert provider_info().id=='mangadex'",
            ],
            env=env,
        )
        launcher = bindir / ("mangate.exe" if os.name == "nt" else "mangate")
        run([str(launcher), "--version"], env=env)
        if integration:
            bundled = bindir.parent / "bundled-executable"
            with zipfile.ZipFile(wheel) as archive:
                name = "mangate/bin/" + (
                    "mangate.exe" if os.name == "nt" else "mangate"
                )
                bundled.write_bytes(archive.read(name))
            bundled.chmod(0o755)
            run(
                [
                    "uv",
                    "run",
                    "--project",
                    "python",
                    "--locked",
                    "pytest",
                    "python/tests/test_binary.py",
                    "python/tests/test_interactive.py",
                    "python/tests/test_terminal.py",
                ],
                env=dict(env, MANGATE_TEST_EXECUTABLE=str(bundled)),
            )
    print(f"Clean wheel installation passed for {target}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["build", "validate", "smoke", "tag"])
    parser.add_argument("--directory", type=Path, default=ROOT / "python/dist")
    parser.add_argument("--archives", type=Path)
    parser.add_argument("--tag")
    parser.add_argument("--snapshot", action="store_true")
    parser.add_argument("--integration", action="store_true")
    args = parser.parse_args()
    if args.command == "tag":
        verify_tag(args.tag or "")
    elif args.command == "build":
        if not args.snapshot:
            verify_tag(args.tag or "")
        build(args.directory.resolve(), args.snapshot)
    elif args.command == "validate":
        validate(args.directory.resolve(), args.archives)
    else:
        smoke(args.directory.resolve(), args.integration)


if __name__ == "__main__":
    main()
