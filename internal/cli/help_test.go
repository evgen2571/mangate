package cli

import (
	"testing"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
)

func TestCoreCommandsHaveBuiltInExamples(t *testing.T) {
	application, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	root := NewRootCmd(application)
	for _, name := range []string{"providers", "provider", "search", "title", "chapters", "download", "tui", "archive", "diagnostics", "completion"} {
		command, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("find %q: %v", name, err)
		}
		if command.Example == "" {
			t.Errorf("%s has no example", command.CommandPath())
		}
	}
}
