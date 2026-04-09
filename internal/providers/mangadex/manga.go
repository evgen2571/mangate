package mangadex

import (
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type mangaDexManga struct {
	ID         string `json:"id"`
	Attributes struct {
		TitleMap    map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		Status      string            `json:"status"`
	} `json:"attributes"`
}

func (mdm *mangaDexManga) getTitle() string {
	title := ""
	for _, t := range mdm.Attributes.TitleMap {
		title = t
		break
	}

	return title
}

func (mdm *mangaDexManga) toSource() *sources.Manga {
	return &sources.Manga{
		ID:          mdm.ID,
		Title:       mdm.getTitle(),
		Description: mdm.Attributes.Description,
	}
}
