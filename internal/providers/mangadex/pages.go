package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/evgen2571/mangate/internal/source"
)

func (pr *Provider) Pages(ctx context.Context, chapter *source.Chapter) ([]*source.Page, error) {
	url := pr.api("at-home/server/" + chapter.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create `pages` request in `%s`: %v", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from `%s`: %v", pr.Name(), err)
	}
	defer resp.Body.Close()

	var pageResp mangaDexPageResponse
	if err = json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return nil, err
	}

	pages := pageResp.toSourcePages(pr.uploads("data/"))
	for _, page := range pages {
		page.From = chapter
	}

	return pages, nil
}

func (mdpr *mangaDexPageResponse) toSourcePages(urlStart string) []*source.Page {
	pages := make([]*source.Page, 0, len(mdpr.Chapter.Data))

	for _, fileName := range mdpr.Chapter.Data {
		page := &source.Page{
			URL: fmt.Sprintf("%s%s/%s", urlStart, mdpr.Chapter.Hash, fileName),
		}

		pages = append(pages, page)
	}

	return pages
}
