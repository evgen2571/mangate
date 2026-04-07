package api

import (
	"net/http"
	"time"
)

type MangaDexClient struct {
	httpClient *http.Client

	baseUrl    string
	uploadsUrl string
}

type MangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
}

func NewClient() MangaDexClient {
	return MangaDexClient{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},

		baseUrl:    "https://api.mangadex.org/manga",
		uploadsUrl: "https://uploads.mangadex.org/data",
	}
}
