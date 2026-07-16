package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Quit    key.Binding
	Help    key.Binding
	Suspend key.Binding
	Config  key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Suspend: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "suspend"),
		),
		Config: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "config"),
		),
	}
}

type searchKeyMap struct {
	Submit   key.Binding
	Clear    key.Binding
	Complete key.Binding
	Previous key.Binding
	Next     key.Binding
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
		Complete: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "complete"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "history prev"),
		),
		Next: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "history next"),
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
	Download key.Binding
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
		Download: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "full"),
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
		k.local.Complete,
		k.local.Previous,
		k.local.Next,
		k.local.Clear,
		k.global.Config,
		k.global.Help,
		k.global.Quit,
	}
}

func (k searchHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Submit, k.local.Complete, k.local.Previous, k.local.Next, k.local.Clear},
		{k.global.Config, k.global.Help, k.global.Suspend, k.global.Quit},
	}
}

func (k loadingHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.global.Help,
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
		k.local.Download,
		k.local.Back,
		k.global.Help,
		k.global.Quit,
	}
}

func (k resultsHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Select, k.local.Download, k.local.Back},
		{k.local.MetaUp, k.local.MetaDown},
		{k.global.Config, k.global.Help, k.global.Suspend, k.global.Quit},
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
	Up          key.Binding
	Down        key.Binding
	Toggle      key.Binding
	Filter      key.Binding
	SelectAll   key.Binding
	DeselectAll key.Binding
	Download    key.Binding
	Back        key.Binding
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
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle chapter"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all"),
		),
		DeselectAll: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "clear"),
		),
		Download: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "download"),
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
		k.local.Toggle,
		k.local.Filter,
		k.local.SelectAll,
		k.local.DeselectAll,
		k.local.Download,
		k.global.Help,
		k.global.Quit,
	}
}

func (k chaptersHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Toggle, k.local.Filter, k.local.SelectAll, k.local.DeselectAll},
		{k.local.Download, k.local.Back},
		{k.global.Config, k.global.Help, k.global.Suspend, k.global.Quit},
	}
}

type configKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Edit    key.Binding
	Confirm key.Binding
	Apply   key.Binding
	Save    key.Binding
	Back    key.Binding
}

func newConfigKeyMap() configKeyMap {
	return configKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Edit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit/confirm"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Apply: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "apply session"),
		),
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save + apply"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back/cancel"),
		),
	}
}

type configHelpKeyMap struct {
	global keyMap
	local  configKeyMap
}

func (k configHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.local.Up,
		k.local.Down,
		k.local.Edit,
		k.local.Apply,
		k.local.Save,
		k.local.Back,
		k.global.Help,
		k.global.Quit,
	}
}

func (k configHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.local.Up, k.local.Down, k.local.Edit, k.local.Apply, k.local.Save, k.local.Back},
		{k.global.Help, k.global.Suspend, k.global.Quit},
	}
}
