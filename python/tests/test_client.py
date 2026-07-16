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
            result = Client().convert("chapter", output_format="cbz", remove_source=True, dry_run=True, assume_yes=True)
        command = popen.call_args.args[0]
        self.assertIn("archive", command)
        self.assertIn("convert", command)
        self.assertIn("--format", command)
        self.assertIn("--remove-source", command)
        self.assertIn("--dry-run", command)
        self.assertIn("--yes", command)
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

    def test_convert_many_preserves_input_and_output_order(self) -> None:
        first = _CompletedProcess({"formatVersion": "1", "operation": "archive.convert", "status": "success", "data": {"outputPath": "first.cbz"}}, 0)
        second = _CompletedProcess({"formatVersion": "1", "operation": "archive.convert", "status": "success", "data": {"outputPath": "second.cbz"}}, 0)
        with patch("mangate.client.subprocess.Popen", side_effect=[first, second]) as popen:
            results = Client().convert_many(["first", "second"], output_format="cbz", outputs=["first.cbz", "second.cbz"], dry_run=True, assume_yes=True)
        self.assertEqual([result["outputPath"] for result in results], ["first.cbz", "second.cbz"])
        first_command, second_command = [call.args[0] for call in popen.call_args_list]
        self.assertEqual(first_command[-1], "first")
        self.assertEqual(second_command[-1], "second")
        self.assertIn("first.cbz", first_command)
        self.assertIn("second.cbz", second_command)
        self.assertIn("--yes", first_command)

    def test_convert_many_rejects_mismatched_destinations(self) -> None:
        with self.assertRaisesRegex(ValueError, "one path per"):
            Client().convert_many(["first", "second"], outputs=["only-one.cbz"])


if __name__ == "__main__":
    unittest.main()
