import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Event, Thread

import pytest

# A valid one-pixel PNG, served locally instead of contacting a provider.
PNG = bytes.fromhex(
    "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082"
)


@pytest.fixture
def provider(tmp_path, monkeypatch):
    state = {
        "fail": False,
        "description": {},
        "chapter_count": 1,
        "random_calls": 0,
        "image_calls": 0,
        "title_calls": 0,
        "original_language": "en",
        "chapter_language": "en",
        "page_count": 2,
        "unavailable": False,
        "missing_chapter": False,
        "invalid_image": False,
        "refresh_changes_count": False,
        "actual_pages": 2,
        "block_stage": "",
        "ambiguous": False,
        "block": False,
        "entered": Event(),
        "release": Event(),
    }

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            path = self.path.split("?")[0]
            title = {
                "id": "title-id",
                "attributes": {
                    "title": {"en": "Solo Leveling"},
                    "description": state["description"],
                    "originalLanguage": state["original_language"],
                },
            }
            if state["block_stage"] and path == state["block_stage"]:
                state["entered"].set()
                state["release"].wait(10)
            if path == "/manga/random":
                state["random_calls"] += 1
                payload = {"result": "ok", "data": title}
            elif path == "/manga/":
                data = [title, title] if state["ambiguous"] else [title]
                payload = {
                    "result": "ok",
                    "data": data,
                    "limit": 100,
                    "offset": 0,
                    "total": len(data),
                }
            elif path == "/manga/title-id":
                state["title_calls"] += 1
                payload = {"result": "ok", "data": title}
            elif path == "/manga/title-id/feed":
                payload = {
                    "result": "ok",
                    "data": [
                        {
                            "id": "chapter-id",
                            "attributes": {
                                "chapter": "1",
                                "volume": "2",
                                "pages": state["page_count"],
                                "translatedLanguage": state["chapter_language"],
                            },
                        }
                    ],
                    "limit": 500,
                    "offset": 0,
                    "total": 1,
                }
                for number in range(2, state["chapter_count"] + 1):
                    payload["data"].append(
                        {
                            "id": "chapter-two" if number == 2 else f"chapter-{number}",
                            "attributes": {
                                "chapter": str(number),
                                "pages": 2,
                                "translatedLanguage": "en",
                            },
                        }
                    )
                    payload["total"] = state["chapter_count"]
                if state["missing_chapter"]:
                    payload["data"] = []
                    payload["total"] = 0
            elif path.startswith("/at-home/server/chapter-"):
                if state["unavailable"]:
                    self.send_error(404)
                    return
                payload = {
                    "result": "ok",
                    "baseUrl": base,
                    "chapter": {
                        "hash": "hash",
                        "data": [
                            f"{i}.png" for i in range(1, state["actual_pages"] + 1)
                        ],
                    },
                }
            elif path.startswith("/data/hash/"):
                state["image_calls"] += 1
                if state["block"] and path.endswith("2.png"):
                    state["entered"].set()
                    state["release"].wait(10)
                if state["refresh_changes_count"] and path.endswith("2.png"):
                    state["actual_pages"] = 3
                    self.send_error(403)
                    return
                if state["fail"] and path.endswith("2.png"):
                    self.send_error(404)
                    return
                self.send_response(200)
                self.send_header("Content-Type", "image/png")
                self.end_headers()
                try:
                    self.wfile.write(
                        b"invalid image" if state["invalid_image"] else PNG
                    )
                except (BrokenPipeError, ConnectionResetError):
                    pass
                return
            else:
                self.send_error(404)
                return
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(payload).encode())

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    base = f"http://127.0.0.1:{server.server_port}"
    thread = Thread(target=server.serve_forever)
    thread.start()
    config = tmp_path / "config.json"
    config.write_text(
        json.dumps(
            {
                "providers": {
                    "mangadex": {"baseUrl": base, "siteUrl": base, "uploadsUrl": base}
                },
                "dirs": {"cache": str(tmp_path / "cache")},
            }
        )
    )
    monkeypatch.setenv("MANGATE_CONFIG", str(config))
    try:
        yield state
    finally:
        state["release"].set()
        server.shutdown()
        thread.join()
        server.server_close()
