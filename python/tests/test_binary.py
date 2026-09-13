"""Exercise the installed Python API with a compiled CLI and local files."""

import base64
import os
import subprocess
import zipfile
from pathlib import Path

import pytest

from mangate import Client, MangateError


@pytest.fixture(scope="module")
def binary(tmp_path_factory: pytest.TempPathFactory) -> Path:
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


@pytest.fixture
def client(binary: Path, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Client:
    monkeypatch.setenv("MANGATE_CONFIG", str(tmp_path / "config.json"))
    return Client(executable=binary, timeout=10)


def test_binary_version_and_provider_discovery(client: Client) -> None:
    assert client.version().startswith("mangate ")
    providers = client.providers()
    assert any(provider["info"]["id"] == "mangadex" for provider in providers)
    assert client.provider_info()["info"]["id"] == "mangadex"


@pytest.mark.parametrize("output_format", ["cbz", "zip"])
def test_binary_archive_round_trip(
    client: Client, tmp_path: Path, output_format: str
) -> None:
    source = tmp_path / "chapter with spaces"
    source.mkdir()
    page = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aPioAAAAASUVORK5CYII="
    )
    (source / "0001.png").write_bytes(page)
    output = tmp_path / f"chapter.{output_format}"
    result = client.convert(source, output_format=output_format, output=output)
    assert result["includedPages"] == 1
    assert source.is_dir()
    with zipfile.ZipFile(output) as archive:
        assert archive.read("0001.png") == page
    assert client.inspect_archive(output)["pageCount"] == 1
    assert client.verify_archive(output)["valid"] is True


def test_binary_reports_missing_archive(client: Client, tmp_path: Path) -> None:
    with pytest.raises(MangateError) as caught:
        client.inspect_archive(tmp_path / "missing.cbz")
    assert caught.value.category == "archive"
    assert "missing.cbz" in str(caught.value)
