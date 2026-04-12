package mangadex

import (
	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/source"
)

type mangaDexManga struct {
	ID         string `json:"id"`
	URL        string
	Attributes struct {
		TitleMap    map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		Status      string            `json:"status"`
		Tag []struct {
			Attributes struct {
				Name struct {
					Genre string `json:"en"`
				} `json:"name"`
			} `json:"attributes"`
		} `json:"tags"`
		AvailableTranslatedLanguages []string `json:"availableTranslatedLanguages"`
	} `json:"attributes"`
	Cover string
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
	m := &source.Manga {
		ID:          mdm.ID,
		URL:         mdm.URL,
		Title:       mdm.getTitle(),
	}
	
	description, exist := mdm.Attributes.Description[config.DefaultLanguage]
	if !exist {
		description = "No description"
	}
	
	var genres []string
    for _, tag := range mdm.Attributes.Tag {
    genre := tag.Attributes.Name.Genre
    genres = append(genres, genre)
    }
    
    m.Metadata.Genres = genres
    m.Metadata.Description = description
    m.Metadata.AvailableLanguages = mdm.Attributes.AvailableTranslatedLanguages
	
	return m
}