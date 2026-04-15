package tui

import (
	"github.com/charmbracelet/bubbles/help"
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

type resultsKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	MetaUp   key.Binding
	MetaDown key.Binding
	Back     key.Binding
}

func newResultsKeyMap() resultsKeyMap {
	return resultsKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		MetaUp: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "meta up"),
		),
		MetaDown: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "meta down"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
	}
}

type resultsHelpKeyMap struct {
	global keyMap
	local  resultsKeyMap
}

func (k resultsHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Up,
		k.local.Down,
		k.local.MetaDown,
		k.local.Back,
		k.global.Help,
		k.global.Quit,
	}
}

func (k resultsHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Back},
		{k.local.MetaUp, k.local.MetaDown},
		{k.global.Help, k.global.Quit},
	}
}

func (m resultsModel) HelpKeys(global keyMap) help.KeyMap {
	return resultsHelpKeyMap{
		global: global,
		local:  m.keys,
	}
}
