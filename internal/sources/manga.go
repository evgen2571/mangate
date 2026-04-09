package sources

type Manga struct {
	ID          string
	URL         string
	Title       string
	Description map[string]string
	Chapters    []Chapter
}

func (m *Manga) GetID() string {
	return m.ID
}

func (m *Manga) GetURL() string {
	return m.URL
}

func (m *Manga) GetTitle() string {
	return m.Title
}
