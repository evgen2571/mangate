package mangadex

import (
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type mangaDexChapter struct {
	ID         string `json:"id"`
	Attributes struct {
		Volume  int    `json:"volume"`
		Chapter int    `json:"chapter"`
		Title   string `json:"title"`
	} `json:"chapter"`
	Index int
}

func (mdm *mangaDexChapter) getTitle() string {
	return mdm.Attributes.Title
}

func (mdm *mangaDexChapter) toSource() *sources.Chapter {
	return &sources.Chapter{
		ID:    mdm.ID,
		Title: mdm.getTitle(),
		Index: mdm.Index,
	}
}
