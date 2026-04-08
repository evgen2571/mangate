package api

import (
	"net/http"
)

type MangaDexClient struct {
	httpClient *http.Client

	baseUrl        string
	uploadsBaseUrl string
}

type MangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
}

func NewClient() MangaDexClient {
	return MangaDexClient{
		httpClient: &http.Client{},
		baseUrl:    "https://api.mangadex.org/manga",
	}
}
