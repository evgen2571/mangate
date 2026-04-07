package main

import (
	"fmt"

	"evgen2571/manga-downloader/internal/api"
)

func main() {

	client := api.NewClient()

	resp, err := client.GetManga("Re:Zero")
	if err != nil {
		panic(err)
	}

	for idx, manga := range resp.Data {
		fmt.Printf("%v. %v\n", idx+1, manga.Attributes.Title["ja-ro"])
	}
}
