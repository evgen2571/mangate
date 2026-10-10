# Releasing Mangate

One stable version (`X.Y.Z`) in `python/pyproject.toml` is used for the Python wheels, bundled executables, Go archives, and GitHub tag `vX.Y.Z`. Source development binaries may report `dev`; release builds stamp the version explicitly. Update the metadata and run `uv lock --project python` before committing a new version.

## First-release checklist

- Confirm you own the PyPI project name `mangate`. If it doesn't exist, add a pending Trusted Publisher in your PyPI account; otherwise configure one in the project's Publishing settings.
- Set its GitHub owner to `evgen2571`, repository to `mangate`, workflow filename to `release.yml`, and environment to `pypi`. These values must exactly match the repository and workflow. No PyPI API token or personal access token is needed. See [PyPI's setup guide](https://docs.pypi.org/trusted-publishers/adding-a-publisher/).
- Create the GitHub environment `pypi`. Restrict deployments to release tags (`v*`) and add required reviewers if your account plan supports them. The workflow requests `id-token: write` only in this job.
- Allow GitHub Actions to run the workflows and allow the GitHub Release job its declared `contents: write` permission. All other jobs use read-only repository permissions.
- Protect the main branch and release tags against unauthorized updates. Keep the checkout, Go toolchain, uv lock, build-hook version, GoReleaser version, and pinned action revisions reviewed.
- Run the non-publishing rehearsal below and inspect all five native installation jobs before pushing the first tag.

## Rehearse without publishing

Install Go 1.26+, uv, and GoReleaser **v2.18.3**. From the repository root:

```sh
bash scripts/check.sh
go test -race ./internal/downloader ./internal/cache ./internal/providers/... ./internal/usecase ./internal/interactive ./internal/cli ./internal/archive
uv run --project python --locked python scripts/release.py build --snapshot
uv run --project python --locked python scripts/release.py validate --archives dist
uv run --project python --locked python scripts/release.py smoke --integration
```

Use an empty `python/dist` for each build (move or remove your previous generated artifacts first). GoReleaser owns `dist` and recreates it with `--clean`. Builds create five Go archives, five `py3-none` platform wheels, Go SHA-256 checksums, and combined `SHA256SUMS`. The validator checks platform tags, wheel records/metadata, binary architecture, static Linux linking, archive checksums, and byte-for-byte equality of archived and bundled binaries. The installation check uses a clean environment and checks both `Client()` and the `mangate` console launcher. Local runtime checks cover the host platform; the workflow tests each wheel on its native OS and architecture.

CI runs the shared `release-build.yml` without publishing. You can also choose **Run workflow** on the Release workflow for a complete rehearsal with release-critical checks and all native installation jobs. A manual dispatch never enables either publishing job.

## Publish

1. Update the Python version, update the lockfile, and merge the tested release commit to `main`.
2. Confirm PyPI has not already published this version and GitHub has no release/draft for this tag.
3. Tag that exact clean main-branch commit with `vX.Y.Z`, then push the tag. Don't move an existing release tag.
4. Review the validated artifacts, approve the `pypi` environment if required, and verify the GitHub Release and PyPI files after publishing.

The tag must be a stable `vX.Y.Z`, match Python metadata, point at the checked-out commit, and belong to `origin/main`. Release-critical checks run before builds. All five native installations and subprocess tests must pass before publication. An existing GitHub Release is rejected; uploads first go to a draft, which is made public only after every asset uploads. PyPI uploads use Trusted Publishing and do not skip existing files. Wheels are the only Python distributions published; they include the binary and need no Go installation. The macOS baseline is 12.0; Linux executables are static and wheels advertise glibc 2.17+ and musl 1.2+.

GitHub and PyPI cannot be published as one transaction. A registry/network failure can leave a published GitHub Release or some PyPI files. Inspect an interrupted release manually; do not rerun blindly or overwrite published versions. An interrupted GitHub upload leaves a draft for inspection. Use a fresh version if PyPI has accepted an incomplete set and you cannot safely finish that upload.

POSIX PTY tests cover completion, cancellation, timeout, interrupted transfers, partial downloads, and terminal restoration with a local provider fixture. Windows runs executable/API/installation tests, but the POSIX fixture is skipped there; a real Windows terminal/ConPTY smoke test remains a manual release check. Nothing in these workflows signs or notarizes macOS/Windows executables.
