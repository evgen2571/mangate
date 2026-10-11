"""Real Bubble Tea-to-Python handoff under a pseudo-terminal, without the network."""

import base64
import json
import os
import select
import struct
import subprocess
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

pytestmark = pytest.mark.skipif(
    os.name == "nt", reason="POSIX PTY fixture; Windows needs a ConPTY host"
)


@pytest.mark.parametrize(
    "outcome",
    [
        "success",
        "cancelled",
        "partial",
        "timeout",
        "standalone",
        "interrupted",
        "multiple",
    ],
)
def test_terminal_handoff(binary, tmp_path, outcome):
    import fcntl
    import termios

    page = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aPioAAAAASUVORK5CYII="
    )

    second_page = threading.Event()
    release_page = threading.Event()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            path = self.path.split("?")[0]
            if outcome == "interrupted" and path.endswith("2.png"):
                second_page.set()
                release_page.wait(timeout=10)
                self.send_error(404)
                return
            if path == "/manga/":
                body = {
                    "data": [
                        {"id": "title", "attributes": {"title": {"en": "Fixture"}}}
                    ],
                    "total": 1,
                }
            elif path == "/manga/title/feed":
                body = {
                    "data": [
                        {"id": "chapter", "attributes": {"chapter": "1", "pages": 2}}
                    ],
                    "total": 1,
                }
            elif path in {"/at-home/server/chapter", "/at-home/server/second"}:
                body = {
                    "baseUrl": f"http://127.0.0.1:{self.server.server_port}",
                    "chapter": {"hash": "hash", "data": ["1.png", "2.png"]},
                }
            elif path.startswith("/data/") and not (
                outcome == "partial" and path.endswith("2.png")
            ):
                self.send_response(200)
                self.send_header("Content-Type", "image/png")
                self.end_headers()
                self.wfile.write(page)
                return
            else:
                self.send_error(404)
                return
            if outcome == "multiple" and path == "/manga/title/feed":
                body["data"].append(
                    {"id": "second", "attributes": {"chapter": "2", "pages": 2}}
                )
                body["total"] = 2
            self.send_response(200)
            self.end_headers()
            self.wfile.write(json.dumps(body).encode())

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    base = f"http://127.0.0.1:{server.server_port}"
    config = tmp_path / "config.json"
    config.write_text(
        json.dumps(
            {
                "providers": {
                    "mangadex": {"baseUrl": base, "uploadsUrl": base, "siteUrl": base}
                },
                "dirs": {"cache": str(tmp_path / "cache")},
            }
        )
    )
    script = tmp_path / "caller.py"
    handoff = tmp_path / "handoff.json"
    script.write_text(
        "import json,sys\nfrom pathlib import Path\nfrom dataclasses import asdict\nfrom mangate import interactive_download,MangateError\n"
        "try:\n r=interactive_download(executable=sys.argv[1],output_dir=sys.argv[2],max_chapters=int(sys.argv[5]),timeout=float(sys.argv[4]))\n"
        "except MangateError as e:\n r={'status':e.category,'result':asdict(e.result) if e.result else None}\n"
        "Path(sys.argv[3]).write_text(json.dumps(asdict(r) if not isinstance(r,dict) else r,default=str))\nprint('PYTHON RESUMED',flush=True)\n"
    )
    master, slave = os.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
    original = termios.tcgetattr(slave)
    env = dict(os.environ, TERM="xterm-256color", MANGATE_CONFIG=str(config))
    command = [
        sys.executable,
        str(script),
        str(binary),
        str(tmp_path / "downloads"),
        str(handoff),
        "1" if outcome == "timeout" else "15",
        "0" if outcome == "multiple" else "1",
    ]
    if outcome == "standalone":
        command = [str(binary), "tui"]
    proc = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave, env=env)
    output = bytearray()

    def until(text):
        start = len(output)
        deadline = time.monotonic() + 10
        while text not in output[start:]:
            if time.monotonic() > deadline:
                pytest.fail(
                    f"TUI did not render {text!r}: {output.decode(errors='replace')}"
                )
            if select.select([master], [], [], 0.1)[0]:
                output.extend(os.read(master, 65536))

    try:
        until(b"Search manga")
        if outcome in {"cancelled", "standalone"}:
            os.write(master, b"\x03")
        elif outcome != "timeout":
            os.write(master, b"fixture\r")
            until(b"Fixture")
            os.write(master, b"\r")
            until(b"Chapter 1")
            os.write(master, b" \x1b[B \r" if outcome == "multiple" else b" \r")
            until(b"Output directory")
            os.write(master, b"\r")
            until(b"2 selected" if outcome == "multiple" else b"1 selected")
            os.write(master, b"\r")
        if outcome == "interrupted":
            assert second_page.wait(timeout=5)
            os.write(master, b"\x03")
        if outcome != "standalone":
            until(b"PYTHON RESUMED")
        else:
            # Drain shutdown output before waiting: macOS PTY buffers are small.
            until(b"\x1b[?1049l")
        assert proc.wait(timeout=10) == 0
        # Query pending input to clear macOS's kernel-maintained PENDIN flag.
        fcntl.ioctl(slave, termios.FIONREAD, struct.pack("I", 0))
        assert termios.tcgetattr(slave) == original
        # Bubble Tea left the alternate screen before the Python continuation.
        if outcome != "standalone":
            assert output.rfind(b"\x1b[?1049l") < output.index(b"PYTHON RESUMED")
            assert b"\x1b[?1049l" in output
            data = json.loads(handoff.read_text())
            assert data["status"] == (
                "cancelled"
                if outcome == "interrupted"
                else "complete"
                if outcome in {"success", "multiple"}
                else outcome
            )
            if outcome in {"success", "multiple"}:
                assert [
                    os.path.basename(p)
                    for p in [page["path"] for page in data["downloads"][0]["pages"]]
                ] == ["0001.png", "0002.png"]
            if outcome == "multiple":
                assert len(data["downloads"]) == 2
                assert all(
                    download["status"] == "complete" for download in data["downloads"]
                )
            if outcome == "partial":
                assert data["result"]["downloads"][0]["pages"] == []
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()
        os.close(master)
        os.close(slave)
        release_page.set()
        server.shutdown()
        server.server_close()
        thread.join()
