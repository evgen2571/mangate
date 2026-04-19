package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/cli"
	"github.com/evgen2571/mangate/internal/config"
)

func main() {
	cfg := config.DefaultConfig()

	a, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	rootCmd := cli.NewRootCmd(a)
	execErr := rootCmd.Execute()
	closeErr := a.Close()
	if err := errors.Join(execErr, closeErr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
