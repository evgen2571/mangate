package sources

type Page struct {
	Index int
	URL   string
	From  *Chapter
}

func (p *Page) GetURL() string {
	return p.URL
}
