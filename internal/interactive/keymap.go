package interactive

import "github.com/charmbracelet/bubbles/key"

type keymap struct {
	up, down, confirm, back, quit, help, filter, toggle, selectAll, clear, rangeSelect key.Binding
}

func newKeymap() keymap {
	return keymap{
		up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		confirm:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
		back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		quit:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		toggle:      key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		selectAll:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all visible")),
		clear:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "clear")),
		rangeSelect: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "select range")),
	}
}

func (m *model) bindings() []key.Binding {
	k := newKeymap()
	switch m.screen {
	case chaptersScreen:
		return []key.Binding{k.up, k.down, k.toggle, k.confirm, k.filter, k.help}
	case workingScreen:
		return []key.Binding{key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "cancel download"))}
	case searchScreen:
		return []key.Binding{k.confirm, key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "settings")), k.quit, k.help}
	default:
		return []key.Binding{k.up, k.down, k.confirm, k.back, k.quit, k.help}
	}
}
