package sources

type Chapter struct {
	ID       string
	URL      string
	Title    string
	Chapters []Page
	From     *Manga
}

func (c *Chapter) GetID() string {
	return c.ID
}

func (c *Chapter) GetURL() string {
	return c.URL
}

func (c *Chapter) GetTitle() string {
	return c.Title
}
