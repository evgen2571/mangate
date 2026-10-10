"""Exercise actual child processes, including output larger than pipe buffers."""

import json
import sys
from threading import Event, Timer

import pytest

from mangate import Client, MangateError


def script_client(tmp_path, script, timeout=3):
    path = tmp_path / "child.py"
    path.write_text(script)
    client = Client(executable=sys.executable, timeout=timeout)
    client._base_args = lambda: [sys.executable, str(path)]
    return client


def test_drains_both_large_streams(tmp_path):
    client = script_client(
        tmp_path,
        """
import json, sys
sys.stderr.write('warning' * 100000)
print(json.dumps({'status': 'success', 'data': {'text': 'x' * 1000000}}))
""",
    )
    assert len(client._json([])["data"]["text"]) == 1000000


@pytest.mark.parametrize("category", ["timeout", "cancelled"])
def test_stop_preserves_partial_result_and_reaps_child(tmp_path, category):
    client = script_client(
        tmp_path,
        """
import json, os, signal, sys, time
from pathlib import Path
Path(sys.argv[-1]).write_text(str(os.getpid()))
def stop(*args):
    print(json.dumps({'status': 'cancelled', 'data': {'chapters': [{'id': 'saved'}]}}), flush=True)
    sys.exit(7)
signal.signal(signal.SIGINT, stop)
if hasattr(signal, 'SIGBREAK'):
    signal.signal(signal.SIGBREAK, stop)
while True: time.sleep(.01)
""",
        timeout=0.5 if category == "timeout" else 3,
    )
    event = Event()
    timer = Timer(0.5, event.set)
    if category == "cancelled":
        timer.start()
    try:
        with pytest.raises(MangateError) as caught:
            client._json([str(tmp_path / "pid")], cancel_event=event)
        assert caught.value.category == category
        if caught.value.result is None and sys.platform == "win32":
            # Without an attached console Windows must terminate the process.
            assert (tmp_path / "pid").is_file()
        else:
            assert caught.value.result["chapters"][0]["id"] == "saved"
    finally:
        timer.cancel()


def test_failed_download_retains_record(tmp_path):
    record = {"status": "failed", "chapters": [{"id": "broken"}]}
    payload = {"status": "failed", "data": record}
    client = script_client(
        tmp_path, f"print({json.dumps(payload)!r})\nraise SystemExit(5)"
    )
    with pytest.raises(MangateError) as caught:
        client.download("title", first=True)
    assert caught.value.result == record


def test_timeout_kills_uncooperative_process(tmp_path, monkeypatch):
    import subprocess

    processes = []
    real_popen = subprocess.Popen

    def start(*args, **kwargs):
        proc = real_popen(*args, **kwargs)
        processes.append(proc)
        return proc

    monkeypatch.setattr("mangate.client.subprocess.Popen", start)
    client = script_client(
        tmp_path,
        """
import signal, time
signal.signal(signal.SIGINT, signal.SIG_IGN)
if hasattr(signal, 'SIGBREAK'): signal.signal(signal.SIGBREAK, signal.SIG_IGN)
while True: time.sleep(.01)
""",
        timeout=0.3,
    )
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "timeout"
    assert processes[0].poll() is not None
    assert processes[0].stdout.closed and processes[0].stderr.closed


@pytest.mark.parametrize("timeout", [0, -1, float("nan"), float("inf")])
def test_invalid_timeout_rejected(timeout):
    with pytest.raises(ValueError, match="timeout"):
        Client(executable=sys.executable, timeout=timeout)


def test_text_timeout_uses_structured_exception(tmp_path):
    client = script_client(tmp_path, "import time\ntime.sleep(20)", timeout=0.1)
    with pytest.raises(MangateError) as caught:
        client.version()
    assert caught.value.category == "timeout"


def test_non_object_json_is_structured_error(tmp_path):
    client = script_client(tmp_path, "print('[]')")
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "invalid_result"


@pytest.mark.skipif(sys.platform == "win32", reason="POSIX process group cleanup")
def test_timeout_handles_descendant_holding_pipes(tmp_path):
    client = script_client(
        tmp_path,
        """
import subprocess, sys
subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'])
""",
        timeout=0.3,
    )
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "timeout"


@pytest.mark.parametrize("signal_available", [True, False])
def test_windows_stop_uses_group_signal_with_termination_fallback(
    monkeypatch, signal_available
):
    from types import SimpleNamespace

    from mangate.client import _stop_process

    calls = []

    class Process:
        def poll(self):
            return None

        def send_signal(self, value):
            calls.append(("signal", value))
            if not signal_available:
                raise OSError("no console")

        def terminate(self):
            calls.append(("terminate",))

        def communicate(self, timeout):
            calls.append(("drain", timeout))
            return "result", "diagnostic"

    monkeypatch.setattr("mangate.client.os", SimpleNamespace(name="nt"))
    monkeypatch.setattr("mangate.client.signal", SimpleNamespace(CTRL_BREAK_EVENT=123))
    assert _stop_process(Process()) == ("result", "diagnostic")
    assert calls[0] == ("signal", 123)
    assert (("terminate",) in calls) == (not signal_available)
    assert calls[-1] == ("drain", 2)
