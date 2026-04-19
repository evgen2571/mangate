package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/evgen2571/mangate/internal/source"
)

type mangaDexChapter struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		Volume   string `json:"volume"`
		Chapter  string `json:"chapter"`
		Title    string `json:"title"`
		Language string `json:"translatedLanguage"`
	} `json:"attributes"`
}

func (pr *Provider) Chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	params := url.Values{}
	params.Set("limit", "500")

	url := pr.api("manga/" + manga.ID + "/feed?" + params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create chapters request in %q: %w", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute chapters request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chapters request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var mangaDexResponse mangaDexResponse[mangaDexChapter]
	if err := json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return nil, fmt.Errorf("decode chapters response in %q: %w", pr.Name(), err)
	}

	sortChaptersByChapter(mangaDexResponse.Data)

	chapters := make([]*source.Chapter, 0, len(mangaDexResponse.Data))
	for _, mdc := range mangaDexResponse.Data {
		if mdc.Attributes.Language != pr.language {
			continue
		}

		mdc.URL = pr.site("chapter/" + mdc.ID)
		chapter := mdc.toSource()
		chapter.From = manga
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func (mdc *mangaDexChapter) getTitle() string {
	return mdc.Attributes.Title
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
