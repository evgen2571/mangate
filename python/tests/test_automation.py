import json
import subprocess
from threading import Event, Thread

import pytest

import mangate


def test_random_plan_replay_and_portable_manifest(binary, provider, tmp_path):
    provider["original_language"] = "ko"
    events = []
    output = tmp_path / "exact"
    source = tmp_path / "source.json"
    policy = mangate.SelectionPolicy(
        original_language="ko",
        chapter_language="en",
        min_pages=2,
        max_pages=2,
        chapter_min=1,
        chapter_max=1,
    )
    plan = mangate.plan_random(
        output,
        policy=policy,
        executable=binary,
        manifest=source,
        progress=events.append,
    )
    assert isinstance(plan, mangate.DownloadPlan)
    assert provider["image_calls"] == 0 and not output.exists()
    assert events[0].stage == "discovery"
    assert events[-1].stage == "completion" and events[-1].status == "planned"
    assert plan.expected_pages == 2
    loaded = mangate.load_manifest(source)
    assert loaded.plan == plan and loaded.status == "planned"
    calls = provider["random_calls"]
    events.clear()
    destination = tmp_path / "result.json"
    result = mangate.execute(
        loaded, executable=binary, manifest=destination, progress=events.append
    )
    assert isinstance(result, mangate.DownloadResult) and result.complete
    assert result.image_paths == (output / "0001.png", output / "0002.png")
    assert provider["random_calls"] == calls
    assert events[0].stage == "validation" and events[-1].status == "complete"
    assert [e.completed_pages for e in events if e.status == "progress"] == [1, 2]
    raw = json.loads(destination.read_text())
    assert raw["plan"]["output"] == "exact"
    assert raw["images"][0]["path"] == "0001.png"
    assert raw["images"][0]["format"] == "png"
    assert raw["images"][0]["index"] == 1
    assert (
        "baseUrl" not in destination.read_text()
        and "hash" not in destination.read_text()
    )
    assert mangate.load_manifest(destination) == result.manifest
    assert mangate.verify_manifest(destination, executable=binary).status == "complete"
    # Both Python-created plans and Go-created manifests replay the same source.
    copy = tmp_path / "copy.json"
    mangate.save_manifest(plan, copy)
    assert mangate.load_manifest(copy).plan == plan
    relocated = mangate.execute(copy, output=tmp_path / "replay", executable=binary)
    assert relocated.complete and provider["random_calls"] == calls
    (output / "0001.png").write_bytes(b"invalid")
    with pytest.raises(mangate.MangateError):
        mangate.verify_manifest(destination, executable=binary)


def test_one_call_random_and_session(binary, provider, tmp_path):
    result = mangate.download_random(
        tmp_path / "one",
        original_language="en",
        chapter_language="en",
        min_pages=2,
        max_attempts=2,
        executable=binary,
    )
    assert result.complete and result.manifest.status == "complete"
    session = mangate.Session(binary, output_dir=tmp_path / "session")
    plan = session.plan_random(min_pages=2)
    assert session.execute(plan).complete
    assert session.download_random(tmp_path / "session-random").complete


@pytest.mark.parametrize(
    "mutation,fields",
    [
        ({"original_language": "ko"}, {"original_language": "en"}),
        ({"original_language": ""}, {"original_language": "ko"}),
        ({"chapter_language": ""}, {"chapter_language": "en"}),
        ({"page_count": 1}, {"min_pages": 2}),
        ({"page_count": 3}, {"min_pages": 2}),
        ({"unavailable": True}, {}),
        ({}, {"max_pages": 1}),
        ({}, {"chapter_min": 2}),
    ],
)
def test_bounded_rejection_never_downloads(
    binary, provider, tmp_path, mutation, fields
):
    provider.update(mutation)
    events = []
    with pytest.raises(mangate.DownloadError):
        mangate.download_random(
            tmp_path / "no-files",
            executable=binary,
            max_attempts=2,
            progress=events.append,
            **fields,
        )
    assert provider["random_calls"] == 2
    assert provider["image_calls"] == 0 and not (tmp_path / "no-files").exists()
    assert any(e.reason == "duplicate candidate" for e in events)
    assert events[-1].stage == "completion" and events[-1].status == "failed"


def test_unknown_page_count_resolved_and_seeded_snapshot(binary, provider, tmp_path):
    provider["page_count"] = 0
    policy = mangate.SelectionPolicy(seed=42, min_pages=2)
    first = mangate.plan_random(tmp_path / "first", policy=policy, executable=binary)
    second = mangate.plan_random(tmp_path / "first", policy=policy, executable=binary)
    assert first == second
    assert first.strategy == "seeded_catalog" and first.candidate_ids == ("title-id",)
    assert first.expected_pages == 2 and first.chapter.page_count == 2
    assert provider["random_calls"] == 0
    assert mangate.execute(first, executable=binary).complete


def test_plan_metadata_changes_and_unavailable_selection(binary, provider, tmp_path):
    plan = mangate.plan_random(tmp_path / "out", executable=binary)
    provider["chapter_language"] = "ko"
    with pytest.raises(mangate.DownloadError, match="metadata changed") as caught:
        mangate.execute(plan, executable=binary)
    assert caught.value.result.status == "failed"
    assert provider["image_calls"] == 0
    provider["chapter_language"] = "en"
    provider["page_count"] = 3
    with pytest.raises(mangate.DownloadError):
        mangate.execute(plan, executable=binary)
    provider["page_count"] = 2
    provider["unavailable"] = True
    with pytest.raises(mangate.DownloadError):
        mangate.execute(plan, executable=binary)
    assert not (tmp_path / "out").exists()


def test_partial_download_manifest_and_replay(binary, provider, tmp_path):
    provider["fail"] = True
    path = tmp_path / "partial.json"
    events = []
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download_random(
            tmp_path / "partial",
            executable=binary,
            manifest=path,
            progress=events.append,
        )
    result = caught.value.result
    assert isinstance(result, mangate.DownloadResult) and result.status == "partial"
    assert result.image_paths == (tmp_path / "partial" / "0001.png",)
    assert result.manifest == mangate.load_manifest(path)
    assert (
        result.manifest.actual_pages == 1 and result.manifest.validation == "incomplete"
    )
    assert result.manifest.error and events[-1].status == "partial"
    calls = provider["random_calls"]
    provider["fail"] = False
    assert mangate.execute(path, executable=binary).complete
    assert provider["random_calls"] == calls


@pytest.mark.parametrize("stage", ["/manga/random", "/manga/title-id/feed"])
def test_cancellation_during_discovery_and_selection(binary, provider, tmp_path, stage):
    provider["block_stage"] = stage
    cancel = Event()
    events = []
    errors = []

    def run():
        try:
            mangate.download_random(
                tmp_path / "cancelled",
                executable=binary,
                cancel_event=cancel,
                progress=events.append,
            )
        except mangate.DownloadError as error:
            errors.append(error)

    thread = Thread(target=run)
    thread.start()
    assert provider["entered"].wait(5)
    cancel.set()
    thread.join(5)
    assert not thread.is_alive() and errors[0].category == "cancelled"
    assert events[-1].stage == "completion" and events[-1].status == "cancelled"
    assert not (tmp_path / "cancelled").exists()


def test_progress_callback_cancels_and_preserves_partial(binary, provider, tmp_path):
    provider["block"] = True
    cancel = Event()
    events = []

    def progress(event):
        events.append(event)
        if event.stage == "downloading" and event.completed_pages == 1:
            cancel.set()

    path = tmp_path / "cancelled.json"
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download_random(
            tmp_path / "cancelled",
            executable=binary,
            cancel_event=cancel,
            progress=progress,
            manifest=path,
        )
    assert caught.value.category == "cancelled"
    assert caught.value.result.status == "cancelled"
    assert caught.value.result.image_paths == (tmp_path / "cancelled" / "0001.png",)
    assert mangate.load_manifest(path).status == "cancelled"
    assert events[-1].status == "cancelled"


def test_timeout_and_callback_exception_cleanup(binary, provider, tmp_path):
    provider["block_stage"] = "/manga/random"
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download_random(tmp_path / "timeout", executable=binary, timeout=0.3)
    assert caught.value.category == "timeout"
    provider["block_stage"] = ""

    def broken_callback(event):
        raise RuntimeError("callback failed")

    with pytest.raises(RuntimeError, match="callback failed"):
        mangate.plan_random(
            tmp_path / "callback", executable=binary, progress=broken_callback
        )


@pytest.mark.parametrize(
    "fields",
    [
        {"min_pages": 0},
        {"min_pages": None},
        {"max_attempts": None},
        {"max_pages": 0},
        {"max_pages": 1, "min_pages": 2},
        {"max_attempts": 0},
        {"max_attempts": True},
        {"chapter_min": float("nan")},
        {"chapter_min": 3, "chapter_max": 2},
        {"seed": 2**64},
        {"seed": True},
        {"original_language": "EN"},
    ],
)
def test_invalid_policy_before_execution(fields):
    with pytest.raises(ValueError):
        mangate.SelectionPolicy(**fields)


@pytest.mark.parametrize(
    "mutation",
    [
        lambda m: m.update(version=99),
        lambda m: m.update(actualPages=1),
        lambda m: m["plan"].update(expectedPages=3),
        lambda m: m["plan"].update(strategy="magic"),
        lambda m: m["plan"]["policy"].update(minPages=4),
        lambda m: m["plan"]["chapter"].update(language="ko"),
        lambda m: m.update(
            images=[{"index": 1, "path": "../0001.png", "format": "png"}]
        ),
    ],
)
def test_malformed_manifests_python_and_cli(binary, provider, tmp_path, mutation):
    path = tmp_path / "bad.json"
    mangate.plan_random(
        tmp_path / "out", chapter_language="en", manifest=path, executable=binary
    )
    data = json.loads(path.read_text())
    mutation(data)
    path.write_text(json.dumps(data))
    with pytest.raises(mangate.MangateError):
        mangate.load_manifest(path)
    result = subprocess.run(
        [str(binary), "--json", "download", "--from-manifest", str(path)],
        text=True,
        capture_output=True,
        timeout=5,
    )
    assert result.returncode and json.loads(result.stdout)["status"] == "error"
    assert provider["image_calls"] == 0


def test_jsonl_and_single_response_contract(binary, provider, tmp_path):
    args = [
        str(binary),
        "random",
        "--min-pages",
        "2",
        "--output",
        str(tmp_path / "out"),
    ]
    streamed = subprocess.run(
        args + ["--jsonl"], text=True, capture_output=True, timeout=10
    )
    assert streamed.returncode == 0, streamed.stderr
    lines = [json.loads(line) for line in streamed.stdout.splitlines()]
    assert lines[0]["stage"] == "discovery"
    assert lines[-2] == {"stage": "completion", "status": "complete"}
    assert (
        lines[-1]["operation"] == "download"
        and lines[-1]["data"]["status"] == "complete"
    )
    single = subprocess.run(
        args + ["--json"], text=True, capture_output=True, timeout=10
    )
    assert single.returncode == 0 and len(single.stdout.splitlines()) == 1
    assert json.loads(single.stdout)["status"] == "success"
    failure = subprocess.run(
        args + ["--jsonl", "--original-language", "ko", "--max-attempts", "1"],
        text=True,
        capture_output=True,
        timeout=10,
    )
    lines = [json.loads(line) for line in failure.stdout.splitlines()]
    assert (
        failure.returncode
        and lines[-2]["status"] == "failed"
        and lines[-1]["status"] == "error"
    )


def test_replay_unavailable_chapter_and_changed_page_list(binary, provider, tmp_path):
    plan = mangate.plan_random(tmp_path / "out", executable=binary)
    provider["missing_chapter"] = True
    with pytest.raises(mangate.DownloadError, match="unavailable"):
        mangate.execute(plan, executable=binary)
    provider["missing_chapter"] = False
    provider["actual_pages"] = 3
    with pytest.raises(mangate.DownloadError, match="page count changed"):
        mangate.execute(plan, executable=binary)
    assert provider["image_calls"] == 0 and not (tmp_path / "out").exists()


def test_existing_policies_and_manifest_destination(binary, provider, tmp_path):
    output = tmp_path / "out"
    result = mangate.download_random(output, executable=binary)
    calls = provider["image_calls"]
    assert mangate.execute(result.manifest, executable=binary).complete
    assert provider["image_calls"] == calls
    with pytest.raises(mangate.DownloadError):
        mangate.execute(
            result.manifest,
            executable=binary,
            options=mangate.DownloadOptions(existing_files="fail"),
        )
    assert provider["image_calls"] == calls
    assert mangate.execute(
        result.manifest,
        executable=binary,
        options=mangate.DownloadOptions(existing_files="replace"),
    ).complete
    assert provider["image_calls"] == calls + 2
    before = (output / ".mangate.json").read_bytes()
    with pytest.raises(mangate.DownloadError, match="outside"):
        mangate.download_random(
            output, executable=binary, manifest=output / ".mangate.json"
        )
    assert (output / ".mangate.json").read_bytes() == before
    with pytest.raises(ValueError, match="outside"):
        mangate.save_manifest(result.manifest, output / "0001.png")


def test_invalid_downloaded_image_keeps_incomplete_manifest(binary, provider, tmp_path):
    provider["invalid_image"] = True
    path = tmp_path / "invalid.json"
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download_random(tmp_path / "out", executable=binary, manifest=path)
    assert caught.value.result.status == "partial"
    manifest = mangate.load_manifest(path)
    assert manifest.validation == "incomplete" and manifest.actual_pages == 2
    assert all(item.format == "unknown" for item in manifest.images)
    assert "validation failed" in manifest.error
    with pytest.raises(mangate.MangateError):
        mangate.verify_manifest(path, executable=binary)


def test_unknown_random_provider_cli(binary, tmp_path):
    result = subprocess.run(
        [
            str(binary),
            "--json",
            "--provider",
            "example",
            "random",
            "--output",
            str(tmp_path / "out"),
        ],
        text=True,
        capture_output=True,
        timeout=5,
    )
    assert result.returncode != 0
    assert json.loads(result.stdout)["data"]["category"] == "unknown_provider"
    assert not (tmp_path / "out").exists()


def test_refresh_cannot_bypass_policy_or_discard_partial_paths(
    binary, provider, tmp_path
):
    provider["refresh_changes_count"] = True
    path = tmp_path / "refresh.json"
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.download_random(
            tmp_path / "out", min_pages=2, max_pages=2, executable=binary, manifest=path
        )
    assert "page count changed" in caught.value.message
    result = caught.value.result
    assert result.status == "partial" and result.image_paths == (
        tmp_path / "out" / "0001.png",
    )
    assert result.manifest == mangate.load_manifest(path)
    assert result.expected_pages == 2 and result.manifest.actual_pages == 1


@pytest.mark.parametrize("mutation", ["image", "complete", "count", "missing"])
def test_local_manifest_integrity(binary, provider, tmp_path, mutation):
    path = tmp_path / "manifest.json"
    output = tmp_path / "out"
    result = mangate.download_random(output, executable=binary, manifest=path)
    calls = (provider["random_calls"], provider["title_calls"], provider["image_calls"])
    assert result.manifest.version == 2
    raw = json.loads(path.read_text())
    import hashlib

    for item in raw["images"]:
        content = (output / item["path"]).read_bytes()
        assert item["bytes"] == len(content)
        assert item["sha256"] == hashlib.sha256(content).hexdigest()
    if mutation == "image":
        # Keep the PNG valid: format-only verification used to accept this.
        with (output / "0001.png").open("ab") as stream:
            stream.write(b"modified")
    elif mutation == "missing":
        (output / "0002.png").unlink()
    else:
        state_path = output / ".mangate.json"
        state = json.loads(state_path.read_text())
        state["complete" if mutation == "complete" else "expectedPages"] = (
            False if mutation == "complete" else 3
        )
        state_path.write_text(json.dumps(state))
    with pytest.raises(mangate.MangateError):
        mangate.verify_manifest(path, executable=binary)
    assert calls == (
        provider["random_calls"],
        provider["title_calls"],
        provider["image_calls"],
    )


def test_legacy_manifest_explicit_replay(binary, provider, tmp_path):
    path = tmp_path / "legacy.json"
    mangate.download_random(tmp_path / "out", executable=binary, manifest=path)
    data = json.loads(path.read_text())
    data["version"] = 1
    for item in data["images"]:
        del item["bytes"], item["sha256"]
    path.write_text(json.dumps(data))
    loaded = mangate.load_manifest(path)
    assert loaded.version == 1
    copy = tmp_path / "legacy-copy.json"
    mangate.save_manifest(loaded, copy)
    assert mangate.load_manifest(copy) == loaded
    with pytest.raises(mangate.MangateError, match="version 1 has no image checksums"):
        mangate.verify_manifest(copy, executable=binary)
    assert mangate.execute(copy, executable=binary).manifest.version == 2


@pytest.mark.skipif(
    __import__("os").name != "posix", reason="requires POSIX file-size limits"
)
def test_event_file_write_failure_preserves_download(binary, provider, tmp_path):
    import resource

    output = tmp_path / "out"
    events = tmp_path / "events.jsonl"

    def limit_file_size():
        resource.setrlimit(resource.RLIMIT_FSIZE, (700, 700))

    result = subprocess.run(
        [
            str(binary),
            "--json",
            "random",
            "--output",
            str(output),
            "--events-file",
            str(events),
        ],
        text=True,
        capture_output=True,
        timeout=10,
        preexec_fn=limit_file_size,
    )
    assert result.returncode != 0
    record = json.loads(result.stdout)
    assert record["status"] == "partial"
    assert "progress" in record["data"]["error"]
    assert record["data"]["manifest"]["status"] == "complete"
    assert (output / "0001.png").is_file() and (output / "0002.png").is_file()
    assert events.read_bytes().startswith(b'{"stage":"discovery"')


def test_failed_acquisition_and_manifest_write_keeps_failed_status(
    binary, provider, tmp_path
):
    plan = mangate.plan_random(tmp_path / "out", executable=binary)
    provider["missing_chapter"] = True
    with pytest.raises(mangate.DownloadError) as caught:
        mangate.execute(
            plan, executable=binary, manifest=tmp_path / "missing" / "result.json"
        )
    assert caught.value.result.status == "failed"
    assert caught.value.result.manifest.status == "failed"
    source = tmp_path / "plan.json"
    mangate.save_manifest(plan, source)
    result = subprocess.run(
        [
            str(binary),
            "--json",
            "download",
            "--from-manifest",
            str(source),
            "--manifest",
            str(tmp_path / "missing" / "result.json"),
        ],
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert json.loads(result.stdout)["data"]["status"] == "failed"


@pytest.mark.parametrize("value", [True, -1, 2**63, "70"])
def test_manifest_byte_size_matches_go_contract(binary, provider, tmp_path, value):
    path = tmp_path / "manifest.json"
    mangate.download_random(tmp_path / "out", executable=binary, manifest=path)
    data = json.loads(path.read_text())
    data["images"][0]["bytes"] = value
    path.write_text(json.dumps(data))
    with pytest.raises(mangate.MangateError):
        mangate.load_manifest(path)


@pytest.mark.parametrize("mode", ["--json", "--jsonl"])
def test_unavailable_events_destination_has_error_envelope(
    binary, provider, tmp_path, mode
):
    result = subprocess.run(
        [
            str(binary),
            "random",
            mode,
            "--plan",
            "--output",
            str(tmp_path / "out"),
            "--events-file",
            str(tmp_path / "missing" / "events.jsonl"),
        ],
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode != 0
    assert json.loads(result.stdout)["status"] == "error"
    assert provider["random_calls"] == 0


def test_random_empty_array_description_downloads_ordered_images(
    binary, provider, tmp_path
):
    provider["description"] = []
    provider["original_language"] = "ko"
    result = mangate.download_random(
        output=tmp_path / "chapter",
        original_language="ko",
        chapter_language="en",
        min_pages=2,
        max_attempts=1,
        executable=binary,
    )
    assert result.complete
    assert result.manga.id == "title-id" and result.chapter.id == "chapter-id"
    assert result.image_paths == (
        tmp_path / "chapter" / "0001.png",
        tmp_path / "chapter" / "0002.png",
    )
    assert all(path.read_bytes() for path in result.image_paths)
    assert provider["random_calls"] == 1 and provider["image_calls"] == 2
