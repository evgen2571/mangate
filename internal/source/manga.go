package source

type Manga struct {
	ID          string
	URL         string
	Title       string
	Description map[string]string
	Chapters    []*Chapter
	Cover       string
}
