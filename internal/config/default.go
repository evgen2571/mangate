package config

var (
	DownloadDir = "downloads"
	CacheDir    = "~/.cache/manga-downloader"

	DownloadType = "jpg"

	Provider        = "mangadex"
	DefaultLanguage = "en"

	MaxConcurrentPageFetches = 4

	MaxConcurrentPageDownloads    = 8
	MaxConcurrentChapterDownloads = 1
)
