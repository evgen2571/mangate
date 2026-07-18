package source

type BrowseRequest struct {
	Limit             int      `json:"limit"`
	Offset            int      `json:"offset"`
	OriginalLanguages []string `json:"originalLanguages,omitempty"`
	ChapterLanguages  []string `json:"chapterLanguages,omitempty"`
	Statuses          []string `json:"statuses,omitempty"`
	ContentRatings    []string `json:"contentRatings,omitempty"`
	IncludedTags      []string `json:"includedTags,omitempty"`
	ExcludedTags      []string `json:"excludedTags,omitempty"`
	OrderBy           string   `json:"orderBy,omitempty"`
	OrderDirection    string   `json:"orderDirection,omitempty"`
}

type BrowseTitle struct {
	Manga              *Manga   `json:"manga"`
	AvailableLanguages []string `json:"availableLanguages,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"`
	UpdatedAt          string   `json:"updatedAt,omitempty"`
}

type BrowsePage struct {
	Titles     []BrowseTitle `json:"titles"`
	Offset     int           `json:"offset"`
	NextOffset int           `json:"nextOffset,omitempty"`
	HasMore    bool          `json:"hasMore"`
	Total      int           `json:"total,omitempty"`
}
