package downloader

import (
	"net/http"

	"github.com/evgen2571/mangate/internal/config"
)

type Downloader struct {
	cfg            config.Config
	client         *http.Client
	pageDownloads  chan struct{}
	pageRetryLimit int
}

func New(config config.Config, client *http.Client) *Downloader {
	pageDownloads := config.Concurrency.PageDownloads
	if pageDownloads <= 0 {
		pageDownloads = 1
	}

	return &Downloader{
		cfg:            config,
		client:         client,
		pageDownloads:  make(chan struct{}, pageDownloads),
		pageRetryLimit: maxPageDownloadRetries,
	}
}

// NewWithPageRetryLimit creates a downloader with the ordinary configuration
// plus a run-scoped retry limit. It is used by dataset collection so its
// persisted runtime settings do not mutate the user's global configuration.
func NewWithPageRetryLimit(config config.Config, client *http.Client, retryLimit int) *Downloader {
	download := New(config, client)
	if retryLimit >= 0 {
		download.pageRetryLimit = retryLimit
	}
	return download
}
