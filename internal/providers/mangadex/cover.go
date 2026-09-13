package mangadex

import (
	"context"
	"fmt"
	"net/url"

	"github.com/evgen2571/mangate/internal/source"
)

type mangaDexCover struct {
	ID         string `json:"id"`
	Attributes struct {
		Filename string `json:"fileName"`
	} `json:"attributes"`
}

func (pr *Provider) Cover(ctx context.Context, manga *source.Manga) (string, error) {
	params := url.Values{"manga[]": {manga.ID}}
	var mangaDexResponse mangaDexResponse[mangaDexCover]
	if err := pr.getJSON(ctx, "cover for "+manga.ID, "cover?"+params.Encode(), &mangaDexResponse); err != nil {
		return "", err
	}

	if len(mangaDexResponse.Data) == 0 {
		return "", fmt.Errorf("cover response in %q did not contain cover data for manga %q", pr.Name(), manga.ID)
	}

	filename := mangaDexResponse.Data[0].Attributes.Filename
	if !validPathSegment(filename) {
		return "", fmt.Errorf("cover response for manga %q has an invalid filename", manga.ID)
	}
	coverURL := pr.uploads("covers/" + url.PathEscape(manga.ID) + "/" + url.PathEscape(filename))

	return coverURL, nil
}
