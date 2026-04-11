package mangadex

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/sources"
)

func (mdpr *mangaDexPageResponse) toSourcePages(uploadsURL string) []*sources.Page {
	pages := make([]*sources.Page, 0, len(mdpr.Chapter.Data))

	for _, fileName := range mdpr.Chapter.Data {
		page := &sources.Page{
			URL: fmt.Sprintf("%sdata/%s/%s", uploadsURL, mdpr.Chapter.Hash, fileName),
		}

		pages = append(pages, page)
	}

	return pages
}
