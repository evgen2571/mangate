package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/evgen2571/mangate/internal/source"
)

type mangaDexManga struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		TitleMap       map[string]string `json:"title"`
		DescriptionMap map[string]string `json:"description"`
		Status         string            `json:"status"`
	} `json:"attributes"`
}

func (pr *Provider) Search(ctx context.Context, title string) ([]*source.Manga, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("limit", "100")

	url := pr.api("manga/?" + params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create search request in %q: %w", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var mangaDexResponse mangaDexResponse[mangaDexManga]
	if err := json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return nil, fmt.Errorf("decode search response in %q: %w", pr.Name(), err)
	}

	mangas := make([]*source.Manga, 0, len(mangaDexResponse.Data))
	for _, mangaDexManga := range mangaDexResponse.Data {
		mangaDexManga.URL = pr.site("title/" + mangaDexManga.ID)
		manga := mangaDexManga.toSource()
		mangas = append(mangas, manga)
	}

	return mangas, nil
}

// Title retrieves one title by its stable MangaDex identifier. The title
// endpoint has the same item shape as search but wraps it in a single data
// object rather than a list.
func (pr *Provider) Title(ctx context.Context, id string) (*source.Manga, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("title id cannot be empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pr.api("manga/"+url.PathEscape(id)), nil)
	if err != nil {
		return nil, fmt.Errorf("create title request in %q: %w", pr.Name(), err)
	}
	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute title request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("title %q not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("title request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var payload struct {
		Data mangaDexManga `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode title response in %q: %w", pr.Name(), err)
	}
	if payload.Data.ID == "" {
		return nil, fmt.Errorf("title response in %q did not include an id", pr.Name())
	}
	payload.Data.URL = pr.site("title/" + payload.Data.ID)
	return payload.Data.toSource(), nil
}

func (mdm *mangaDexManga) getTitle() string {
	title := ""
	for _, t := range mdm.Attributes.TitleMap {
		title = t
		break
	}

	return title
}

func (mdm *mangaDexManga) toSource() *source.Manga {
	return &source.Manga{
		ID:    mdm.ID,
		URL:   mdm.URL,
		Title: mdm.getTitle(),
		Metadata: source.MangaMetadata{
			Description: mdm.Attributes.DescriptionMap,
		},
	}
}
