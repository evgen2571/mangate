package source

type Manga struct {
	ID       string
	URL      string
	Title    string
	Chapters []*Chapter
	Cover    Cover
	Metadata MangaMetadata
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
	Description  map[string]string
	ChapterCount int
	Status       string
	ContentType  string
}

type Cover struct {
	URL      string
	FileName string
}
