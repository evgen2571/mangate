package source

import (
	"fmt"
	"strings"
)

type Chapter struct {
	URL       string
	ID        string
	Index     string
	Title     string
	PageCount int
	Pages     []*Page
	From      *Manga
}

func (c *Chapter) DisplayTitle(fallbackIndex int) string {
	if c == nil {
		return fmt.Sprintf("Unknown chapter #%d", fallbackIndex+1)
	}

	index, title := c.trimmedIndexAndTitle()
	switch {
	case index != "" && title != "":
		return fmt.Sprintf("Chapter %s - %s", index, title)
	case index != "":
		return fmt.Sprintf("Chapter %s", index)
	case title != "":
		return title
	default:
		return fmt.Sprintf("Unknown chapter #%d", fallbackIndex+1)
	}
}

func (c *Chapter) DisplayName() string {
	if c == nil {
		return "Unknown chapter"
	}

	index, title := c.trimmedIndexAndTitle()
	switch {
	case index != "" && title != "":
		return fmt.Sprintf("Chapter %s - %s", index, title)
	case index != "":
		return fmt.Sprintf("Chapter %s", index)
	case title != "":
		return title
	default:
		return "Unknown chapter"
	}
}

func (c *Chapter) LogName() string {
	if c == nil {
		return "unknown chapter"
	}

	index, title := c.trimmedIndexAndTitle()
	switch {
	case index != "" && title != "":
		return fmt.Sprintf("chapter %s (%s)", index, title)
	case index != "":
		return fmt.Sprintf("chapter %s", index)
	case title != "":
		return title
	default:
		return "unknown chapter"
	}
}

func (c *Chapter) DownloadDirName() string {
	if c == nil {
		return "unknown-chapter"
	}

	index, title := c.trimmedIndexAndTitle()
	switch {
	case index != "" && title != "":
		return "Chapter-" + index + "-" + title
	case index != "":
		return "Chapter-" + index
	case title != "":
		return "Title-" + title
	default:
		return "unknown-chapter"
	}
}

func (c *Chapter) trimmedIndexAndTitle() (string, string) {
	if c == nil {
		return "", ""
	}
	return strings.TrimSpace(c.Index), strings.TrimSpace(c.Title)
}
