package tui

import "github.com/evgen2571/mangate/internal/source"

type mangasLoadedMsg struct {
	items []*source.Manga
	err   error
	query string
}

type mangaSelectedMsg struct {
	manga *source.Manga
}

type mangaDownloadRequestedMsg struct {
	manga *source.Manga
}

type chapterDownloadRequestedMsg struct {
	manga   *source.Manga
	chapter *source.Chapter
}

type chaptersLoadedMsg struct {
	manga    *source.Manga
	chapters []*source.Chapter
	err      error
}

type downloadFinishedMsg struct {
	err error
}

type chaptersDownloadRequestedMsg struct {
	manga    *source.Manga
	chapters []*source.Chapter
}

type backToSearchMsg struct{}
type backToMangasMsg struct{}
type backFromDownloadMsg struct{}
