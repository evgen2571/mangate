"""Exercise actual child processes, including output larger than pipe buffers."""

import json
import sys
from threading import Event, Timer

import pytest

from mangate import MangateError
from mangate._transport import Transport


def script_client(tmp_path, script, timeout=3):
    path = tmp_path / "child.py"
    path.write_text(script)
    client = Transport(executable=sys.executable, timeout=timeout)
    client._base_args = lambda: [sys.executable, str(path)]
    return client


def test_drains_both_large_streams(tmp_path):
    client = script_client(
        tmp_path,
        """
import json, sys
sys.stderr.write('warning' * 100000)
print(json.dumps({'formatVersion': '1', 'operation': 'probe', 'status': 'success', 'data': {'text': 'x' * 1000000}}))
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
    payload = {
        "formatVersion": "1",
        "operation": "download",
        "status": "failed",
        "data": record,
    }
    client = script_client(
        tmp_path, f"print({json.dumps(payload)!r})\nraise SystemExit(5)"
    )
    with pytest.raises(MangateError) as caught:
        client._json(["download", "--first", "title"])
    assert caught.value.result == record


def test_timeout_kills_uncooperative_process(tmp_path, monkeypatch):
    import subprocess

    processes = []
    streams = []
    real_popen = subprocess.Popen

    def start(*args, **kwargs):
        proc = real_popen(*args, **kwargs)
        streams.extend([kwargs["stdout"], kwargs["stderr"]])
        processes.append(proc)
        return proc

    monkeypatch.setattr("mangate._transport.subprocess.Popen", start)
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
    assert all(stream.closed for stream in streams)


@pytest.mark.parametrize("timeout", [0, -1, float("nan"), float("inf")])
def test_invalid_timeout_rejected(timeout):
    with pytest.raises(ValueError, match="timeout"):
        Transport(executable=sys.executable, timeout=timeout)


def test_text_timeout_uses_structured_exception(tmp_path):
    client = script_client(tmp_path, "import time\ntime.sleep(20)", timeout=0.1)
    with pytest.raises(MangateError) as caught:
        client._text(["--version"])
    assert caught.value.category == "timeout"


def test_non_object_json_is_structured_error(tmp_path):
    client = script_client(tmp_path, "print('[]')")
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "invalid_result"


def test_timeout_handles_descendant_holding_pipes(tmp_path):
    client = script_client(
        tmp_path,
        """
import subprocess, sys, time
subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(20)'])
time.sleep(20)
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

    from mangate._transport import _stop_process

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

        def wait(self, timeout):
            calls.append(("wait", timeout))
            return 0

    monkeypatch.setattr("mangate._transport.os", SimpleNamespace(name="nt"))
    monkeypatch.setattr(
        "mangate._transport.signal", SimpleNamespace(CTRL_BREAK_EVENT=123)
    )
    assert _stop_process(Process()) is None
    assert calls[0] == ("signal", 123)
    assert (("terminate",) in calls) == (not signal_available)
    assert calls[-1] == ("wait", 2)


def process_running(pid):
    import os

    if sys.platform == "win32":
        import ctypes
        from ctypes import wintypes

        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
        kernel.OpenProcess.restype = wintypes.HANDLE
        kernel.GetExitCodeProcess.argtypes = [wintypes.HANDLE, ctypes.c_void_p]
        kernel.CloseHandle.argtypes = [wintypes.HANDLE]
        handle = kernel.OpenProcess(0x1000, False, pid)
        if not handle:
            return False
        try:
            code = wintypes.DWORD()
            assert kernel.GetExitCodeProcess(handle, ctypes.byref(code))
            return code.value == 259
        finally:
            kernel.CloseHandle(handle)
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    from pathlib import Path

    status = Path(f"/proc/{pid}/stat")
    return not status.exists() or status.read_text().split(")", 1)[1].split()[0] != "Z"


@pytest.mark.parametrize("category", ["success", "timeout", "cancelled"])
@pytest.mark.parametrize("uncooperative", [False, True])
def test_descendant_cleanup_is_bounded(tmp_path, category, uncooperative):
    import time

    child = f"""
import os, signal, time
from pathlib import Path
if {uncooperative!r}:
    signal.signal(signal.SIGINT, signal.SIG_IGN)
    if hasattr(signal, 'SIGBREAK'): signal.signal(signal.SIGBREAK, signal.SIG_IGN)
Path({str(tmp_path / "descendant.pid")!r}).write_text(str(os.getpid()))
time.sleep(30)
"""
    client = script_client(
        tmp_path,
        f"""
import json, subprocess, sys, time
from pathlib import Path
subprocess.Popen([sys.executable, '-c', {child!r}])
while not Path({str(tmp_path / "descendant.pid")!r}).exists(): time.sleep(.01)
print(json.dumps({{'formatVersion': '1', 'operation': 'probe', 'status': 'success', 'data': {{'saved': True}}}}), flush=True)
if {category != "success"!r}: time.sleep(30)
""",
        timeout=0.7 if category == "timeout" else 5,
    )
    event = Event()
    timer = Timer(0.7, event.set)
    if category == "cancelled":
        timer.start()
    started = time.monotonic()
    try:
        if category == "success":
            assert client._json([])["data"]["saved"]
        else:
            with pytest.raises(MangateError) as caught:
                client._json([], cancel_event=event)
            assert caught.value.category == category
            assert caught.value.result == {"saved": True}
        assert time.monotonic() - started < 6
        pid = int((tmp_path / "descendant.pid").read_text())
        deadline = time.monotonic() + 2
        while process_running(pid) and time.monotonic() < deadline:
            time.sleep(0.02)
        assert not process_running(pid), f"descendant {pid} survived cleanup"
    finally:
        timer.cancel()


def test_stop_tolerates_process_exiting_during_signal(monkeypatch):
    from types import SimpleNamespace

    from mangate._transport import _stop_process

    class Process:
        exited = False

        def poll(self):
            return 0 if self.exited else None

        def send_signal(self, value):
            self.exited = True
            raise OSError("process exited")

        def terminate(self):
            raise OSError("process exited")

        def wait(self, timeout):
            assert timeout == 2
            return 0

    monkeypatch.setattr("mangate._transport.os", SimpleNamespace(name="nt"))
    monkeypatch.setattr(
        "mangate._transport.signal", SimpleNamespace(CTRL_BREAK_EVENT=123)
    )
    _stop_process(Process())


def test_forced_cleanup_never_waits_without_deadline(monkeypatch):
    import subprocess
    from types import SimpleNamespace

    from mangate._transport import _stop_process

    waits = []

    class Process:
        def poll(self):
            return None

        def send_signal(self, value):
            pass

        def wait(self, timeout):
            waits.append(timeout)
            raise subprocess.TimeoutExpired("child", timeout)

        def kill(self):
            pass

    monkeypatch.setattr("mangate._transport.os", SimpleNamespace(name="nt"))
    monkeypatch.setattr(
        "mangate._transport.signal", SimpleNamespace(CTRL_BREAK_EVENT=123)
    )
    _stop_process(Process())
    assert waits == [2, 2]


@pytest.mark.skipif(sys.platform != "win32", reason="Windows job assignment failure")
def test_job_assignment_failure_reaps_suspended_child(tmp_path, monkeypatch):
    import subprocess

    from mangate._windows import ProcessJob

    processes = []
    real_popen = subprocess.Popen

    def start(*args, **kwargs):
        proc = real_popen(*args, **kwargs)
        processes.append(proc)
        return proc

    def reject(self, proc):
        raise OSError("job assignment rejected")

    monkeypatch.setattr("mangate._transport.subprocess.Popen", start)
    monkeypatch.setattr(ProcessJob, "start", reject)
    client = script_client(tmp_path, "import time; time.sleep(30)")
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "executable"
    assert processes[0].poll() is not None


def test_output_keeps_utf8_replacement_and_universal_newlines(tmp_path):
    client = script_client(
        tmp_path,
        "import sys\nsys.stdout.buffer.write(b'first\\r\\nsecond\\r\\xff')\n"
        "sys.stderr.buffer.write(b'warning\\r\\n\\xff')",
    )
    stdout, stderr, code = client._run(client._base_args())
    assert code == 0
    assert stdout == "first\nsecond\n\ufffd"
    assert stderr == "warning\n\ufffd"


@pytest.mark.parametrize(
    "payload",
    [
        {
            "formatVersion": "999",
            "operation": "search",
            "status": "success",
            "data": {},
        },
        {"formatVersion": "1", "operation": "search", "status": "success"},
        {"formatVersion": "1", "operation": "search", "status": "partial", "data": {}},
        {
            "formatVersion": "1",
            "operation": "search",
            "status": "no_results",
            "data": {},
        },
    ],
)
def test_rejects_malformed_json_envelope(tmp_path, payload):
    client = script_client(tmp_path, f"print({json.dumps(payload)!r})")
    with pytest.raises(MangateError) as caught:
        client._json([])
    assert caught.value.category == "invalid_result"
