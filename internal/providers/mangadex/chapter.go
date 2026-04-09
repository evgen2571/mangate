package mangadex

import (
	"encoding/json"

	"github.com/evgen2571/manga-downloader/internal/client"
	"github.com/evgen2571/manga-downloader/internal/sources"
)

func (md *MangaDex) GetChapters(id string) ([]*sources.Chapter, error) {
	url := md.BaseURL + "manga/" + id + "/feed"

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

	var chapters []*sources.Chapter
	// for _, mangaDexChapter := range mangaDexResponse.Data {
	// 	chapter := mangaDexChapter.toSource()
	// 	chapters = append(chapters, chapter)
	// }

	return chapters, nil
}
