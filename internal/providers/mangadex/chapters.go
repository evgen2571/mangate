package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

func (pr *Provider) Chapters(manga *source.Manga) ([]*source.Chapter, error) {
	params := url.Values{}
	params.Set("limit", "500") // set maximum possible limit

	url := pr.baseURL + "manga/" + manga.ID + "/feed?translatedLanguage[]=" + pr.language

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create `chapters` request in `%s`: %v", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from `%s`: %v", pr.Name(), err)
	}
	defer resp.Body.Close()

	var mangaDexResponse mangaDexResponse[mangaDexChapter]
	if err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return nil, err
	}

	sortChaptersByChapter(mangaDexResponse.Data)

	var chapters []*source.Chapter
	for _, mangaDexChapter := range mangaDexResponse.Data {
		mangaDexChapter.URL = pr.siteURL + "chapter/" + mangaDexChapter.ID
		chapter := mangaDexChapter.toSource()
		chapter.From = manga
		chapters = append(chapters, chapter)
	}

	return chapters, nil
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
