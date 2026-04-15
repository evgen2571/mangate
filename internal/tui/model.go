package tui

type screen int

const (
	screenSearch = iota
)

type model struct {
	current screen
	width   int
	height  int
	search  searchModel
}
