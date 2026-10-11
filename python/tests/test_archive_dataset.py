import json
from dataclasses import FrozenInstanceError

import pytest

import mangate


def test_archive_plan_and_conversion(binary, provider, tmp_path):
    result = mangate.download(
        title_id="title-id",
        chapter_id="chapter-id",
        output=tmp_path / "chapter",
        executable=binary,
    )
    planned = mangate.archive.convert(result.output, dry_run=True, executable=binary)
    assert isinstance(planned, mangate.archive.ConversionPlan)
    assert not planned.output_path.exists()
    converted = mangate.archive.convert(result.output, executable=binary)
    assert converted.included_pages == 2 and converted.validation.valid
    inspected = mangate.archive.inspect(converted.output_path, executable=binary)
    assert inspected.metadata.chapter_id == "chapter-id"
    assert mangate.archive.verify(converted.output_path, executable=binary).valid


@pytest.mark.parametrize("failed", [False, True])
def test_dataset_plan_collect_status_verify_export(binary, provider, tmp_path, failed):
    provider["fail"] = failed
    expected_pages = 0 if failed else 2
    config = tmp_path / "collection.json"
    root = tmp_path / "dataset"
    config.write_text(
        json.dumps(
            {
                "version": 1,
                "datasetId": "fixture",
                "provider": "mangadex",
                "output": {
                    "directory": str(root),
                    "format": "jpeg",
                    "existingFiles": "skip",
                },
                "discovery": {
                    "originalLanguages": ["en"],
                    "chapterLanguages": ["en"],
                    "candidatePoolSize": 1,
                    "orderBy": "updatedAt",
                    "orderDirection": "desc",
                },
                "sampling": {
                    "seed": 1,
                    "titleStrategy": "sequential",
                    "chapterStrategy": "first",
                    "maxTitles": 1,
                    "maxChaptersPerTitle": 1,
                },
                "runtime": {
                    "pageWorkers": 1,
                    "chapterWorkers": 1,
                    "titleWorkers": 1,
                    "validationWorkers": 1,
                },
                "validation": {
                    "minimumWidth": 1,
                    "maximumWidth": 100,
                    "maximumHeight": 100,
                    "maximumDecodedPixels": 10000,
                    "minimumHeight": 1,
                    "fullDecode": True,
                    "calculateSHA256": True,
                },
            }
        )
    )
    options = dict(executable=binary, collection_config=config)
    planned = mangate.dataset.plan(**options)
    assert planned.plan.titles == 1 and planned.plan.chapters == 1
    assert planned.format == "jpeg"
    assert provider["image_calls"] == 0
    collected = mangate.dataset.collect(**options, resume=True)
    assert collected.counters.valid_pages == expected_pages
    assert collected.state == ("partial" if failed else "completed")
    with pytest.raises(FrozenInstanceError):
        collected.state = "changed"
    info = mangate.dataset.status(root, executable=binary)
    assert (
        info.config.dataset_id == "fixture"
        and info.counters.valid_pages == expected_pages
    )
    verification = mangate.dataset.verify(root, executable=binary)
    assert verification.valid is (not failed)
    if failed:
        assert verification.unexpected_files > 0
    exported = mangate.dataset.export(root, executable=binary)
    assert exported.manifest_path.is_file() and exported.summary_path.is_file()


def test_download_many_retains_selection(binary, provider, tmp_path):
    provider["chapter_count"] = 2
    result = mangate.download_many(
        "title-id",
        chapter_ids=("chapter-id", "chapter-two"),
        output_dir=tmp_path,
        executable=binary,
    )
    assert result.status == "complete"
    assert {download.chapter.id for download in result.downloads} == {
        "chapter-id",
        "chapter-two",
    }
    assert all(
        download.complete and len(download.image_paths) == 2
        for download in result.downloads
    )
    assert len(result.downloads[0].image_paths) == 2


def test_archive_source_removal_requires_confirmation(binary, provider, tmp_path):
    chapter = mangate.download(
        title_id="title-id",
        chapter_id="chapter-id",
        output=tmp_path / "source",
        executable=binary,
    )
    before = {path.name: path.read_bytes() for path in chapter.output.iterdir()}
    with pytest.raises(mangate.MangateError):
        mangate.archive.convert(chapter.output, remove_source=True, executable=binary)
    assert {path.name: path.read_bytes() for path in chapter.output.iterdir()} == before


def test_broad_download_selection_can_be_confirmed(binary, provider, tmp_path):
    provider["chapter_count"] = 11
    result = mangate.download_many(
        "title-id",
        all_chapters=True,
        assume_yes=True,
        output_dir=tmp_path,
        executable=binary,
    )
    assert result.status == "complete" and len(result.downloads) == 11
    assert all(len(download.image_paths) == 2 for download in result.downloads)
