package source

type Manga struct {
	ID       string
	URL      string
	Title    string
	Chapters []*Chapter
	Cover    Cover
	Metadata MangaMetadata
}

type MangaMetadata struct {
	Description  map[string]string
	ChapterCount int
}

type Cover struct {
	URL      string
	FileName string
}
