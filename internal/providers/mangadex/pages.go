package mangadex

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/source"
)

func (pr *Provider) Pages(ctx context.Context, chapter *source.Chapter) ([]*source.Page, error) {
	var pageResp mangaDexPageResponse
	if err := pr.getJSON(ctx, "pages for "+chapter.ID, "at-home/server/"+url.PathEscape(chapter.ID), &pageResp); err != nil {
		return nil, err
	}

	pages, err := pageResp.toSourcePages()
	if err != nil {
		return nil, fmt.Errorf("%w: pages response for chapter %q: %w", source.ErrCandidateUnavailable, chapter.ID, err)
	}

	return pages, nil
}

func (pr *Provider) paceAtHomeRequest(ctx context.Context) error {
	if pr.atHomeMinInterval <= 0 {
		return nil
	}

	select {
	case pr.atHomeGate <- struct{}{}:
		defer func() { <-pr.atHomeGate }()
	case <-ctx.Done():
		return ctx.Err()
	}

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

func (mdpr *mangaDexPageResponse) toSourcePages() ([]*source.Page, error) {
	base, err := url.Parse(mdpr.BaseURL)
	if err != nil ||
		(base.Scheme != "https" &&
			base.Scheme != "http") ||
		base.Host == "" ||
		base.User != nil ||
		base.RawQuery != "" ||
		base.Fragment != "" {
		return nil, fmt.Errorf("invalid image base URL")
	}

	if !validPathSegment(mdpr.Chapter.Hash) || len(mdpr.Chapter.Data) == 0 {
		return nil, fmt.Errorf("missing chapter hash or page filenames")
	}

	pages := make([]*source.Page, 0, len(mdpr.Chapter.Data))
	urlStart := strings.TrimRight(mdpr.BaseURL, "/") + "/data/"

	for _, fileName := range mdpr.Chapter.Data {
		if !validPathSegment(fileName) {
			return nil, fmt.Errorf("invalid page filename %q", fileName)
		}

		page := &source.Page{
			URL: urlStart + url.PathEscape(mdpr.Chapter.Hash) + "/" + url.PathEscape(fileName),
		}

		pages = append(pages, page)
	}

	return pages, nil
}
