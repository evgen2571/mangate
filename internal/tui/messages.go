package tui

import "github.com/evgen2571/mangate/internal/source"

type searchSubmittedMsg struct {
	Query string
}

type searchSucceededMsg struct {
	Query   string
	Results []*source.Manga
}

type searchFailedMsg struct {
	Err error
}

type goBackMsg struct{}
