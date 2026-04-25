package tui

import (
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
)

type searchSubmittedMsg struct {
	Query string
}

type searchSucceededMsg struct {
	Query   string
	Results []*source.Manga
}

type searchFailedMsg struct {
	Err error
}

type chaptersOpenRequestedMsg struct {
	Manga *source.Manga
}

type fullMangaDownloadRequestedMsg struct {
	Manga *source.Manga
}

type chaptersLoadedMsg struct {
	Manga    *source.Manga
	Chapters []*source.Chapter
}

type chaptersFailedMsg struct {
	Manga *source.Manga
	Err   error
}

type downloadRequestedMsg struct {
	Manga    *source.Manga
	Chapters []*source.Chapter
}

type downloadProgressMsg struct {
	Title     string
	Detail    string
	Status    string
	Completed int
	Total     int
	Chapters  []chapterProgressView
}

type downloadSucceededMsg struct {
	Manga    *source.Manga
	Chapters []*source.Chapter
}

type downloadFailedMsg struct {
	Manga    *source.Manga
	Chapters []*source.Chapter
	Err      error
}

type coverLoadRequestedMsg struct {
	MangaID string
}

type coverLoadedMsg struct {
	MangaID string
	Path    string
	Render  string
}

type coverFailedMsg struct {
	MangaID string
	Err     error
}

type goBackMsg struct{}

type configApplyRequestedMsg struct {
	Config config.Config
}

type configSaveRequestedMsg struct {
	Config config.Config
}
