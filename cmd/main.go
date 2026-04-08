package main

import (
	"flag"
	"fmt"
	"os"

	"evgen2571/manga-downloader/internal/api"
)

func main() {
	client := api.NewClient()

	if len(os.Args) < 2 {
		os.Exit(1)
	}

	switch os.Args[1] {
	case "search":
		searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
		searchCmd.Parse(os.Args[2:])

		if searchCmd.NArg() < 1 {
			fmt.Println("provide manga title")
			os.Exit(1)
		}

		title := searchCmd.Arg(0)

		resp, err := client.GetManga(title)
		if err != nil {
			panic(err)
		}

		for idx, manga := range resp.Data {
			fmt.Printf("%v. Title: %v\n   Id: %v\n\n", idx+1, manga.Attributes.Title, manga.Id)
		}

	case "chapters":
		chaptersCmd := flag.NewFlagSet("chapters", flag.ExitOnError)
		chaptersCmd.Parse(os.Args[2:])

		if chaptersCmd.NArg() < 1 {
			fmt.Println("provide manga id")
			os.Exit(1)
		}

		mangaId := chaptersCmd.Arg(0)
		fmt.Println(mangaId)

		// TODO: Client request to get available manga chapters

	case "download":
		downloadCmd := flag.NewFlagSet("download", flag.ExitOnError)

		// Flags
		downloadType := downloadCmd.String("type", "plain", "download type")
		downloadDir := downloadCmd.String("dir", ".", "directory where to save downloaded chapter")

		downloadCmd.Parse(os.Args[2:])

		if downloadCmd.NArg() < 1 {
			fmt.Println("provide chapter id")
			os.Exit(1)
		}

		chapterId := downloadCmd.Arg(0)
		fmt.Println(chapterId, *downloadType, *downloadDir)

		// TODO: Downloader request to download manga chapter
	}
}
