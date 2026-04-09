package mangadex

type MangaDex struct {
	BaseURL        string
	UploadsBaseURL string
	HomeBaseURL    string
}

type MangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
}

func GetProviderObject() MangaDex {
	return MangaDex{
		BaseURL:        "https://api.mangadex.org/",
		UploadsBaseURL: "https://uploads.mangadex.org/",
		HomeBaseURL:    "https://api.mangadex.org/at-home/",
	}
}
