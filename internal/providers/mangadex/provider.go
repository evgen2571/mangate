package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/source"
)

type Provider struct {
	client     *http.Client
	siteURL    string
	baseURL    string
	uploadsURL string
	language   string
}

func New(cfg config.Config, client *http.Client) (*Provider, error) {
	return &Provider{
		client:     client,
		siteURL:    cfg.Providers.MangaDex.SiteURL,
		baseURL:    cfg.Providers.MangaDex.BaseURL,
		uploadsURL: cfg.Providers.MangaDex.UploadsURL,
		language:   cfg.Language,
	}, nil
}

func (pr *Provider) Name() string {
	return "mangadex"
}

func (md *MangaDex) GetPages(chapter *source.Chapter) ([]*source.Page, error) {
	url := md.BaseURL + "at-home/server/" + chapter.ID

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pageResp mangaDexPageResponse
	if err = json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return nil, err
	}

	pages := pageResp.toSourcePages(md.UploadsBaseURL)
	for _, page := range pages {
		page.From = chapter
	}

	return pages, nil
}

func (md *MangaDex) GetCover(manga *source.Manga) (string, error) {
	url := "https://api.mangadex.org/cover?manga[]=" + manga.ID

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var coverResp mangaDexCoverResponse
	err = json.NewDecoder(resp.Body).Decode(&coverResp)
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	coverUrl := md.UploadsBaseURL + manga.ID + "/" + coverResp.Attributes.Filename

	return coverUrl, nil
}
