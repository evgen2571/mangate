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

type mangaDexManga struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		TitleMap                     map[string]string   `json:"title"`
		AltTitles                    []map[string]string `json:"altTitles"`
		DescriptionMap               map[string]string   `json:"description"`
		Status                       string              `json:"status"`
		ContentRating                string              `json:"contentRating"`
		OriginalLanguage             string              `json:"originalLanguage"`
		Year                         int                 `json:"year"`
		AvailableTranslatedLanguages []string            `json:"availableTranslatedLanguages"`
		CreatedAt                    string              `json:"createdAt"`
		UpdatedAt                    string              `json:"updatedAt"`
		Tags                         []struct {
			Attributes struct {
				Name map[string]string `json:"name"`
			} `json:"attributes"`
		} `json:"tags"`
	} `json:"attributes"`
}

func (pr *Provider) Search(ctx context.Context, title string) ([]*source.Manga, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("limit", "100")

	mangas := make([]*source.Manga, 0)
	for offset := 0; ; {
		params.Set("offset", strconv.Itoa(offset))
		var payload mangaDexResponse[mangaDexManga]
		if err := pr.getJSON(ctx, "search", "manga/?"+params.Encode(), &payload); err != nil {
			return nil, err
		}
		if payload.Total > maxCollectionResults {
			return nil, fmt.Errorf("MangaDex search has %d results; narrow the query to at most %d results", payload.Total, maxCollectionResults)
		}
		next, more, err := payload.nextOffset(offset, 100)
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		for _, item := range payload.Data {
			if strings.TrimSpace(item.ID) == "" {
				return nil, fmt.Errorf("search response contains a manga without an id")
			}
			item.URL = pr.site("title/" + url.PathEscape(item.ID))
			mangas = append(mangas, item.toSource(pr.language))
		}
		if !more {
			break
		}
		offset = next
	}

	return mangas, nil
}

// BrowseManga reads one bounded MangaDex catalog page. Unlike Search it does
// not depend on a title query and is intended for collection planning.
func (pr *Provider) BrowseManga(ctx context.Context, browse source.BrowseRequest) (source.BrowsePage, error) {
	limit := browse.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if browse.Offset < 0 {
		return source.BrowsePage{}, fmt.Errorf("browse offset must be >= 0")
	}
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("offset", strconv.Itoa(browse.Offset))
	for _, value := range browse.OriginalLanguages {
		params.Add("originalLanguage[]", value)
	}
	for _, value := range browse.ChapterLanguages {
		params.Add("availableTranslatedLanguage[]", value)
	}
	for _, value := range browse.Statuses {
		params.Add("status[]", value)
	}
	for _, value := range browse.ContentRatings {
		params.Add("contentRating[]", value)
	}
	for _, value := range browse.IncludedTags {
		params.Add("includedTags[]", value)
	}
	for _, value := range browse.ExcludedTags {
		params.Add("excludedTags[]", value)
	}
	orderBy := strings.TrimSpace(browse.OrderBy)
	if orderBy == "" {
		orderBy = "updatedAt"
	}
	direction := strings.ToLower(strings.TrimSpace(browse.OrderDirection))
	if direction != "asc" && direction != "desc" {
		direction = "desc"
	}
	params.Set("order["+orderBy+"]", direction)
	var payload mangaDexResponse[mangaDexManga]
	if err := pr.getJSON(ctx, "browse", "manga/?"+params.Encode(), &payload); err != nil {
		return source.BrowsePage{}, err
	}

	next, more, err := payload.nextOffset(browse.Offset, limit)
	if err != nil {
		return source.BrowsePage{}, fmt.Errorf("browse: %w", err)
	}
	result := source.BrowsePage{Offset: payload.Offset, Total: payload.Total, NextOffset: next, HasMore: more}
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		item.URL = pr.site("title/" + url.PathEscape(item.ID))
		tags := make([]string, 0, len(item.Attributes.Tags))
		for _, tag := range item.Attributes.Tags {
			if name := localizedValue(tag.Attributes.Name, pr.language); name != "" {
				tags = append(tags, name)
			}
		}
		result.Titles = append(result.Titles, source.BrowseTitle{Manga: item.toSource(pr.language), AvailableLanguages: append([]string(nil), item.Attributes.AvailableTranslatedLanguages...), Tags: tags, CreatedAt: item.Attributes.CreatedAt, UpdatedAt: item.Attributes.UpdatedAt})
	}
	return result, nil
}

// Title retrieves one title by its stable MangaDex identifier. The title
// endpoint has the same item shape as search but wraps it in a single data
// object rather than a list.
func (pr *Provider) Title(ctx context.Context, id string) (*source.Manga, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("title id cannot be empty")
	}

	var payload struct {
		Data mangaDexManga `json:"data"`
	}
	if err := pr.getJSON(ctx, "title "+id, "manga/"+url.PathEscape(id), &payload); err != nil {
		return nil, err
	}

	if payload.Data.ID == "" {
		return nil, fmt.Errorf("title response in %q did not include an id", pr.Name())
	}
	payload.Data.URL = pr.site("title/" + url.PathEscape(payload.Data.ID))
	return payload.Data.toSource(pr.language), nil
}

func (mdm *mangaDexManga) getTitle(preferredLanguage string) string {
	return localizedValue(mdm.Attributes.TitleMap, preferredLanguage)
}

func (mdm *mangaDexManga) alternativeTitle(preferredLanguage, primaryTitle string) string {
	for _, language := range []string{preferredLanguage, "en"} {
		for _, titles := range mdm.Attributes.AltTitles {
			if title := strings.TrimSpace(titles[language]); title != "" && title != primaryTitle {
				return title
			}
		}
	}
	for _, titles := range mdm.Attributes.AltTitles {
		if title := localizedValue(titles, preferredLanguage); title != "" && title != primaryTitle {
			return title
		}
	}
	return ""
}

func localizedValue(values map[string]string, preferredLanguage string) string {
	for _, language := range []string{preferredLanguage, "en"} {
		if value := strings.TrimSpace(values[language]); value != "" {
			return value
		}
	}

	languages := make([]string, 0, len(values))
	for language := range values {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	for _, language := range languages {
		if value := strings.TrimSpace(values[language]); value != "" {
			return value
		}
	}
	return ""
}

func (mdm *mangaDexManga) toSource(preferredLanguage string) *source.Manga {
	primaryTitle := mdm.getTitle(preferredLanguage)
	return &source.Manga{
		ID:    mdm.ID,
		URL:   mdm.URL,
		Title: primaryTitle,
		Metadata: source.MangaMetadata{
			Description:      mdm.Attributes.DescriptionMap,
			AlternativeTitle: mdm.alternativeTitle(preferredLanguage, primaryTitle),
			Status:           mdm.Attributes.Status,
			ContentType:      mdm.Attributes.ContentRating,
			Language:         mdm.Attributes.OriginalLanguage,
			Year:             mdm.Attributes.Year,
		},
	}
}
