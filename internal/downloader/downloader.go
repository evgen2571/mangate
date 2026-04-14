package downloader

import (
	"net/http"

	"github.com/evgen2571/mangate/internal/config"
)

type Downloader struct {
	cfg    config.Config
	client *http.Client
}

func New(config config.Config, client *http.Client) *Downloader {
	return &Downloader{
		cfg:    config,
		client: client,
	}
}
