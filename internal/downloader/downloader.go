package downloader

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/converter"
	"github.com/evgen2571/mangate/internal/util"
)

type Downloader struct {
	cfg       config.Config
	client    *http.Client
	converter *converter.Converter

	pageDownloads chan struct{}

	basePathOnce sync.Once
	workPath     string
	ownsWorkPath bool
	basePathErr  error
}

func New(config config.Config, client *http.Client) *Downloader {
	pageDownloads := config.Concurrency.PageDownloads
	if pageDownloads <= 0 {
		pageDownloads = 1
	}

	return &Downloader{
		cfg:           config,
		client:        client,
		converter:     converter.New(config),
		pageDownloads: make(chan struct{}, pageDownloads),
	}
}

func (d *Downloader) Close() error {
	var cleanupErr error

	if err := util.CleanupTemp(d.cfg.Dirs.Temp, 24*time.Hour); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup stale temp directories: %w", err))
	}

	if !d.ownsWorkPath || d.workPath == "" {
		return cleanupErr
	}

	if err := os.RemoveAll(d.workPath); err != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary work directory %q: %w", d.workPath, err))
	}

	d.workPath = ""
	d.ownsWorkPath = false

	return cleanupErr
}
