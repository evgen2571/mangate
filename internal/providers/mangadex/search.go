package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/evgen2571/manga-downloader/internal/source"
)

type mangaDexManga struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		TitleMap    map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		Status      string            `json:"status"`
	} `json:"attributes"`
	Cover string
}

func (pr *Provider) Search(title string) ([]*source.Manga, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("limit", "100") // set maximum possible limit

	url := pr.baseURL + "manga/?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create `search` request in `%s`: %v", pr.Name(), err)
	}

	resp, err := pr.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from `%s`: %v", pr.Name(), err)
	}
	defer resp.Body.Close()

	var mangaDexResponse mangaDexResponse[mangaDexManga]
	err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse)
	if err != nil {
		return nil, err
	}

	var mangas []*source.Manga
	for _, mangaDexManga := range mangaDexResponse.Data {
		mangaDexManga.URL = pr.siteURL + "title/" + mangaDexManga.ID
		manga := mangaDexManga.toSource()
		mangas = append(mangas, manga)
	}

	return mangas, nil
}

func (mdm *mangaDexManga) getTitle() string {
	title := ""
	for _, t := range mdm.Attributes.TitleMap {
		title = t
		break
	}

	return title
}

func (mdm *mangaDexManga) toSource() *source.Manga {
	return &source.Manga{
		ID:          mdm.ID,
		URL:         mdm.URL,
		Title:       mdm.getTitle(),
		Description: mdm.Attributes.Description,
		Cover:       mdm.Cover,
	}
}
