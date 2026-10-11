"""Exercise the installed Python API with a compiled CLI and local files."""

import base64
import zipfile
from pathlib import Path

import pytest

from mangate import MangateError, archive, cli_version, provider_info, providers


@pytest.fixture
def client(binary: Path, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> dict:
    monkeypatch.setenv("MANGATE_CONFIG", str(tmp_path / "config.json"))
    return dict(executable=binary, timeout=10)


def test_binary_version_and_provider_discovery(client: dict) -> None:
    assert cli_version(**client).startswith("mangate ")
    found = providers(**client)
    assert any(provider.id == "mangadex" for provider in found)
    assert provider_info(**client).id == "mangadex"


@pytest.mark.parametrize("output_format", ["cbz", "zip"])
def test_binary_archive_round_trip(
    client: dict, tmp_path: Path, output_format: str
) -> None:
    source = tmp_path / "chapter with spaces"
    source.mkdir()
    page = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aPioAAAAASUVORK5CYII="
    )
    (source / "0001.png").write_bytes(page)
    output = tmp_path / f"chapter.{output_format}"
    result = archive.convert(
        source, output_format=output_format, output=output, **client
    )
    assert result.included_pages == 1
    assert source.is_dir()
    with zipfile.ZipFile(output) as zipped:
        assert zipped.read("0001.png") == page
    assert archive.inspect(output, **client).page_count == 1
    assert archive.verify(output, **client).valid is True


def test_binary_reports_missing_archive(client: dict, tmp_path: Path) -> None:
    with pytest.raises(MangateError) as caught:
        archive.inspect(tmp_path / "missing.cbz", **client)
    assert caught.value.category == "archive"
    assert "missing.cbz" in str(caught.value)
