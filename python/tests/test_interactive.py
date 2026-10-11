"""The result file, rather than terminal output, controls the handoff."""

import json
import os
import sys
from pathlib import Path

import pytest

from mangate import MangateError, Session


@pytest.fixture
def interactive_client(tmp_path, monkeypatch):
    # Substitute only the terminal check; the child and its files are real.
    monkeypatch.setattr("mangate._terminal._has_terminal", lambda: True, raising=False)
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
    client = Session(executable=sys.executable)
    monkeypatch.setattr(
        "mangate._transport.Transport._base_args",
        lambda _: [sys.executable, str(script), str(payload), str(client.code)],
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
    actual = client.interactive_download(max_chapters=1)
    assert actual.status == "complete"
    assert actual.downloads[0].image_paths == (
        Path(expected["chapters"][0]["imagePaths"][0]),
    )
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
    assert caught.value.result.status == status


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
    monkeypatch.setattr("mangate._terminal._has_terminal", lambda: False, raising=False)
    with pytest.raises(MangateError, match="terminal"):
        Session().interactive_download()


def test_missing_executable(interactive_client, tmp_path, monkeypatch):
    client, _ = interactive_client
    monkeypatch.setattr(
        "mangate._transport.Transport._base_args",
        lambda _: [str(tmp_path / "no-executable")],
    )
    with pytest.raises(MangateError) as caught:
        client.interactive_download()
    assert caught.value.category == "executable"


def test_timeout_restores_terminal(interactive_client, monkeypatch):
    client, _ = interactive_client
    Path(client._transport._base_args()[1]).write_text("import time\ntime.sleep(60)\n")
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


def test_multiple_chapters_keep_completed_results(interactive_client, tmp_path):
    client, payload = interactive_client
    data = result(tmp_path)
    failed = {
        "chapter": {"id": "second"},
        "expectedPages": 2,
        "status": "failed",
        "outputDir": str(tmp_path / "title" / "second"),
        "imagePaths": [],
        "error": "page request failed",
    }
    data["chapters"].append(failed)
    data["status"] = "partial"
    client.code = 5
    payload.write_text(json.dumps(data))
    with pytest.raises(MangateError) as caught:
        client.interactive_download(max_chapters=0)
    result_value = caught.value.result
    assert result_value.status == "partial"
    assert len(result_value.downloads) == 2
    assert result_value.downloads[0].complete
    assert result_value.downloads[0].image_paths[0].is_file()
    assert result_value.downloads[1].status == "failed"
    assert not result_value.downloads[1].image_paths


def test_timeout_keeps_completed_chapters(interactive_client, tmp_path):
    client, payload = interactive_client
    payload.write_text(json.dumps(result(tmp_path)))
    script = Path(client._transport._base_args()[1])
    script.write_text(
        script.read_text().replace(
            "sys.exit(int(args[2]))", "import time\ntime.sleep(60)"
        )
    )
    with pytest.raises(MangateError) as caught:
        client.interactive_download(timeout=0.1)
    assert caught.value.category == "timeout"
    assert caught.value.result.downloads[0].complete


@pytest.mark.skipif(os.name == "nt", reason="POSIX process-group probe")
def test_interactive_timeout_reaps_descendants(interactive_client, tmp_path):
    import time

    client, _ = interactive_client
    marker = tmp_path / "escaped"
    script = Path(client._transport._base_args()[1])
    script.write_text(
        "import subprocess,sys,time\n"
        f"subprocess.Popen([sys.executable,'-c', {('import time; from pathlib import Path; time.sleep(1); Path(' + repr(str(marker)) + ').touch()')!r}])\n"
        "time.sleep(60)\n"
    )
    with pytest.raises(MangateError):
        client.interactive_download(timeout=0.1)
    time.sleep(1.2)
    assert not marker.exists()
