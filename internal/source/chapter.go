package source

import (
	"fmt"
	"strings"

	"github.com/evgen2571/mangate/internal/util"
)

type Chapter struct {
	URL          string  `json:"url,omitempty"`
	ID           string  `json:"id"`
	Volume       string  `json:"volume,omitempty"`
	Index        string  `json:"number,omitempty"`
	Title        string  `json:"title,omitempty"`
	Language     string  `json:"language,omitempty"`
	ReleaseGroup string  `json:"releaseGroup,omitempty"`
	PublishedAt  string  `json:"publishedAt,omitempty"`
	PageCount    int     `json:"pageCount,omitempty"`
	Pages        []*Page `json:"pages,omitempty"`
	From         *Manga  `json:"-"`
}

func (c *Chapter) DisplayTitle(fallbackIndex int) string {
	if c == nil {
		return fmt.Sprintf("Unknown chapter #%d", fallbackIndex+1)
	}

	index, title := c.trimmedIndexAndTitle()
	switch {
	case index != "" && title != "":
		return fmt.Sprintf("Chapter %s - %s", util.SanitizeTerminalText(index), util.SanitizeTerminalText(title))
	case index != "":
		return fmt.Sprintf("Chapter %s", util.SanitizeTerminalText(index))
	case title != "":
		return util.SanitizeTerminalText(title)
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
		return fmt.Sprintf("Chapter %s - %s", util.SanitizeTerminalText(index), util.SanitizeTerminalText(title))
	case index != "":
		return fmt.Sprintf("Chapter %s", util.SanitizeTerminalText(index))
	case title != "":
		return util.SanitizeTerminalText(title)
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
		return fmt.Sprintf("chapter %s (%s)", util.SanitizeTerminalText(index), util.SanitizeTerminalText(title))
	case index != "":
		return fmt.Sprintf("chapter %s", util.SanitizeTerminalText(index))
	case title != "":
		return util.SanitizeTerminalText(title)
	default:
		return "unknown chapter"
	}
}

func (c *Chapter) trimmedIndexAndTitle() (string, string) {
	if c == nil {
		return "", ""
	}
	return strings.TrimSpace(c.Index), strings.TrimSpace(c.Title)
}
