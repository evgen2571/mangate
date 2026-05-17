package source

type Manga struct {
	ID       string
	URL      string
	Title    string
	Chapters []*Chapter
	Metadata MangaMetadata
}

type MangaMetadata struct {
	Description  map[string]string
	ChapterCount int
}
