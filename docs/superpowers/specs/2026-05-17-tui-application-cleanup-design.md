# TUI Application Boundary Cleanup Design

Date: 2026-05-17
Project: `mangate`
Scope: Architecture cleanup with TUI-first focus

## Goal

Clean up the project by making `internal/tui` a presentation layer instead of an application workflow layer.

The primary objective is to move search, chapter loading, cover loading, download orchestration, and config apply/save workflows out of the TUI and behind a dedicated TUI-facing application layer.

This cleanup is driven by three priorities, in order:

1. Architecture cleanup
2. UX cleanup
3. Repo polish

CLI unification is explicitly out of scope for this pass. The CLI may continue using the existing `app/usecase` wiring directly.

## Current Problems

The current structure is already reasonably modular, but the TUI still owns responsibilities that should live outside presentation code.

Observed issues:

1. `internal/tui` calls `a.UseCases()` directly and coordinates application workflows itself.
2. `internal/tui` depends directly on `internal/source` and, through commands, on `internal/usecase`.
3. TUI code mutates loaded domain objects and combines UI state with domain workflow state.
4. Config apply/save behavior is triggered directly from the TUI against `app.App`.
5. Async workflow coordination for download progress and cover loading lives in the TUI command layer.

This makes the TUI harder to reason about, harder to test cleanly, and more resistant to UX changes because screen code is coupled to lower-level application structure.

## Design Summary

Introduce a dedicated TUI-facing application layer between `internal/tui` and the current `internal/app` + `internal/usecase` internals.

After the cleanup:

1. `internal/tui` should depend on a single application interface tailored to its needs.
2. `internal/tui` should work with UI-oriented data types rather than `source.Manga` and `source.Chapter`.
3. Workflow orchestration should move out of TUI commands and into the new application layer.
4. Existing provider, downloader, cache, and usecase internals should be reused rather than redesigned wholesale.

This is a deep cleanup of the TUI boundary, not a full domain rewrite.

## Target Architecture

### Layers

1. `internal/tui`
   - rendering
   - screen state
   - input handling
   - UI-local selection state
   - state transitions between screens

2. New TUI application layer
   - TUI-facing service interface
   - command/query execution for TUI workflows
   - mapping from domain objects to UI-facing models
   - config apply/save coordination
   - download progress translation
   - cover loading coordination

3. Existing lower layers
   - `internal/app`
   - `internal/usecase`
   - `internal/cache`
   - `internal/downloader`
   - `internal/providers`
   - `internal/source`

### Dependency Direction

Target direction:

`tui -> tuiapp interface -> application/service implementation -> app/usecase/downloader/cache/providers`

Forbidden direction after cleanup:

1. `internal/tui -> internal/source`
2. `internal/tui -> internal/usecase`
3. `internal/tui -> app.App` for direct workflow/config operations

The TUI may still receive a top-level dependency at construction time, but that dependency should be the new TUI-facing interface, not the mutable `app.App` container.

## Proposed New Boundary

Create a new package dedicated to TUI-facing workflow coordination. Name can be finalized during implementation, but it should reflect purpose clearly. Preferred candidates:

1. `internal/tuiapp`
2. `internal/app/tui`

Recommendation: `internal/tuiapp`

Reasoning:

1. It gives the boundary a clear identity.
2. It avoids making `internal/app` even more overloaded.
3. It keeps the TUI-specific application contract explicit.

## TUI-Facing Interface

The new package should expose a single interface consumed by `internal/tui`.

Representative shape:

```go
type Service interface {
	Search(context.Context, string) ([]SearchResult, error)
	LoadChapters(context.Context, string) (MangaDetails, []ChapterItem, error)
	LoadCover(context.Context, string, CoverSize) (CoverResult, error)
	Download(context.Context, DownloadRequest, func(DownloadProgress)) error
	Config() ConfigState
	ApplyConfig(context.Context, ConfigState) (ConfigState, error)
	SaveConfig(context.Context, ConfigState) (ConfigState, error)
	SearchHistory(context.Context) ([]string, error)
}
```

The exact method set may change during planning, but the core rule is stable:

The TUI talks to one purpose-built service instead of composing lower-level pieces itself.

## TUI-Facing Data Types

The new layer should define stable presentation-facing models. These should be plain structs, not abstractions for their own sake.

Representative types:

1. `SearchResult`
2. `MangaDetails`
3. `ChapterItem`
4. `DownloadRequest`
5. `DownloadProgress`
6. `CoverSize`
7. `CoverResult`
8. `ConfigState`

Expected characteristics:

1. Fields are chosen for UI needs, not provider completeness.
2. IDs required for later operations remain available.
3. Display-ready values can be precomputed where that reduces UI logic.
4. The types should not expose `source.Manga`, `source.Chapter`, or `usecase` types.

## Workflow Ownership After Cleanup

### Search

Current:
TUI issues search requests directly through `UseCases()`.

Target:
TUI submits a query to the new service and receives `[]SearchResult`.

The service owns:

1. calling the underlying search use case
2. recording search history where appropriate
3. mapping results into UI-facing models

### Chapters

Current:
TUI loads chapters from domain objects and manages full-manga download setup around those objects.

Target:
TUI requests chapters using a stable manga identifier or selection object and receives `MangaDetails` plus `[]ChapterItem`.

The service owns:

1. loading chapters
2. associating them with the selected manga
3. preparing the UI-facing data needed for selection and display

### Cover Loading

Current:
TUI fetches a cover path and then renders the terminal cover text itself using domain-oriented inputs.

Target:
TUI asks for cover data using a stable manga identifier and desired size information.

The service owns:

1. cover lookup coordination
2. cache interaction
3. mapping failures into operation-level errors

Cover text rendering may remain in `internal/tui` if it is strictly presentation logic. The service boundary only needs to ensure that cover retrieval is not coupled to domain objects in the TUI.

### Downloads

Current:
TUI builds download flows, owns progress-channel lifecycle decisions, and translates progress from usecase models.

Target:
TUI submits a `DownloadRequest` and receives progress callbacks in TUI-facing `DownloadProgress` form.

The service owns:

1. validating the request
2. resolving the target manga/chapters
3. invoking the underlying download use case
4. converting lower-level progress models into UI-facing progress events

### Config

Current:
TUI calls `ApplyConfig` and `ApplyAndSaveConfig` on `app.App`.

Target:
TUI reads and mutates configuration only through the new service.

The service owns:

1. retrieving current config state
2. validation
3. applying session-only changes
4. persisting changes
5. returning the normalized config state back to the TUI

## Role of `app.App`

`app.App` should stop being the primary thing the TUI manipulates.

Desired direction:

1. `app.App` may remain as a composition root and lifecycle holder for now.
2. `app.App` may be used to construct the new TUI-facing service.
3. TUI code should not depend on `app.App` methods for day-to-day workflows.

This keeps the current wiring usable without forcing a larger rewrite in the same pass.

## UX Impact

This architecture cleanup is expected to improve UX work in practical ways:

1. screen models can evolve without carrying domain baggage
2. user-facing names and status text can be owned closer to the TUI boundary
3. operations such as full download, chapter selection, and cover refresh become easier to reason about
4. error handling can be normalized for TUI interactions instead of leaking lower-level details directly

The goal is not a visual redesign. The goal is making the UX easier to improve because the UI is no longer acting as the application coordinator.

## Repo Cleanup Included In Scope

Repo cleanup in this pass should be narrow and architecture-driven.

Included:

1. moving orchestration code out of `internal/tui`
2. renaming files, packages, or methods where needed to make the new boundary clear
3. reducing direct domain coupling in TUI files
4. updating or replacing minimal docs so the new structure is understandable

Excluded:

1. unrelated package reshuffles
2. broad style-only cleanup
3. CLI redesign
4. provider redesign
5. downloader redesign
6. large-scale domain model rework outside what the new TUI boundary requires

## Migration Strategy

Implement incrementally rather than rewriting the TUI in one move.

Suggested sequence:

1. define the new TUI-facing interface and data types
2. add an implementation that adapts current `app/usecase` behavior
3. route one workflow at a time through the new boundary
4. remove direct `UseCases()` calls from TUI code
5. remove direct `source.*` dependencies from TUI models and messages
6. tighten tests around the new boundary
7. do final naming/doc cleanup once the structure settles

This keeps the system running while the boundary moves.

## Testing Strategy

### New Tests

Add focused tests for the new TUI-facing application layer:

1. search result mapping
2. chapter loading and mapping
3. download request validation
4. progress translation
5. config apply/save behavior
6. cover-loading coordination

### TUI Tests

Retain TUI tests, but narrow their purpose:

1. state transitions
2. key handling
3. selection behavior
4. status messaging
5. rendering-sensitive behavior where already covered

TUI tests should stop needing provider/downloader/usecase knowledge wherever possible.

### Existing Lower-Layer Tests

Keep existing usecase/provider/downloader tests unless a boundary change requires small adaptation. Do not rewrite those tests unless they block the cleanup.

## Success Criteria

This cleanup is successful when all of the following are true:

1. `internal/tui` no longer imports `internal/source`
2. `internal/tui` no longer imports `internal/usecase`
3. TUI workflows run through a single TUI-facing service interface
4. search, chapters, cover, download, and config workflows are owned outside TUI
5. TUI tests are more presentation-focused than workflow-focused
6. the codebase documents the new boundary clearly enough for future work

## Risks

1. Over-designing the new boundary and creating unnecessary indirection
2. Moving too much at once and breaking TUI behavior
3. Duplicating domain and UI structures in a way that creates confusion
4. Letting the new service grow into another oversized catch-all

Mitigations:

1. keep the new interface limited to actual TUI workflows
2. migrate incrementally
3. keep UI-facing types plain and purpose-specific
4. leave CLI and broader domain refactoring out of scope

## Open Decisions For Planning

These do not block the design, but they should be finalized in the implementation plan:

1. final package location and name for the new TUI-facing application layer
2. exact IDs and data carried by `SearchResult`, `MangaDetails`, and `ChapterItem`
3. whether cover rendering text generation stays entirely in TUI or gets a thin helper boundary
4. whether search history should be folded into the same service interface or a narrow auxiliary interface

## Recommendation

Proceed with a TUI-first application boundary cleanup centered on a new TUI-facing service package, using incremental migration and explicit UI-facing models.

This gives the project a cleaner architectural center without expanding the effort into a full-system rewrite.
