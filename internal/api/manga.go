package api

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type Manga struct {
	Id         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes MangaAttributes `json:"attributes"`
}

type MangaAttributes struct {
	Title       map[string]string `json:"title"`
	Description map[string]string `json:"description"`
	Status      string            `json:"status"`
}

func (m *Manga) GetId() string {
	return m.Id
}

func (c *MangaDexClient) GetManga(title string) (MangaDexResponse[Manga], error) {
	params := url.Values{}
	params.Set("title", title)

	url := c.baseUrl + "?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return MangaDexResponse[Manga]{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MangaDexResponse[Manga]{}, err
	}
	defer resp.Body.Close()

	var mangaDexResponse MangaDexResponse[Manga]
	err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse)
	if err != nil {
		return MangaDexResponse[Manga]{}, err
	}

	return mangaDexResponse, nil
}
