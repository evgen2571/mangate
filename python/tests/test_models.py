import pytest

import mangate


@pytest.mark.parametrize(
    "values",
    [
        dict(id="", title="Title", provider="mangadex"),
        dict(id=1, title="Title", provider="mangadex"),
        dict(id="id", title=7, provider="mangadex"),
    ],
)
def test_manga_rejects_invalid_identity(values):
    with pytest.raises(ValueError):
        mangate.Manga(**values)


@pytest.mark.parametrize(
    "values",
    [
        dict(id="", provider="mangadex"),
        dict(id="id", provider="mangadex", page_count=-1),
        dict(id="id", provider="mangadex", page_count=True),
    ],
)
def test_chapter_rejects_invalid_identity_and_page_count(values):
    with pytest.raises(ValueError):
        mangate.Chapter(**values)
