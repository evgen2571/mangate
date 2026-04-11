package sources

type Chapter struct {
	ID    string
	Title string
	Index int
	Pages []*Page
	From  *Manga
}

func (c *Chapter) GetID() string {
	return c.ID
}

func (c *Chapter) GetTitle() string {
	return c.Title
}
