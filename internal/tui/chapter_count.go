package tui

import "github.com/evgen2571/mangate/internal/source"

func nonNilChapterCount(chapters []*source.Chapter) int {
	count := 0
	for _, chapter := range chapters {
		if chapter != nil {
			count++
		}
	}
	return count
}
