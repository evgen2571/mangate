package mangadex

import (
	"fmt"

	"github.com/evgen2571/manga-downloader/internal/source"
)

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
