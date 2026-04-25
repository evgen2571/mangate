# Mangate roadmap

This roadmap reflects the current `develop` branch. The old checklist mixed completed MVP work with stale items, so this file now tracks the remaining high-value work.

## Done / baseline

- [x] MangaDex provider: search, chapters, covers, pages
- [x] Language-aware MangaDex chapter filtering
- [x] MangaDex chapter pagination
- [x] MangaDex@Home page URLs use the returned `baseUrl`
- [x] Lazy page loading during downloads to avoid MangaDex@Home request bursts
- [x] Rate-limit retries for page downloads
- [x] Stable TUI download progress with per-chapter rows
- [x] Config loading from user config path with runtime application
- [x] Plain downloads preserve provider-native image file types
- [x] Converter package and safe replacement behavior
- [x] TUI search/results/chapters/download flow
- [x] CLI `search` and `config` commands

## Next priority: CLI parity with TUI

- [x] Add `mangate chapters <manga-id>` to list chapters without launching the TUI
- [ ] Add `mangate download <manga-id> --chapters <selector>` for non-interactive downloads
- [ ] Support chapter selectors by index/range/ID, for example `1`, `1,3,5`, `1-10`, `all`
- [ ] Print download progress in CLI mode without Bubble Tea
- [ ] Add CLI smoke tests for command wiring and output formatting

## Config and install ergonomics

- [ ] Add `mangate config path`
- [ ] Add `mangate config init` to write default config only when explicitly requested
- [ ] Add safe config update commands or documented examples for common edits
- [ ] Keep `scripts/install.sh` focused on install/build and delegate default config generation to Go code
- [ ] Document `MANGATE_CONFIG` and common flags in README

## Download formats and metadata

- [ ] Verify and finish `zip` output format
- [ ] Verify and finish `cbz` output format
- [ ] Add manga/chapter metadata files beside downloads where useful
- [ ] Preserve stable page ordering across all output formats
- [ ] Add regression tests for format replacement and partial failure cleanup

## TUI improvements

- [ ] Add a manga details screen before opening chapters
- [ ] Show description, alt titles, status/year/tags when provider metadata supports them
- [ ] Add a provider/config screen only after CLI config commands are stable
- [ ] Add clearer empty/error states for network failures
- [ ] Keep the current Bubble help style unless a deliberate redesign is chosen

## Provider architecture

- [ ] Keep provider-layer chapter titles raw; presentation fallback belongs in UI/CLI/downloader naming
- [ ] Cache MangaDex@Home metadata in-memory for the short validity window to reduce duplicate page metadata calls within one run
- [ ] Consider a dedicated low-rate budget for MangaDex@Home metadata requests if large downloads still hit endpoint limits
- [ ] Add another provider only after CLI download and config UX are reliable

## Quality gates

Run before commits:

```bash
gofmt -w <changed Go files>
go test ./...
go vet ./...
```

For config/script work, also run:

```bash
go run ./cmd/mangate config
go run ./cmd/mangate --language ru --page-downloads 3 config
bash -n scripts/run.sh scripts/install.sh
```
