package source

type Manga struct {
	ID          string
	URL         string
	Title       string
	Chapters    []*Chapter
	Cover       string
	Metadata struct {
		Description string
		Genres []string
		AvailableLanguages []string
	}
}
