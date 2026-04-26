package source

type Manga struct {
	ID       string
	URL      string
	Title    string
	Chapters []*Chapter
	Cover    Cover
	Metadata struct {
		Description  map[string]string
		ChapterCount int
	}
}

type Cover struct {
	URL      string
	FileName string
}
