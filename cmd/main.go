package main

import (
	"github.com/evgen2571/manga-downloader/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		panic(err)
	}
}
