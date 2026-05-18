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

const mangaDexChapterPageLimit = 500

type mangaDexChapter struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		Volume   string `json:"volume"`
		Chapter  string `json:"chapter"`
		Title    string `json:"title"`
		Pages    int    `json:"pages"`
		Language string `json:"translatedLanguage"`
	} `json:"attributes"`
}

func (pr *Provider) Chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	allChapters := make([]mangaDexChapter, 0)

	for offset := 0; ; offset += mangaDexChapterPageLimit {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(mangaDexChapterPageLimit))
		params.Set("offset", strconv.Itoa(offset))
		params.Add("translatedLanguage[]", pr.language)

		url := pr.api("manga/" + manga.ID + "/feed?" + params.Encode())

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create chapters request in %q: %w", pr.Name(), err)
		}

		resp, err := pr.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute chapters request in %q: %w", pr.Name(), err)
		}

		var mangaDexResponse mangaDexResponse[mangaDexChapter]
		decodeErr := func() error {
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("chapters request in %q returned unexpected status: %s", pr.Name(), resp.Status)
			}

			if err := json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
				return fmt.Errorf("decode chapters response in %q: %w", pr.Name(), err)
			}

			return nil
		}()
		if decodeErr != nil {
			return nil, decodeErr
		}

		allChapters = append(allChapters, mangaDexResponse.Data...)

		if len(mangaDexResponse.Data) == 0 || mangaDexResponse.Total <= mangaDexResponse.Offset+len(mangaDexResponse.Data) {
			break
		}
	}

	sortChaptersByChapter(allChapters)

	chapters := make([]*source.Chapter, 0, len(allChapters))
	for _, mdc := range allChapters {
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
	index := "0"

	if mdc.Attributes.Chapter != "" {
		index = mdc.Attributes.Chapter
	}

	return index
}

func (mdc *mangaDexChapter) toSource() *source.Chapter {
	return &source.Chapter{
		ID:        mdc.ID,
		URL:       mdc.URL,
		Index:     mdc.getIndex(),
		Title:     mdc.getTitle(),
		PageCount: mdc.Attributes.Pages,
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
