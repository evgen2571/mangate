package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evgen2571/mangate/internal/app"
	"github.com/evgen2571/mangate/internal/cli"
	"github.com/evgen2571/mangate/internal/config"
)

func main() {
	cfg, configPath, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}

	a, err := app.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	a.ConfigPath = configPath

	rootCmd := cli.NewRootCmd(a)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		jsonOutput, _ := rootCmd.Flags().GetBool("json")
		var reported *cli.ReportedError
		if errors.As(err, &reported) {
			if !jsonOutput {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(reported.Code)
		}
		if jsonOutput {
			_ = cli.WriteError(os.Stdout, "command", err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(cli.ExitCode(err.Error()))
	}
}
