from __future__ import annotations

import json
import unittest
from unittest.mock import patch

from mangate.client import Client


class _CompletedProcess:
    def __init__(self, payload: dict[str, object], returncode: int) -> None:
        self._payload = payload
        self.returncode = returncode

    def poll(self) -> int:
        return self.returncode

    def communicate(self) -> tuple[str, str]:
        return json.dumps(self.payload), ""

    @property
    def payload(self) -> dict[str, object]:
        return self._payload

    @payload.setter
    def payload(self, value: dict[str, object]) -> None:
        self._payload = value


class ClientTests(unittest.TestCase):
    def test_rejects_unknown_output_format(self) -> None:
        with self.assertRaisesRegex(ValueError, "output_format"):
            Client(output_format="rar")

    def test_download_returns_partial_result_despite_nonzero_exit(self) -> None:
        payload = {
            "formatVersion": "1",
            "operation": "download",
            "status": "partial",
            "data": {"chapters": [{"id": "completed", "status": "complete"}]},
        }
        process = _CompletedProcess(payload, 5)
        with patch("mangate.client.subprocess.Popen", return_value=process):
            result = Client().download("title-id", latest=True)
        self.assertEqual(result["chapters"][0]["status"], "complete")

    def test_convert_uses_archive_command_and_requested_format(self) -> None:
        payload = {
            "formatVersion": "1",
            "operation": "archive.convert",
            "status": "success",
            "data": {"format": "cbz"},
        }
        process = _CompletedProcess(payload, 0)
        with patch("mangate.client.subprocess.Popen", return_value=process) as popen:
            result = Client().convert("chapter", output_format="cbz", remove_source=True, dry_run=True)
        command = popen.call_args.args[0]
        self.assertIn("archive", command)
        self.assertIn("convert", command)
        self.assertIn("--format", command)
        self.assertIn("--remove-source", command)
        self.assertIn("--dry-run", command)
        self.assertEqual(result["format"], "cbz")

    def test_download_dry_run_uses_requested_format_without_writes(self) -> None:
        payload = {
            "formatVersion": "1",
            "operation": "download.plan",
            "status": "success",
            "data": {"format": "zip", "status": "planned"},
        }
        process = _CompletedProcess(payload, 0)
        with patch("mangate.client.subprocess.Popen", return_value=process) as popen:
            result = Client().download("title-id", latest=True, output_format="zip", dry_run=True, assume_yes=True)
        command = popen.call_args.args[0]
        self.assertIn("--dry-run", command)
        self.assertIn("--yes", command)
        self.assertIn("zip", command)
        self.assertEqual(result["status"], "planned")


if __name__ == "__main__":
    unittest.main()
