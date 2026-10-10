import importlib.util
from pathlib import Path

import pytest


def test_release_targets_have_platform_specific_tags():
    hook = Path(__file__).resolve().parents[1] / "hatch_build.py"
    assert hook.exists(), "platform wheel build hook is missing"
    spec = importlib.util.spec_from_file_location("wheel_hook", hook)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    assert module.TARGETS == {
        "linux-amd64": "manylinux_2_17_x86_64.musllinux_1_2_x86_64",
        "linux-arm64": "manylinux_2_17_aarch64.musllinux_1_2_aarch64",
        "darwin-amd64": "macosx_12_0_x86_64",
        "darwin-arm64": "macosx_12_0_arm64",
        "windows-amd64": "win_amd64",
    }
    with pytest.raises(ValueError, match="Unsupported"):
        module.target_platform("plan9", "mips")


def test_console_launcher_rejects_itself(monkeypatch, tmp_path, capsys):
    import sys

    from mangate import binary

    launcher = tmp_path / "mangate"
    launcher.write_text("launcher")
    monkeypatch.setattr(binary, "resolve_executable", lambda: str(launcher))
    monkeypatch.setattr(sys, "argv", [str(launcher), "--version"])
    with pytest.raises(SystemExit) as caught:
        binary.main()
    assert caught.value.code == 3
    assert "executable" in capsys.readouterr().err
