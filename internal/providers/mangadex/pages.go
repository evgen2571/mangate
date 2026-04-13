package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/evgen2571/manga-downloader/internal/source"
)

func (pr *Provider) GetPages(chapter *source.Chapter) ([]*source.Page, error) {
	url := pr.baseURL + "at-home/server/" + chapter.ID

	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	pages := pageResp.toSourcePages(pr.uploadsURL)
	for _, page := range pages {
		page.From = chapter
	}

	return pages, nil
}

func (mdpr *mangaDexPageResponse) toSourcePages(uploadsURL string) []*source.Page {
	pages := make([]*source.Page, 0, len(mdpr.Chapter.Data))

	for _, fileName := range mdpr.Chapter.Data {
		page := &source.Page{
			URL: fmt.Sprintf("%sdata/%s/%s", uploadsURL, mdpr.Chapter.Hash, fileName),
		}

		pages = append(pages, page)
	}

	return pages
}
