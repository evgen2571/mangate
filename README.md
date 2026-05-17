# Manga Downloader

Terminal manga downloader built around Cobra for CLI entrypoints and Bubble Tea for the interactive TUI.

## Architecture

- `internal/cli`: Cobra commands and entrypoint wiring
- `internal/tui`: presentation and state transitions
- `internal/tuiapp`: TUI-facing application service and UI models
- `internal/usecase`: provider-agnostic application workflows
- `internal/providers`: provider implementations
- `internal/downloader`: chapter and page download execution

## Run

```bash
go test ./...
go run ./cmd/mangate
```
