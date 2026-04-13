package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/evgen2571/manga-downloader/internal/source"
)

func (pr *Provider) Cover(manga *source.Manga) (string, error) {
	url := "https://api.mangadex.org/cover?manga[]=" + manga.ID

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create `cover` request in `%s`: %v", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get response from `%s`: %v", pr.Name(), err)
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

	coverUrl := pr.uploadsURL + manga.ID + "/" + coverResp.Attributes.Filename

	return coverUrl, nil
}
