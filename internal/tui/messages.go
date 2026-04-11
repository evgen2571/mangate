package tui

import "github.com/evgen2571/manga-downloader/internal/sources"

type mangasLoadedMsg struct {
	items []*sources.Manga
	err   error
	query string
}

type mangaSelectedMsg struct {
	manga *sources.Manga
}

type mangaDownloadRequestedMsg struct {
	manga *sources.Manga
}

type chapterDownloadRequestedMsg struct {
	manga   *sources.Manga
	chapter *sources.Chapter
}

type chaptersLoadedMsg struct {
	manga    *sources.Manga
	chapters []*sources.Chapter
	err      error
}

type downloadFinishedMsg struct {
	err error
}

type backToSearchMsg struct{}
type backToMangasMsg struct{}
type backFromDownloadMsg struct{}
