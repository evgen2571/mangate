package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/evgen2571/mangate/internal/source"
)

type mangaDexCover struct {
	ID         string `json:"id"`
	Attributes struct {
		Filename string `json:"filename"`
	} `json:"attributes"`
}

func (pr *Provider) Cover(ctx context.Context, manga *source.Manga) (string, error) {
	url := pr.api("cover?manga[]=" + manga.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create cover request in %q: %w", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute cover request in %q: %w", pr.Name(), err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var mangaDexResponse mangaDexResponse[mangaDexCover]
	if err := json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return "", fmt.Errorf("decode cover response in %q: %w", pr.Name(), err)
	}
	if len(mangaDexResponse.Data) == 0 {
		return "", fmt.Errorf("cover response in %q did not contain cover data for manga %q", pr.Name(), manga.ID)
	}

	coverURL := pr.uploads("covers/" + manga.ID + "/" + mangaDexResponse.Data[0].Attributes.Filename)

	return coverURL, nil
}
