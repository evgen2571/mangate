"""The result file, rather than terminal output, controls the handoff."""

import json
import os
import sys
from pathlib import Path

import pytest

from mangate import Client, MangateError


@pytest.fixture
def interactive_client(tmp_path, monkeypatch):
    # Substitute only the terminal check; the child and its files are real.
    monkeypatch.setattr("mangate.client._has_terminal", lambda: True, raising=False)
    if os.name != "nt":
        import termios

        monkeypatch.setattr(termios, "tcgetattr", lambda _: [])
        monkeypatch.setattr(termios, "tcsetattr", lambda *_: None)
    monkeypatch.setattr(sys, "stdin", sys.__stdin__)
    script = tmp_path / "child.py"
    script.write_text(
        "import json,sys\n"
        "from pathlib import Path\n"
        "args=sys.argv\n"
        "Path(args[args.index('--result-file')+1]).write_text("
        "Path(args[1]).read_text())\n"
        "Path(args[1]+'.location').write_text(args[args.index('--result-file')+1])\n"
        "sys.exit(int(args[2]))\n"
    )
    payload = tmp_path / "payload.json"
    client = Client(executable=sys.executable)
    monkeypatch.setattr(
        client,
        "_base_args",
        lambda: [sys.executable, str(script), str(payload), str(client.code)],
    )
    client.code = 0
    return client, payload


def result(tmp_path):
    chapter = tmp_path / "title" / "chapter"
    chapter.mkdir(parents=True)
    image = chapter / "0001.png"
    image.write_bytes(b"image")
    return {
        "formatVersion": "1",
        "status": "success",
        "title": {"id": "title", "title": "Title"},
        "outputDir": str(tmp_path),
        "chapters": [
            {
                "chapter": {"id": "chapter"},
                "expectedPages": 1,
                "status": "success",
                "outputDir": str(chapter),
                "imagePaths": [str(image)],
            }
        ],
    }


def test_interactive_success(interactive_client, tmp_path):
    client, payload = interactive_client
    expected = result(tmp_path)
    payload.write_text(json.dumps(expected))
    assert client.interactive_download(max_chapters=1) == expected
    assert not Path(Path(str(payload) + ".location").read_text()).parent.exists()


@pytest.mark.parametrize("status,code", [("cancelled", 7), ("partial", 5)])
def test_interactive_never_returns_failed_download(
    interactive_client, tmp_path, status, code
):
    client, payload = interactive_client
    data = result(tmp_path)
    data["status"] = status
    payload.write_text(json.dumps(data))
    client.code = code
    with pytest.raises(MangateError) as caught:
        client.interactive_download()
    assert caught.value.category == status
    assert caught.value.result["status"] == status


@pytest.mark.parametrize(
    "mutation", ["json", "version", "missing", "escape", "order", "empty", "exit"]
)
def test_invalid_interactive_result(interactive_client, tmp_path, mutation):
    client, payload = interactive_client
    data = result(tmp_path)
    if mutation == "version":
        data["formatVersion"] = "999"
    elif mutation == "missing":
        Path(data["chapters"][0]["imagePaths"][0]).unlink()
    elif mutation == "escape":
        data["chapters"][0]["outputDir"] = str(tmp_path / "elsewhere")
    elif mutation == "order":
        data["chapters"][0]["imagePaths"] *= 2
    elif mutation == "empty":
        data["chapters"] = []
    elif mutation == "exit":
        client.code = 5
    payload.write_text("garbage" if mutation == "json" else json.dumps(data))
    with pytest.raises(MangateError):
        client.interactive_download()


def test_missing_terminal(monkeypatch):
    monkeypatch.setattr("mangate.client._has_terminal", lambda: False, raising=False)
    with pytest.raises(MangateError, match="terminal"):
        Client().interactive_download()


def test_missing_executable(interactive_client, tmp_path, monkeypatch):
    client, _ = interactive_client
    monkeypatch.setattr(client, "_base_args", lambda: [str(tmp_path / "no-executable")])
    with pytest.raises(MangateError) as caught:
        client.interactive_download()
    assert caught.value.category == "executable"


def test_timeout_restores_terminal(interactive_client, monkeypatch):
    client, _ = interactive_client
    Path(client._base_args()[1]).write_text("import time\ntime.sleep(60)\n")
    restored = []
    if os.name != "nt":
        import termios

        monkeypatch.setattr(termios, "tcsetattr", lambda *args: restored.append(args))
    with pytest.raises(MangateError) as caught:
        client.interactive_download(timeout=0.05)
    assert caught.value.category == "timeout"
    if os.name != "nt":
        assert len(restored) == 1


def test_rejects_truncated_success_sequence(interactive_client, tmp_path):
    client, payload = interactive_client
    data = result(tmp_path)
    data["chapters"][0]["expectedPages"] = 2
    payload.write_text(json.dumps(data))
    with pytest.raises(MangateError):
        client.interactive_download()
