package main

import (
	// "fmt"
	// "os"

	// tea "github.com/charmbracelet/bubbletea"
	// "github.com/evgen2571/manga-downloader/internal/tui"
	"github.com/evgen2571/manga-downloader/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
			panic(err)
		}
}

//	p := tea.NewProgram(
//		New(),
//		tea.WithAltScreen(),
//	)
//
//	_, err := p.Run()
//	return err
