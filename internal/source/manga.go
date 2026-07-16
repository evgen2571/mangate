package source

type Manga struct {
	ID       string        `json:"id"`
	URL      string        `json:"url,omitempty"`
	Title    string        `json:"title"`
	Chapters []*Chapter    `json:"chapters,omitempty"`
	Cover    Cover         `json:"cover,omitempty"`
	Metadata MangaMetadata `json:"metadata"`
}

// ProviderInfo is static, safe-to-display provider metadata. Its identifier is
// a public compatibility value and must not change once released.
type ProviderInfo struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Version           string   `json:"version"`
	Capabilities      []string `json:"capabilities"`
	Authentication    string   `json:"authentication"`
	Restrictions      []string `json:"restrictions"`
	DownloadPermitted bool     `json:"downloadPermitted"`
	Availability      string   `json:"availability"`
}

type MangaMetadata struct {
	Description  map[string]string `json:"description,omitempty"`
	ChapterCount int               `json:"chapterCount,omitempty"`
	Status       string            `json:"status,omitempty"`
	ContentType  string            `json:"contentType,omitempty"`
}

type Cover struct {
	URL      string `json:"url,omitempty"`
	FileName string `json:"fileName,omitempty"`
}
