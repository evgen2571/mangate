package sources

type Page struct {
	URL  string
	From *Chapter
}

func (p *Page) GetURL() string {
	return p.URL
}
