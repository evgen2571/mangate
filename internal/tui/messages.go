package tui

import (
	"github.com/evgen2571/mangate/internal/tuiapp"
)

type searchSubmittedMsg struct {
	Query string
}

type searchSucceededMsg struct {
	Query   string
	Results []tuiapp.SearchResult
	History []string
}

type searchFailedMsg struct {
	Err error
}

type chaptersOpenRequestedMsg struct {
	Result tuiapp.SearchResult
}

type fullMangaDownloadRequestedMsg struct {
	Result tuiapp.SearchResult
}

type chaptersLoadedMsg struct {
	Manga    tuiapp.MangaDetails
	Chapters []tuiapp.ChapterItem
}

type chaptersFailedMsg struct {
	MangaID string
	Err     error
}

type downloadRequestedMsg struct {
	Manga    tuiapp.MangaDetails
	Chapters []tuiapp.ChapterItem
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
	Manga    tuiapp.MangaDetails
	Chapters []tuiapp.ChapterItem
}

type downloadFailedMsg struct {
	Manga    tuiapp.MangaDetails
	Chapters []tuiapp.ChapterItem
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
	Config tuiapp.ConfigState
}

type configSaveRequestedMsg struct {
	Config tuiapp.ConfigState
}
