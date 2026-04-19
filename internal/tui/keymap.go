package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Suspend key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Suspend: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "suspend"),
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

type resultsKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
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
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open chapters"),
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

func (k searchHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Submit,
		k.local.Clear,
		k.global.Help,
		k.global.Suspend,
		k.global.Quit,
	}
}

func (k searchHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Submit, k.local.Clear},
		{k.global.Help, k.global.Suspend, k.global.Quit},
	}
}

func (k loadingHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.global.Help,
		k.global.Suspend,
		k.global.Quit,
	}
}

func (k loadingHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.global.Help, k.global.Suspend, k.global.Quit},
	}
}

func (k resultsHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Up,
		k.local.Down,
		k.local.Select,
		k.local.MetaDown,
		k.local.Back,
		k.global.Help,
		k.global.Suspend,
		k.global.Quit,
	}
}

func (k resultsHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Select, k.local.Back},
		{k.local.MetaUp, k.local.MetaDown},
		{k.global.Help, k.global.Suspend, k.global.Quit},
	}
}

func (m resultsModel) HelpKeys(global keyMap) help.KeyMap {
	return resultsHelpKeyMap{
		global: global,
		local:  m.keys,
	}
}

type chaptersHelpKeyMap struct {
	global keyMap
	local  chaptersKeyMap
}

type chaptersKeyMap struct {
	Up   key.Binding
	Down key.Binding
	Back key.Binding
}

func newChaptersKeyMap() chaptersKeyMap {
	return chaptersKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k chaptersHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Up,
		k.local.Down,
		k.local.Back,
		k.global.Help,
		k.global.Suspend,
		k.global.Quit,
	}
}

func (k chaptersHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Back},
		{k.global.Help, k.global.Suspend, k.global.Quit},
	}
}
