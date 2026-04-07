# MVP Plan
Only one provider (*MangaDex*), CLI version with these commands:
- `search <name>`
- `chapters <manga-id>`
- `download <chapter-id>`

Download flags:
- `--dir` - save location (_default: '.'_)
- `--format` - format of downloaded chapter (_e.g. plain, .zip, .cbz; default: plain_)

---

## Main parts
- [x] Create simple API request (DO NOT MOVE TO THE NEXT STEPS BEFORE THIS ONE)
- [x] Create [MangaDex API](https://api.mangadex.org/docs/) client
- [ ] Add client method to get manga chapters (_by manga id_)
- [ ] Create `Downloader` struct (_insdide `internal/downloader/downloader.go`_) that will be like 2nd client
- [ ] Add downloader method to download manga's chapter (_by chapter id_)
- [ ] Create simple CLI interface (_temp solution to complete MVP_)
- [ ] Create TUI interface (using [bubbltea](https://github.com/charmbracelet/bubbletea) i guess)

## Features
- [ ] Search for title 
- [ ] Move between titles shown as result on search request
- [ ] Show title metadata (e.g. cover, alternative titles; maybe we will need to use [AniList API](https://docs.anilist.co/) for this)
- [ ] Choose title
- [ ] Show it's chapters, move between them
- [ ] Choose one or more chapters
- [ ] Download chosen chapters (maybe add possibility to choose download type, e. g. .zip, images)
