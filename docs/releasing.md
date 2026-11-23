# Release Mangate

The version in [`python/pyproject.toml`](../python/pyproject.toml) must match the `vX.Y.Z` tag. The release workflow builds five Go archives and five Python wheels for Linux and macOS (x86-64 and ARM64) and Windows (x86-64).

## One-time setup

- In PyPI, configure a [Trusted Publisher](https://docs.pypi.org/trusted-publishers/adding-a-publisher/) for `evgen2571/mangate`, workflow `release.yml`, environment `pypi`. No PyPI token is needed.
- In GitHub, create the `pypi` environment and restrict it to release tags (`v*`). Protect `main` and release tags against unauthorized updates.

## Rehearse

Manually run the **Release** workflow to build and test all five wheels on their target platforms without publishing. For a local rehearsal, use Go 1.26+, uv, and GoReleaser v2.18.3:

```sh
bash scripts/check.sh
uv run --project python --locked python scripts/release.py build --snapshot
uv run --project python --locked python scripts/release.py validate --archives dist
uv run --project python --locked python scripts/release.py smoke --integration
```

Start with an empty `python/dist`; the build also recreates `dist`. Local smoke tests cover the host platform. Test the TUI in a real Windows terminal before shipping; the automated interactive tests use a POSIX terminal.

## Publish

1. Update the version in `python/pyproject.toml`, run `uv lock --project python`, and merge the tested release commit into `main`.
2. Check that neither GitHub nor PyPI already has the version. Tag that exact commit `vX.Y.Z` and push the tag. Don't move a release tag.
3. Check that all native installation jobs pass, approve the `pypi` environment if required, and verify the published files on both services.

GitHub and PyPI publish separately. If publication stops partway through, inspect the GitHub draft or release and the PyPI files before retrying. Don't overwrite a published version. If the PyPI upload is incomplete and can't safely be finished, release a new version.
