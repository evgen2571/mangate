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
		Volume    string `json:"volume"`
		Chapter   string `json:"chapter"`
		Title     string `json:"title"`
		Pages     int    `json:"pages"`
		Language  string `json:"translatedLanguage"`
		PublishAt string `json:"publishAt"`
	} `json:"attributes"`
	Relationships []struct {
		Type       string `json:"type"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	} `json:"relationships"`
}

func (pr *Provider) Chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	allChapters := make([]mangaDexChapter, 0)

	for offset := 0; ; offset += mangaDexChapterPageLimit {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(mangaDexChapterPageLimit))
		params.Set("offset", strconv.Itoa(offset))
		params.Add("includes[]", "scanlation_group")

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
			defer resp.Body.Close()

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
		ID:           mdc.ID,
		URL:          mdc.URL,
		Volume:       mdc.Attributes.Volume,
		Index:        mdc.getIndex(),
		Title:        mdc.getTitle(),
		Language:     mdc.Attributes.Language,
		ReleaseGroup: mdc.releaseGroup(),
		PublishedAt:  mdc.Attributes.PublishAt,
		PageCount:    mdc.Attributes.Pages,
	}
}

func (mdc *mangaDexChapter) releaseGroup() string {
	for _, relationship := range mdc.Relationships {
		if relationship.Type == "scanlation_group" && relationship.Attributes.Name != "" {
			return relationship.Attributes.Name
		}
	}
	return ""
}

func sortChaptersByChapter(chapters []mangaDexChapter) {
	sort.Slice(chapters, func(i, j int) bool {
		chapterI, errI := strconv.ParseFloat(chapters[i].Attributes.Chapter, 64)
		chapterJ, errJ := strconv.ParseFloat(chapters[j].Attributes.Chapter, 64)

		if errI != nil && errJ != nil {
			return chapterTieBreak(chapters[i], chapters[j])
		}
		if errI != nil {
			return false
		}
		if errJ != nil {
			return true
		}

		if chapterI != chapterJ {
			return chapterI < chapterJ
		}
		return chapterTieBreak(chapters[i], chapters[j])
	})
}

func chapterTieBreak(left, right mangaDexChapter) bool {
	leftFields := []string{left.Attributes.Chapter, left.Attributes.Language, left.releaseGroup(), left.Attributes.PublishAt, left.ID}
	rightFields := []string{right.Attributes.Chapter, right.Attributes.Language, right.releaseGroup(), right.Attributes.PublishAt, right.ID}
	for index := range leftFields {
		if leftFields[index] != rightFields[index] {
			return leftFields[index] < rightFields[index]
		}
	}
	return false
}
