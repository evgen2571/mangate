package mangadex

import "fmt"

const maxCollectionResults = 10000

type mangaDexResponse[T any] struct {
	Result   string `json:"result"`
	Response string `json:"response"`
	Data     []T    `json:"data"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
	Total    int    `json:"total"`
}

func (r mangaDexResponse[T]) nextOffset(offset, limit int) (int, bool, error) {
	if r.Data == nil || r.Offset != offset || r.Total < 0 || r.Limit < 0 || len(r.Data) > limit {
		return 0, false, fmt.Errorf("invalid collection response at offset %d", offset)
	}
	next := offset + len(r.Data)
	if len(r.Data) > 0 && next > r.Total {
		return 0, false, fmt.Errorf("collection page at offset %d exceeds reported total %d", offset, r.Total)
	}
	if len(r.Data) == 0 && next < r.Total {
		return 0, false, fmt.Errorf("incomplete collection response at offset %d of %d", offset, r.Total)
	}
	more := next < r.Total
	if more && next >= maxCollectionResults {
		return 0, false, fmt.Errorf("MangaDex result limit reached; narrow the query with filters")
	}
	return next, more, nil
}

type mangaDexPageResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}
