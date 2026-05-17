package tuiapp

import (
	"context"
	"time"
)

type SearchResult struct {
	ID           string
	Title        string
	URL          string
	SummaryMD    string
	ChapterCount int
}

type MangaDetails struct {
	ID           string
	Title        string
	URL          string
	SummaryMD    string
	ChapterCount int
}

type ChapterItem struct {
	ID          string
	Index       string
	Title       string
	DisplayText string
	URL         string
}

type CoverSize struct {
	Width  int
	Height int
}

type CoverResult struct {
	MangaID string
	Path    string
}

type DownloadRequest struct {
	Manga    MangaDetails
	Chapters []ChapterItem
}

type ChapterProgress struct {
	Name           string
	CompletedPages int
	TotalPages     int
	Active         bool
	Completed      bool
}

type DownloadProgress struct {
	CompletedPages    int
	TotalPages        int
	CompletedChapters int
	TotalChapters     int
	Chapters          []ChapterProgress
}

type ConfigState struct {
	Provider           string
	Language           string
	HTTPTimeout        time.Duration
	DownloadDir        string
	DownloadType       string
	PageDownloads      int
	ChapterDownloads   int
	SearchHistoryMax   int
	CacheDir           string
	TempDir            string
	MangaDexSiteURL    string
	MangaDexBaseURL    string
	MangaDexUploadsURL string
}

type Service interface {
	Search(context.Context, string) ([]SearchResult, error)
	SearchHistory(context.Context) ([]string, error)
	LoadChapters(context.Context, SearchResult) (MangaDetails, []ChapterItem, error)
	LoadCover(context.Context, SearchResult, CoverSize) (CoverResult, error)
	Download(context.Context, DownloadRequest, func(DownloadProgress)) error
	Config() ConfigState
	ApplyConfig(context.Context, ConfigState) (ConfigState, error)
	SaveConfig(context.Context, ConfigState) (ConfigState, error)
}
