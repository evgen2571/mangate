package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit key.Binding
	Help key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "esc"),
			key.WithHelp("esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

type searchKeyMap struct {
	Submit key.Binding
	Clear  key.Binding
}

func newSearchKeyMap() searchKeyMap {
	return searchKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "search"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear"),
		),
	}
}

type searchHelpKeyMap struct {
	global keyMap
	local  searchKeyMap
}

func (k searchHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Submit,
		k.local.Clear,
		k.global.Help,
		k.global.Quit,
	}
}

func (k searchHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Submit, k.local.Clear},
		{k.global.Help, k.global.Quit},
	}
}
