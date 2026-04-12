package source

type Chapter struct {
	URL   string
	ID    string
	Index string
	Title string
	Pages []*Page
	From  *Manga
}
