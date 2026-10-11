import importlib.util

import pytest

import mangate


def test_legacy_client_is_removed():
    assert not hasattr(mangate, "Client")
    assert importlib.util.find_spec("mangate.client") is None


def test_supported_operations_are_discoverable():
    for name in (
        "interactive_download",
        "providers",
        "provider_info",
        "cli_version",
        "download_many",
    ):
        assert callable(getattr(mangate, name, None)), name
    for module, names in (
        ("archive", ("convert", "inspect", "verify")),
        ("dataset", ("plan", "collect", "status", "verify", "export")),
    ):
        for name in names:
            assert callable(getattr(getattr(mangate, module, None), name, None)), (
                module,
                name,
            )


@pytest.mark.parametrize(
    "call",
    [
        lambda: mangate.search(1, executable="unused"),
        lambda: mangate.search("title", limit=True, executable="unused"),
        lambda: mangate.chapters("", executable="unused"),
        lambda: mangate.download(title=1, chapter=1, executable="unused"),
        lambda: mangate.download(title_id="id", chapter=True, executable="unused"),
    ],
)
def test_invalid_arguments_fail_before_starting_process(call):
    with pytest.raises(ValueError):
        call()
