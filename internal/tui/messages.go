package tui

import "github.com/evgen2571/mangate/internal/source"

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

type chaptersLoadedMsg struct {
	Manga    *source.Manga
	Chapters []*source.Chapter
}

type chaptersFailedMsg struct {
	Manga *source.Manga
	Err   error
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
