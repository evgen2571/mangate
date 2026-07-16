package downloader

import (
	"net/http"

	"github.com/evgen2571/mangate/internal/config"
)

type Downloader struct {
	cfg           config.Config
	client        *http.Client
	pageDownloads chan struct{}
}

func New(config config.Config, client *http.Client) *Downloader {
	pageDownloads := config.Concurrency.PageDownloads
	if pageDownloads <= 0 {
		pageDownloads = 1
	}

	return &Downloader{
		cfg:           config,
		client:        client,
		pageDownloads: make(chan struct{}, pageDownloads),
	}
}
