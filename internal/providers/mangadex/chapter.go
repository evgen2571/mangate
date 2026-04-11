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
	Index int
}

func (mdm *mangaDexChapter) getTitle() string {
	var title string

	if mdm.Attributes.Chapter != "" {
		title += mdm.Attributes.Chapter
	}

	if mdm.Attributes.Title != "" {
		title += ": "
		title += mdm.Attributes.Title
	}

	return title
}

func (mdm *mangaDexChapter) toSource() *sources.Chapter {
	return &sources.Chapter{
		ID:    mdm.ID,
		Title: mdm.getTitle(),
		Index: mdm.Index,
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
