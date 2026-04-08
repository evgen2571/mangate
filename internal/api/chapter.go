package api

import (
	"net/http"
	"encoding/json"
)

type Chapter struct {
	ID string `json:"id"`
	Type string  `json:"type"`
	
	ChapterInfo struct {
		Pages []string `json:"data"`
	} `json:"chapter"`
}

func (c *MangaDexClient) GetChapters(id string) (MangaDexResponse[Chapter], error) {
	url := c.baseUrl + "manga/" + id + "/feed"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return MangaDexResponse[Chapter]{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MangaDexResponse[Chapter]{}, err
	}
	defer resp.Body.Close()

	var mangaDexResponse MangaDexResponse[Chapter]
	err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse)
	if err != nil {
		return MangaDexResponse[Chapter]{}, err
	}
	
	for idx, chapter := range mangaDexResponse.Data {
		url := c.baseUrl + "at-home/server/" + chapter.ID

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return MangaDexResponse[Chapter]{}, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return MangaDexResponse[Chapter]{}, err
		}

		err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse.Data[idx])
		resp.Body.Close()
		if err != nil {
			return MangaDexResponse[Chapter]{}, err
		}
		
	}

	return mangaDexResponse, nil
}