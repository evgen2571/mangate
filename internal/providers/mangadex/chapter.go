package mangadex

import (
	"sort"
	"strconv"

	"github.com/evgen2571/manga-downloader/internal/source"
)

type mangaDexChapter struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		Volume  string `json:"volume"`
		Chapter string `json:"chapter"`
		Title   string `json:"title"`
	} `json:"attributes"`
}

func (mdc *mangaDexChapter) getTitle() string {
	var title string = "Chapter " + mdc.getIndex()

	if mdc.Attributes.Title != "" {
		title = mdc.Attributes.Title
	}

	return title
}

func (mdc *mangaDexChapter) getIndex() string {
	var index string = "0"

	if mdc.Attributes.Chapter != "" {
		index = mdc.Attributes.Chapter
	}

	return index
}

func (mdc *mangaDexChapter) toSource() *source.Chapter {
	return &source.Chapter{
		ID:    mdc.ID,
		URL:   mdc.URL,
		Index: mdc.getIndex(),
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
