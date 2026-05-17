package tui

import "github.com/evgen2571/mangate/internal/tuiapp"

func nonNilChapterCount(chapters []tuiapp.ChapterItem) int {
	count := 0
	for _, chapter := range chapters {
		if isChapterItemSet(chapter) {
			count++
		}
	}
	return count
}
