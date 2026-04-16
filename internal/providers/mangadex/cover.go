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

	var mangaDexResponse mangaDexResponse[mangaDexCover]
	err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse)
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	coverUrl := pr.uploads("covers/" + manga.ID + "/" + mangaDexResponse.Data[0].Attributes.Filename)

	return coverUrl, nil
}
