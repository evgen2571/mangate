package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

type Cache struct {
	cfg    config.Config
	client *http.Client

	mu       sync.Mutex
	inflight map[string]*sync.WaitGroup
}

func New(cfg config.Config, client *http.Client) *Cache {
	return &Cache{
		cfg:      cfg,
		client:   client,
		inflight: make(map[string]*sync.WaitGroup),
	}
}

func (c *Cache) Get(ctx context.Context, provider providers.Provider, manga *source.Manga) (string, error) {
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
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create cover cache dir: %w", err)
	}

	key := provider.Name() + ":" + manga.ID

	wg := c.lockInflight(key)
	if wg != nil {
		defer c.unlockInflight(key)
	} else {
		// another goroutine is downloading the same cover
		return path, nil
	}

	tmp := path + ".tmp"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download cover: unexpected status %s", resp.Status)
	}

	out, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("save cover: %w", err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("finalize cover file: %w", err)
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

func (c *Cache) lockInflight(key string) *sync.WaitGroup {
	c.mu.Lock()
	defer c.mu.Unlock()

	if wg, exists := c.inflight[key]; exists {
		c.mu.Unlock()
		wg.Wait()
		c.mu.Lock()
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.inflight[key] = wg
	return wg
}

func (c *Cache) unlockInflight(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if wg, ok := c.inflight[key]; ok {
		wg.Done()
		delete(c.inflight, key)
	}
}
