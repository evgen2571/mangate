package mangadex

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

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

	for offset := 0; ; {
		params := url.Values{}
		params.Set("limit", strconv.Itoa(mangaDexChapterPageLimit))
		params.Set("offset", strconv.Itoa(offset))
		params.Add("includes[]", "scanlation_group")

		var mangaDexResponse mangaDexResponse[mangaDexChapter]
		if err := pr.getJSON(ctx, "chapters for "+manga.ID, "manga/"+url.PathEscape(manga.ID)+"/feed?"+params.Encode(), &mangaDexResponse); err != nil {
			return nil, err
		}

		next, more, err := mangaDexResponse.nextOffset(offset, mangaDexChapterPageLimit)
		if err != nil {
			return nil, fmt.Errorf("chapters for %s: %w", manga.ID, err)
		}

		allChapters = append(allChapters, mangaDexResponse.Data...)

		if !more {
			break
		}

		offset = next
	}

	sortChaptersByChapter(allChapters)

	chapters := make([]*source.Chapter, 0, len(allChapters))
	for _, mdc := range allChapters {
		if strings.TrimSpace(mdc.ID) == "" {
			return nil, fmt.Errorf("chapters response contains a chapter without an id")
		}

		mdc.URL = pr.site("chapter/" + url.PathEscape(mdc.ID))
		chapter := mdc.toSource()
		chapter.From = manga
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func (mdc *mangaDexChapter) toSource() *source.Chapter {
	return &source.Chapter{
		ID:           mdc.ID,
		URL:          mdc.URL,
		Volume:       mdc.Attributes.Volume,
		Index:        mdc.Attributes.Chapter,
		Title:        mdc.Attributes.Title,
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
