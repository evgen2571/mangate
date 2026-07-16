package interactive

import "github.com/evgen2571/mangate/internal/source"

type searchDone struct {
	query   string
	results []*source.Manga
	err     error
}
type chaptersDone struct {
	manga    *source.Manga
	chapters []*source.Chapter
	err      error
	all      bool
}
type downloadProgress struct {
	completed, total, completedChapters, totalChapters int
	active                                             string
}
type downloadDone struct {
	err                        error
	completed, skipped, failed int
	paths                      []string
}
