package cli

import (
	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/config"
	"testing"
)

func TestIntegrationFlagsRequireResultFile(t *testing.T) {
	a, err := app.New(config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd(a)
	cmd.SetArgs([]string{"tui", "--exit-on-complete"})
	err = cmd.Execute()
	if err == nil || err.Error() != "--exit-on-complete requires --result-file" {
		t.Fatalf("error = %v", err)
	}
}
