package source

type Chapter struct {
	URL       string
	ID        string
	Index     string
	Title     string
	PageCount int
	Pages     []*Page
	From      *Manga
}
