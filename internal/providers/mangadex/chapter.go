package mangadex

import (
	"sort"
	"strconv"

	"github.com/evgen2571/manga-downloader/internal/sources"
)

type mangaDexChapter struct {
	ID         string `json:"id"`
	Attributes struct {
		Volume  string `json:"volume"`
		Chapter string `json:"chapter"`
		Title   string `json:"title"`
	} `json:"attributes"`
}

func (mdc *mangaDexChapter) getTitle() string {
	var title string

	if mdc.Attributes.Chapter != "" {
		title += mdc.Attributes.Chapter
	}

	if mdc.Attributes.Title != "" {
		title += ": "
		title += mdc.Attributes.Title
	}

	return title
}

func (mdc *mangaDexChapter) toSource() *sources.Chapter {
	return &sources.Chapter{
		ID:    mdc.ID,
		Title: mdc.getTitle(),
	}
}

func sortChaptersByChapter(chapters []mangaDexChapter) {
	sort.Slice(chapters, func(i, j int) bool {
		chapterI, errI := strconv.ParseFloat(chapters[i].Attributes.Chapter, 64)
		chapterJ, errJ := strconv.ParseFloat(chapters[j].Attributes.Chapter, 64)

		if errI != nil && errJ != nil {
			return chapters[i].Attributes.Chapter < chapters[j].Attributes.Chapter
		}
		if errI != nil {
			return false
		}
		if errJ != nil {
			return true
		}

		return chapterI < chapterJ
	})
}
