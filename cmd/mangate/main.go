package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/cli"
	"github.com/evgen2571/mangate/internal/config"
)

func main() {
	cfg, configPath, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	a, err := app.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	a.ConfigPath = configPath

	rootCmd := cli.NewRootCmd(a)
	execErr := rootCmd.Execute()
	closeErr := a.Close()
	if err := errors.Join(execErr, closeErr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
