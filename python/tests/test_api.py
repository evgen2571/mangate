import json
import sys
from dataclasses import FrozenInstanceError
from threading import Event, Thread

import pytest
from provider_fixture import PNG

import mangate


def test_function_download_and_session_defaults(binary, provider, tmp_path):
    found = mangate.search("Solo Leveling", executable=binary)
    assert found[0].provider == "mangadex"
    assert mangate.title(found[0].id, executable=binary) == found[0]
    with pytest.raises(FrozenInstanceError):
        found[0].title = "changed"
    chapters = mangate.chapters(found[0], executable=binary)
    assert chapters[0].number == "1"
    assert chapters[0].volume == "2"
    output = tmp_path / "exact"
    result = mangate.download(
        title="Solo Leveling", chapter=1, output=output, executable=binary
    )
    assert isinstance(result, mangate.DownloadResult)
    assert result.complete and result.status == "complete"
    assert result.manga.title == "Solo Leveling"
    assert result.chapter.id == "chapter-id"
    assert result.image_paths == (output / "0001.png", output / "0002.png")
    assert [p.index for p in result.pages] == [1, 2]
    assert all(p.is_file() for p in result.image_paths)
    again = mangate.Session(executable=binary).download(
        found[0], chapters[0], output=output
    )
    assert again.complete
    organized = mangate.Session(
        executable=binary, output_dir=tmp_path / "library"
    ).download(title_id="title-id", chapter_id="chapter-id")
    assert organized.output.parent.name == "Solo-Leveling-title-id"
    assert organized.image_paths


def test_partial_download_raises_with_typed_paths(binary, provider, tmp_path):
    provider["fail"] = True
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download(
            title_id="title-id",
            chapter_id="chapter-id",
            output=tmp_path / "partial",
            executable=binary,
        )
    result = caught.value.result
    assert isinstance(result, mangate.DownloadResult)
    assert not result.complete and result.status == "partial"
    assert result.expected_pages == 2
    assert result.image_paths == (tmp_path / "partial" / "0001.png",)
    provider["fail"] = False
    assert mangate.download(
        title_id="title-id",
        chapter_id="chapter-id",
        output=tmp_path / "partial",
        executable=binary,
    ).complete


def test_rejects_ambiguous_title(binary, provider, tmp_path):
    provider["ambiguous"] = True
    with pytest.raises(mangate.MangateError, match="ambiguous"):
        mangate.download(
            title="Solo Leveling",
            chapter=1,
            output=tmp_path / "exact",
            executable=binary,
        )
    assert not (tmp_path / "exact").exists()


def test_destination_does_not_overwrite_unidentified_files(binary, provider, tmp_path):
    output = tmp_path / "existing"
    output.mkdir()
    old = output / "0001.png"
    old.write_bytes(b"user data")
    with pytest.raises(mangate.MangateError):
        mangate.download(
            title_id="title-id", chapter=1, output=output, executable=binary
        )
    assert old.read_bytes() == b"user data"


@pytest.mark.parametrize(
    "mutation",
    ["order", "missing", "escape", "count", "identity", "status", "chapter_status"],
)
def test_typed_result_rejects_contradictory_contract(tmp_path, monkeypatch, mutation):
    output = tmp_path / "chapter"
    output.mkdir()
    paths = [output / "0001.png", output / "0002.png"]
    for path in paths:
        path.write_bytes(PNG)
    record = {
        "provider": "mangadex",
        "title": {"id": "title", "title": "Title"},
        "status": "complete",
        "chapters": [
            {
                "id": "chapter",
                "chapter": {"id": "chapter", "number": "1"},
                "status": "complete",
                "expectedPages": 2,
                "outputPath": str(output),
                "imagePaths": [str(path) for path in paths],
            }
        ],
    }
    item = record["chapters"][0]
    if mutation == "order":
        item["imagePaths"].reverse()
    elif mutation == "missing":
        paths[0].unlink()
    elif mutation == "escape":
        outside = tmp_path / "0001.png"
        outside.write_bytes(PNG)
        item["imagePaths"][0] = str(outside)
    elif mutation == "count":
        item["expectedPages"] = 3
    elif mutation == "identity":
        item["chapter"]["id"] = "other"
    elif mutation == "chapter_status":
        item["status"] = "incomplete"
    else:
        record["status"] = "unexpected"
    monkeypatch.setattr(
        "mangate.api._commands.download", lambda *args, **kwargs: record
    )
    with pytest.raises(mangate.MangateError) as caught:
        mangate.download(title_id="title", chapter=1, executable="unused")
    assert caught.value.category == "invalid_result"


@pytest.mark.parametrize(
    "options",
    [
        {"existing_files": "erase"},
        {"output_format": "zip"},
        {"page_downloads": 0},
        {"page_downloads": True},
    ],
)
def test_download_options_validate(options):
    with pytest.raises(ValueError):
        mangate.DownloadOptions(**options)


@pytest.mark.parametrize("category", ["cancelled", "timeout"])
def test_actual_executable_stop_preserves_typed_partial(
    binary, provider, tmp_path, category
):
    provider["block"] = True
    cancel = Event()
    output = tmp_path / "interrupted"
    errors = []

    def run():
        try:
            mangate.download(
                title_id="title-id",
                chapter_id="chapter-id",
                output=output,
                executable=binary,
                cancel_event=cancel,
                timeout=1 if category == "timeout" else 5,
                options=mangate.DownloadOptions(page_downloads=1),
            )
        except Exception as error:
            errors.append(error)

    worker = Thread(target=run)
    worker.start()
    try:
        assert provider["entered"].wait(5), "second page never started"
        assert (output / "0001.png").is_file()
        if category == "cancelled":
            cancel.set()
        worker.join(5)
        assert not worker.is_alive(), "subprocess did not stop"
        assert len(errors) == 1 and isinstance(errors[0], mangate.DownloadError)
        assert errors[0].category == category
        result = errors[0].result
        if result is None and sys.platform == "win32":
            # A headless Windows process cannot emit a graceful signal result.
            state = (
                json.loads((output / ".mangate.json").read_text())
                if (output / ".mangate.json").exists()
                else None
            )
            assert state is None or not state["complete"]
            return
        assert result.status == "cancelled" and not result.complete, result
        assert result.image_paths == (output / "0001.png",)
    finally:
        cancel.set()
        provider["release"].set()
        worker.join(10)


def test_cancelled_download_does_not_resolve_title(tmp_path):
    event = Event()
    event.set()
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download(
            title="Solo Leveling",
            chapter=1,
            executable=tmp_path / "missing",
            cancel_event=event,
        )
    assert caught.value.category == "cancelled"


@pytest.mark.parametrize("complete", [False, True])
@pytest.mark.parametrize("exact", [False, True])
def test_rejected_download_preserves_state_and_pages(
    binary, provider, tmp_path, complete, exact
):
    provider["fail"] = not complete
    arguments = {
        "title_id": "title-id",
        "chapter_id": "chapter-id",
        "executable": binary,
    }
    if exact:
        arguments["output"] = tmp_path / "chapter"
    else:
        # Use Session so ordinary hierarchy stays within the test directory.
        session = mangate.Session(executable=binary, output_dir=tmp_path / "organized")
        arguments.pop("executable")
    download = mangate.download if exact else session.download
    if complete:
        original = download(**arguments)
    else:
        with pytest.raises(mangate.DownloadError) as caught:
            download(**arguments)
        original = caught.value.result
    directory = original.output
    before = {path.name: path.read_bytes() for path in directory.iterdir()}
    provider["fail"] = False
    with pytest.raises(mangate.DownloadError) as caught:
        download(**arguments, options=mangate.DownloadOptions(existing_files="fail"))
    assert caught.value.result.status == "failed"
    assert {path.name: path.read_bytes() for path in directory.iterdir()} == before
    resumed = download(**arguments)
    assert resumed.complete
    assert (directory / "0001.png").read_bytes() == before["0001.png"]
    replaced = download(
        **arguments,
        options=mangate.DownloadOptions(existing_files="replace", output_format="jpeg"),
    )
    assert replaced.complete
    assert [path.suffix for path in replaced.image_paths] == [".jpeg", ".jpeg"]
    assert not list(directory.glob("*.png"))


def test_archive_conversion_does_not_change_original_pages(binary, provider, tmp_path):
    original = mangate.Session(
        executable=binary, output_dir=tmp_path / "organized"
    ).download(title_id="title-id", chapter_id="chapter-id")
    directory = original.output
    before = {path.name: path.read_bytes() for path in directory.iterdir()}
    converted = mangate.archive.convert(directory, executable=binary)
    assert converted.included_pages == 2
    assert {path.name: path.read_bytes() for path in directory.iterdir()} == before


def test_explicit_download_reports_progress(binary, provider, tmp_path):
    events = []
    result = mangate.download(
        title_id="title-id",
        chapter_id="chapter-id",
        output=tmp_path / "progress",
        executable=binary,
        progress=events.append,
    )
    assert result.complete
    assert any(
        event.completed_pages == 2 for event in events if event.stage == "downloading"
    )
    assert events[-1].stage == "completion" and events[-1].status == "complete"


def test_empty_batch_cannot_report_success(monkeypatch):
    monkeypatch.setattr(
        "mangate.api._commands.download",
        lambda *args, **kwargs: {"status": "complete", "chapters": []},
    )
    with pytest.raises(mangate.MangateError) as caught:
        mangate.download_many("title", all_chapters=True, executable="unused")
    assert caught.value.category == "invalid_result"


def test_session_title_and_batch_match_functions(binary, provider, tmp_path):
    provider["chapter_count"] = 2
    session = mangate.Session(
        executable=binary,
        provider="MangaDex",
        timeout=5,
        output_dir=tmp_path / "library",
    )
    assert session.title("title-id") == mangate.title(
        "title-id", executable=binary, timeout=5
    )
    expected = mangate.download_many(
        "title-id",
        chapter_ids=("chapter-id", "chapter-two"),
        executable=binary,
        timeout=5,
        output_dir=tmp_path / "library",
    )
    assert (
        session.download_many("title-id", chapter_ids=("chapter-id", "chapter-two"))
        == expected
    )
    overridden = session.download_many(
        "title-id", first=True, output_dir=tmp_path / "override"
    )
    assert overridden.downloads[0].output.is_relative_to(tmp_path / "override")
    provider["fail"] = True
    for download in (session.download_many, mangate.download_many):
        with pytest.raises(mangate.DownloadError) as caught:
            download(
                "title-id",
                first=True,
                output_dir=tmp_path / "partial-batch",
                **({"executable": binary} if download == mangate.download_many else {}),
            )
        assert caught.value.result.status == "partial"
        assert caught.value.result.downloads[0].pages[0].index == 1


def test_session_title_and_batch_forward_defaults(monkeypatch, tmp_path):
    session = mangate.Session(
        executable="custom", provider="other", timeout=3, output_dir=tmp_path
    )
    calls = []

    def capture(*args, **kwargs):
        calls.append((args, kwargs))
        return "result"

    monkeypatch.setattr(mangate.api, "title", capture)
    monkeypatch.setattr(mangate.api, "download_many", capture)
    assert session.title("stable-id") == "result"
    assert calls[-1] == (
        ("stable-id",),
        {"executable": "custom", "provider": "other", "timeout": 3},
    )
    cancellation = Event()
    assert (
        session.download_many(
            "stable-id", chapter_ids=("chapter",), cancel_event=cancellation
        )
        == "result"
    )
    assert calls[-1][1] == {
        "executable": "custom",
        "provider": "other",
        "timeout": 3,
        "output_dir": str(tmp_path),
        "chapter_ids": ("chapter",),
        "chapter_numbers": (),
        "chapter_range": None,
        "first": False,
        "latest": False,
        "all_chapters": False,
        "assume_yes": False,
        "options": None,
        "progress": None,
        "cancel_event": cancellation,
    }
