package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	url := pr.api("manga/?" + params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create search request in %q: %w", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var mangaDexResponse mangaDexResponse[mangaDexManga]
	if err := json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return nil, fmt.Errorf("decode search response in %q: %w", pr.Name(), err)
	}

	mangas := make([]*source.Manga, 0, len(mangaDexResponse.Data))
	for _, mangaDexManga := range mangaDexResponse.Data {
		mangaDexManga.URL = pr.site("title/" + mangaDexManga.ID)
		manga := mangaDexManga.toSource(pr.language)
		mangas = append(mangas, manga)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pr.api("manga/?"+params.Encode()), nil)
	if err != nil {
		return source.BrowsePage{}, fmt.Errorf("create browse request in %q: %w", pr.Name(), err)
	}
	resp, err := pr.doWithRateLimitRetry(req)
	if err != nil {
		return source.BrowsePage{}, fmt.Errorf("execute browse request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return source.BrowsePage{}, fmt.Errorf("browse request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}
	var payload mangaDexResponse[mangaDexManga]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return source.BrowsePage{}, fmt.Errorf("decode browse response in %q: %w", pr.Name(), err)
	}
	result := source.BrowsePage{Offset: payload.Offset, Total: payload.Total}
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		item.URL = pr.site("title/" + item.ID)
		tags := make([]string, 0, len(item.Attributes.Tags))
		for _, tag := range item.Attributes.Tags {
			if name := localizedValue(tag.Attributes.Name, pr.language); name != "" {
				tags = append(tags, name)
			}
		}
		result.Titles = append(result.Titles, source.BrowseTitle{Manga: item.toSource(pr.language), AvailableLanguages: append([]string(nil), item.Attributes.AvailableTranslatedLanguages...), Tags: tags, CreatedAt: item.Attributes.CreatedAt, UpdatedAt: item.Attributes.UpdatedAt})
	}
	result.NextOffset = payload.Offset + len(payload.Data)
	result.HasMore = len(payload.Data) > 0 && (payload.Total == 0 || result.NextOffset < payload.Total)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pr.api("manga/"+url.PathEscape(id)), nil)
	if err != nil {
		return nil, fmt.Errorf("create title request in %q: %w", pr.Name(), err)
	}
	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute title request in %q: %w", pr.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("title %q not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("title request in %q returned unexpected status: %s", pr.Name(), resp.Status)
	}

	var payload struct {
		Data mangaDexManga `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode title response in %q: %w", pr.Name(), err)
	}
	if payload.Data.ID == "" {
		return nil, fmt.Errorf("title response in %q did not include an id", pr.Name())
	}
	payload.Data.URL = pr.site("title/" + payload.Data.ID)
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
