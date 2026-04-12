package source

type Manga struct {
	ID          string
	URL         string
	Title       string
	Description map[string]string
	Chapters    []*Chapter
	Cover       string
}

func (m *Manga) GetID() string {
	return m.ID
}

func (m *Manga) GetTitle() string {
	return m.Title
}

