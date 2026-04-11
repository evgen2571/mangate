# Tasks

### Refactoring
- [x] Refactor config logic (_it's really disgusting_)
- [x] Refactor folder managment
- [ ] Refactor unnecessary public functions, variables, methods to private
- [x] Change `sources` package to `source` (_@evgen2571 i really don't like this title_)

### Source managment
- [x] Somehow get a manga cover
- [ ] Improve metadata organization
- [ ] Create converter to download in custom types (_e.g. zip, cbz, plain_)

### CLI
- [ ] Possibility to switch between TUI and CLI (_maybe create `tui` endpoint to use it_)
- [ ] Add some flags (_e.g. --path, --type_)
- [ ] Add flags to update config

---

## Problems to fix

- [ ] Some mangas has multiple languages (*add default language to config*), and as result we see a lot of same chapters, but in diffrent languages
- [ ] `DownloadManga` in TUI is really slow
- [ ] CLI broke after I change something

---

## Main parts
- [x] Create simple API request (DO NOT MOVE TO THE NEXT STEPS BEFORE THIS ONE)
- [x] Create [MangaDex API](https://api.mangadex.org/docs/) client
- [x] Add client method to get manga chapters (_by manga id_)
- [x] Add download function to download manga's chapter & manga itself
- [x] Create simple CLI interface (_using [cobra](https://github.com/spf13/cobra)_)
- [x] Create TUI interface (_using [bubbltea](https://github.com/charmbracelet/bubbletea)_)

## Features
- [x] Search for title 
- [x] Move between titles shown as result on search request
- [ ] Show title metadata (e.g. cover, alternative titles; maybe we will need to use [AniList API](https://docs.anilist.co/) for this)
- [x] Choose title
- [x] Show it's chapters, move between them
- [ ] Choose one or more chapters
- [ ] Choose provider
- [ ] Download chosen chapters (maybe add possibility to choose download type, e. g. .zip, plain images)

---

### MVP Plan
Only one provider (*MangaDex*), CLI version with these commands:
- `search <name>`
- `chapters <manga-id>`
- `download <query>`

Download flags:
- `--dir` - save location (_default: '.'_)
- `--type` - format of downloaded chapter (_e.g. plain, .zip, .cbz; default: plain_)

