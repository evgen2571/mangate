package main

import (
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
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
