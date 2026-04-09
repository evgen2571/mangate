package sources

type Page struct {
	ID    string
	URL   string
	Title string
	From  *Chapter
}

func (p *Page) GetID() string {
	return p.ID
}

func (p *Page) GetURL() string {
	return p.URL
}

func (p *Page) GetTitle() string {
	return p.Title
}
