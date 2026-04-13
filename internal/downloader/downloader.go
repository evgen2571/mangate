package downloader

import "github.com/evgen2571/manga-downloader/internal/config"

type Downloader struct {
	cfg config.Config
}

func New(config config.Config) *Downloader {
	return &Downloader{
		cfg: config,
	}
}
