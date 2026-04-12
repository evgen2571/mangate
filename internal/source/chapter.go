package source

type Chapter struct {
	URL   string
	ID    string
	Index string
	Title string
	Pages []*Page
	From  *Manga
}

func (c *Chapter) GetID() string {
	return c.ID
}

func (c *Chapter) GetTitle() string {
	return c.Title
}
