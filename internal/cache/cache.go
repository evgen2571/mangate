package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/source"
	"github.com/evgen2571/mangate/internal/util"
)

type CoverProvider interface {
	Name() string
	Cover(context.Context, *source.Manga) (string, error)
}

type inflightDownload struct {
	done chan struct{}
	err  error
}

type Cache struct {
	cfg    config.Config
	client *http.Client

	mu       sync.Mutex
	inflight map[string]*inflightDownload
}

func New(cfg config.Config, client *http.Client) *Cache {
	return &Cache{
		cfg:      cfg,
		client:   client,
		inflight: make(map[string]*inflightDownload),
	}
}

func (c *Cache) Get(ctx context.Context, provider CoverProvider, manga *source.Manga) (string, error) {
	if manga == nil {
		return "", fmt.Errorf("nil manga")
	}

	remoteURL, err := provider.Cover(ctx, manga)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remoteURL) == "" {
		return "", fmt.Errorf("empty cover url")
	}

	path := c.coverPath(provider.Name(), manga.ID, remoteURL)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat cover cache file %q: %w", path, err)
	}

	key := provider.Name() + ":" + manga.ID
	download, owner := c.beginInflight(key)
	if !owner {
		<-download.done
		if download.err != nil {
			return "", download.err
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat cover cache file %q after download: %w", path, err)
		}
		return "", fmt.Errorf("cover cache file missing after download: %s", path)
	}

	var downloadErr error
	defer func() {
		c.finishInflight(key, download, downloadErr)
	}()

	if err := util.EnsureDir(filepath.Dir(path), "cover cache directory"); err != nil {
		downloadErr = err
		return "", downloadErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		downloadErr = fmt.Errorf("create request: %w", err)
		return "", downloadErr
	}

	resp, err := c.client.Do(req)
	if err != nil {
		downloadErr = fmt.Errorf("download cover: %w", err)
		return "", downloadErr
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		downloadErr = fmt.Errorf("download cover: unexpected status %s", resp.Status)
		return "", downloadErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "cover-*.tmp")
	if err != nil {
		downloadErr = fmt.Errorf("create temp file: %w", err)
		return "", downloadErr
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		downloadErr = fmt.Errorf("save cover: %w", err)
		return "", downloadErr
	}

	if err := tmp.Close(); err != nil {
		downloadErr = fmt.Errorf("close temp file: %w", err)
		return "", downloadErr
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		downloadErr = fmt.Errorf("finalize cover file: %w", err)
		return "", downloadErr
	}

	return path, nil
}

func (c *Cache) coverPath(providerName, mangaID, remoteURL string) string {
	ext := filepath.Ext(remoteURL)
	if ext == "" {
		ext = ".img"
	}

	sum := sha1.Sum([]byte(remoteURL))
	name := mangaID + "-" + hex.EncodeToString(sum[:8]) + ext

	return filepath.Join(c.cfg.Dirs.Cache, "covers", providerName, name)
}

func (c *Cache) beginInflight(key string) (*inflightDownload, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if download, ok := c.inflight[key]; ok {
		return download, false
	}

	download := &inflightDownload{done: make(chan struct{})}
	c.inflight[key] = download
	return download, true
}

func (c *Cache) finishInflight(key string, download *inflightDownload, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if current, ok := c.inflight[key]; ok && current == download {
		download.err = err
		close(download.done)
		delete(c.inflight, key)
	}
}
