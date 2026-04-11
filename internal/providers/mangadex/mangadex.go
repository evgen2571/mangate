package mangadex

import (
	"encoding/json"
	"net/url"

	"github.com/evgen2571/manga-downloader/internal/client"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

type MangaDex struct {
	BaseURL        string
	UploadsBaseURL string
}

type mangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
}

type mangaDexPageResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
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

	var mangaDexResponse mangaDexResponse[mangaDexManga]
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

func (md *MangaDex) GetChapters(manga *sources.Manga) ([]*sources.Chapter, error) {
	url := md.BaseURL + "manga/" + manga.GetID() + "/feed"

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var mangaDexResponse mangaDexResponse[mangaDexChapter]
	if err = json.NewDecoder(resp.Body).Decode(&mangaDexResponse); err != nil {
		return nil, err
	}

	sortChaptersByChapter(mangaDexResponse.Data)

	var chapters []*sources.Chapter
	for _, mangaDexChapter := range mangaDexResponse.Data {
		chapter := mangaDexChapter.toSource()
		chapter.From = manga
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func (md *MangaDex) GetPages(chapter *sources.Chapter) error {
	url := md.BaseURL + "at-home/server/" + chapter.GetID()

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var pageResp mangaDexPageResponse
	if err = json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return err
	}

	pages := pageResp.toSourcePages(md.UploadsBaseURL)
	for _, page := range pages {
		page.From = chapter
	}

	chapter.Pages = pages
	return nil
}
