package mangadex

import (
	"encoding/json"
	"net/url"

	"github.com/evgen2571/manga-downloader/internal/client"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type MangaDexManga struct {
	ID         string `json:"id"`
	Attributes struct {
		TitleMap    map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		Status      string            `json:"status"`
	} `json:"attributes"`
}

func (mdm *MangaDexManga) getTitle() string {
	title := ""
	for _, t := range mdm.Attributes.TitleMap {
		title = t
		break
	}

	return title
}

func (md *MangaDex) GetManga(title string) ([]*sources.Manga, error) {
	params := url.Values{}
	params.Set("title", title)

	url := md.BaseURL + "manga/?" + params.Encode()

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var mangaDexResponse MangaDexResponse[MangaDexManga]
	err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse)
	if err != nil {
		return nil, err
	}

	var mangas []*sources.Manga
	for _, mangaDexManga := range mangaDexResponse.Data {
		manga := mangaDexManga.toSource()
		mangas = append(mangas, manga)
	}

	return mangas, nil
}

func (mdm *MangaDexManga) toSource() *sources.Manga {
	baseUrl := MangaDexProvider.BaseURL
	url := baseUrl + mdm.ID + "/feed"
	return &sources.Manga{
		ID:          mdm.ID,
		URL:         url,
		Title:       mdm.getTitle(),
		Description: mdm.Attributes.Description,
	}
}
