package mangadex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/evgen2571/manga-downloader/internal/client"
	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/source"
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

type mangaDexCoverResponse struct {
	ID string `json:"id"`
	Attributes struct {
		Filename string `json:"filename"`
	} `json:"attributes"`
	
}

func (md *MangaDex) GetManga(title string) ([]*source.Manga, error) {
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
	
	for _, mangaDexManga := range mangaDexResponse.Data {
		mangaDexManga.Cover, _ = mangaDexManga.getCover()
	}

	var mangas []*source.Manga
	for _, mangaDexManga := range mangaDexResponse.Data {
		manga := mangaDexManga.toSource()
		mangas = append(mangas, manga)
	}

	return mangas, nil
}

func (md *MangaDex) GetChapters(manga *source.Manga) ([]*source.Chapter, error) {
	url := md.BaseURL + "manga/" + manga.GetID() + "/feed?translatedLanguage[]=" + config.DefaultLanguage

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

	var chapters []*source.Chapter
	for _, mangaDexChapter := range mangaDexResponse.Data {
		chapter := mangaDexChapter.toSource()
		chapter.From = manga
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func (md *MangaDex) GetPages(chapter *source.Chapter) ([]*source.Page, error) {
	url := md.BaseURL + "at-home/server/" + chapter.GetID()

	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pageResp mangaDexPageResponse
	if err = json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return nil, err
	}

	pages := pageResp.toSourcePages(md.UploadsBaseURL)
	for _, page := range pages {
		page.From = chapter
	}

	return pages, nil
}

func (mdm *mangaDexManga) getCover() (string, error) {
	url := "https://api.mangadex.org/cover?manga[]=" + mdm.ID
	
	req := client.NewRequest(url, nil)

	resp, err := client.DoRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}
	
	var coverResp mangaDexCoverResponse
	err = json.NewDecoder(resp.Body).Decode(&coverResp)
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}
	
	coverUrl := "https://uploads.mangadex.org/covers/" +  mdm.ID + "/" + coverResp.Attributes.Filename
	
	return coverUrl, nil
}