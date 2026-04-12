package tui

import "github.com/charmbracelet/lipgloss"

type uiStyles struct {
	App       lipgloss.Style
	Logo      lipgloss.Style
	Card      lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	InputWrap lipgloss.Style
	InputBox  lipgloss.Style
	Hint      lipgloss.Style
	Error     lipgloss.Style
	Status    lipgloss.Style

	Pane           lipgloss.Style
	ListItem       lipgloss.Style
	ListItemActive lipgloss.Style
	Muted          lipgloss.Style
	CoverBox       lipgloss.Style
	MetaBox        lipgloss.Style

	PaneTitle    lipgloss.Style
	SectionTitle lipgloss.Style
	Label        lipgloss.Style
	Footer       lipgloss.Style

	ListCard        lipgloss.Style
	ListCardActive  lipgloss.Style
	ListTitle       lipgloss.Style
	ListTitleActive lipgloss.Style
	Index           lipgloss.Style

	ListCardSelected       lipgloss.Style
	ListCardSelectedActive lipgloss.Style
	ListTitleSelected      lipgloss.Style
}

var searchLogo string = `
███╗   ███╗ █████╗ ███╗   ██╗ ██████╗  █████╗
████╗ ████║██╔══██╗████╗  ██║██╔════╝ ██╔══██╗
██╔████╔██║███████║██╔██╗ ██║██║  ███╗███████║
██║╚██╔╝██║██╔══██║██║╚██╗██║██║   ██║██╔══██║
██║ ╚═╝ ██║██║  ██║██║ ╚████║╚██████╔╝██║  ██║
╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝
`

func newUIStyles() uiStyles {
	return uiStyles{
		App: lipgloss.NewStyle().
			Padding(0, 0),

		Logo: lipgloss.NewStyle().
			Bold(true).
			Align(lipgloss.Center).
			MarginBottom(1),

		SectionTitle: lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			MarginBottom(1),

		Footer: lipgloss.NewStyle().
			Faint(true).
			Align(lipgloss.Center),

		Card: lipgloss.NewStyle().
			Width(58).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()),

		Title: lipgloss.NewStyle().
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Faint(true).
			MarginBottom(1),

		InputWrap: lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1),

		InputBox: lipgloss.NewStyle().
			Width(50).
			Padding(0, 1).
			Border(lipgloss.NormalBorder()),

		Hint: lipgloss.NewStyle().
			Faint(true).
			Align(lipgloss.Center).
			MarginTop(1),

		Error: lipgloss.NewStyle().
			Bold(true).
			MarginTop(1),

		Status: lipgloss.NewStyle().
			Faint(true).
			MarginTop(1),

		Pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1),

		PaneTitle: lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			MarginBottom(1),

		ListItem: lipgloss.NewStyle().
			Padding(0, 1),

		ListItemActive: lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true),

		Muted: lipgloss.NewStyle().
			Faint(true),

		Label: lipgloss.NewStyle().
			Bold(true).
			MarginTop(1),

		CoverBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1),

		MetaBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1),

		ListCard: lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			BorderLeft(true),

		ListCardActive: lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Bold(true).
			BorderLeft(true),

		ListTitle: lipgloss.NewStyle(),

		ListTitleActive: lipgloss.NewStyle().
			Bold(true),

		Index: lipgloss.NewStyle().
			Faint(true),

		ListCardSelected: lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Bold(true).
			BorderLeft(true),

		ListCardSelectedActive: lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Bold(true).
			Underline(false).
			BorderLeft(false),

		ListTitleSelected: lipgloss.NewStyle().
			Bold(true).
			Underline(false),
	}
}
