# GitHub Actions CI Design

Date: 2026-05-18
Project: `mangate`
Status: Approved design

## Goal

Add a strict GitHub Actions CI baseline for this Go project so pull requests and branch pushes are gated by automated quality checks.

The CI baseline should catch:

- test regressions
- data races and order-dependent tests
- static analysis issues
- module manifest drift
- known Go dependency vulnerabilities

## Current Project Context

This repository is a Go CLI/TUI application with the module path `github.com/evgen2571/mangate`.

Observed local baseline:

- `go.mod` declares `go 1.26`
- `README.md` documents `go test ./...` and `go run ./cmd/mangate`
- there are no existing GitHub Actions workflow files
- local `go test ./...` passes

## Scope

Create one unified CI workflow at:

- `.github/workflows/ci.yml`

The workflow will run on:

- `push`
- `pull_request`

The workflow will define independent jobs for:

- tests
- vet
- tidy consistency
- lint
- vulnerability scanning

## Non-Goals

This design intentionally excludes:

- release automation
- coverage upload/reporting
- Dependabot or Renovate
- CodeQL
- container build or image scanning
- multi-version Go test matrix

These may be added later without changing the core CI contract.

## Workflow Design

### Trigger Policy

The CI workflow should run for both `push` and `pull_request` so developers get feedback on feature branches before and during review.

### Concurrency

The workflow should use GitHub Actions concurrency so new commits on the same branch cancel older in-progress runs. This avoids wasting CI time on superseded revisions.

Expected shape:

- group by workflow name plus git ref
- `cancel-in-progress: true`

### Global Permissions

The workflow should default to least privilege.

Expected top-level permissions:

- `contents: read`

No write-scoped permissions are needed for the baseline jobs in this design.

### Shared Setup

Each job should:

- check out the repository
- install Go `1.26`
- enable module/build caching through `actions/setup-go`

Action versions should use pinned major tags, not floating branches.

## Jobs

### `test`

Purpose:

- validate the repository compiles and tests pass
- detect data races
- catch test interdependence by randomizing execution order

Command:

```bash
go test -race -shuffle=on ./...
```

Notes:

- keep this as a single-package-tree invocation for a small repository
- no coverage artifact is required in this first phase

### `vet`

Purpose:

- run Go's built-in static analysis

Command:

```bash
go vet ./...
```

### `tidy`

Purpose:

- ensure `go.mod` and `go.sum` remain canonical after changes

Commands:

```bash
go mod tidy
git diff --exit-code -- go.mod go.sum
```

Behavior:

- fail if `go mod tidy` would modify tracked module files
- this prevents dependency drift from merging unnoticed

### `lint`

Purpose:

- enforce broader static analysis and style checks than `go vet` alone

Implementation:

- use `golangci/golangci-lint-action`
- rely on the action's bundled runner

Initial policy:

- no custom `.golangci.yml` file unless the default rule set proves noisy
- if the repo exposes actionable lint failures, fix code before weakening the gate

### `vuln`

Purpose:

- detect known vulnerabilities affecting the compiled dependency graph

Commands:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

Notes:

- this is network-dependent in CI but does not require repository secrets
- failures should block merges until reviewed and fixed or intentionally deferred in a future policy change

## Job Independence

Jobs should run independently rather than in a serial pipeline.

Rationale:

- branch protection gets one clear check per gate
- failures are easier to triage
- a lint or vulnerability issue should not hide test outcomes

## Repository Settings Required After Merge

After the workflow is merged, repository settings should be updated in GitHub:

- require pull requests for the main branch
- require passing status checks for all CI jobs
- restrict direct pushes to the protected branch as desired
- keep workflow permissions at least privilege

These settings are not stored in the repository, but they are required for CI to function as an enforcement gate rather than a reporting tool.

## Files To Add Or Modify During Implementation

Expected additions:

- `.github/workflows/ci.yml`

Potential future additions, not part of this implementation:

- `.golangci.yml`
- `.github/dependabot.yml`
- `.github/workflows/codeql.yml`

## Acceptance Criteria

The implementation is complete when:

- `.github/workflows/ci.yml` exists
- the workflow triggers on `push` and `pull_request`
- the workflow uses concurrency cancellation
- the workflow uses least-privilege permissions
- the workflow defines `test`, `vet`, `tidy`, `lint`, and `vuln` jobs
- the test job runs `go test -race -shuffle=on ./...`
- the vet job runs `go vet ./...`
- the tidy job fails on `go.mod` or `go.sum` drift
- the lint job runs `golangci-lint`
- the vuln job runs `govulncheck ./...`
- the workflow syntax is valid YAML

## Risks And Tradeoffs

- `golangci-lint` may surface existing issues that are not currently visible locally. That is acceptable because the user requested stricter gates from the start.
- `govulncheck` can occasionally introduce external signal volatility if vulnerability data changes. This is still appropriate for a strict CI gate.
- using a single Go version keeps the workflow fast and aligned with the repository's declared toolchain, but it does not provide compatibility testing across versions.

## Implementation Boundary

This design covers only repository-committed GitHub Actions workflow code. It does not include post-merge GitHub repository administration beyond documenting the required settings.
