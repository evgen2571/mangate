package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/source"
)

func (pr *Provider) Pages(ctx context.Context, chapter *source.Chapter) ([]*source.Page, error) {
	if err := pr.paceAtHomeRequest(ctx); err != nil {
		return nil, fmt.Errorf("wait for at-home request slot in %q: %w", pr.Name(), err)
	}

	url := pr.api("at-home/server/" + chapter.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create pages request in %q: %w", pr.Name(), err)
	}

	resp, err := pr.doWithRateLimitRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute pages request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pages request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var pageResp mangaDexPageResponse
	if err := json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return nil, fmt.Errorf("decode pages response in %q: %w", pr.Name(), err)
	}

	pages := pageResp.toSourcePages()
	for _, page := range pages {
		page.From = chapter
	}

	return pages, nil
}

func (pr *Provider) paceAtHomeRequest(ctx context.Context) error {
	if pr.atHomeMinInterval <= 0 {
		return nil
	}

	pr.atHomeMu.Lock()
	defer pr.atHomeMu.Unlock()

	if !pr.lastAtHomeRequest.IsZero() {
		wait := pr.atHomeMinInterval - time.Since(pr.lastAtHomeRequest)
		if wait > 0 {
			if err := sleepWithContext(ctx, wait); err != nil {
				return err
			}
		}
	}
	pr.lastAtHomeRequest = time.Now()
	return nil
}

func (mdpr *mangaDexPageResponse) toSourcePages() []*source.Page {
	pages := make([]*source.Page, 0, len(mdpr.Chapter.Data))
	urlStart := strings.TrimRight(mdpr.BaseURL, "/") + "/data/"

	for _, fileName := range mdpr.Chapter.Data {
		page := &source.Page{
			URL: fmt.Sprintf("%s%s/%s", urlStart, mdpr.Chapter.Hash, fileName),
		}

		pages = append(pages, page)
	}

	return pages
}
