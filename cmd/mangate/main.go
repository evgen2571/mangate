package main

import (
	"fmt"
	"log"
	"os"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/cli"
	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/util"
)

func main() {
	cfg := config.DefaultConfig()

	a, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	rootCmd := cli.NewRootCmd(a)
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("%v", err)
		util.CleanupTemp(cfg.Dirs.Temp)
		os.Exit(1)
	}
}
