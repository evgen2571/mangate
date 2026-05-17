package mangadex

type mangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
	Total    int    `json:"total"`
}

type mangaDexPageResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}
