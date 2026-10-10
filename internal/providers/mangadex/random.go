package mangadex

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/evgen2571/mangate/internal/source"
)

func (pr *Provider) RandomManga(ctx context.Context) (*source.Manga, error) {
	var payload struct {
		Data mangaDexManga `json:"data"`
	}
	if err := pr.getJSON(ctx, "random manga", "manga/random", &payload); err != nil {
		return nil, err
	}
	if !validPathSegment(payload.Data.ID) {
		return nil, fmt.Errorf("random manga response has no valid id")
	}
	payload.Data.URL = pr.site("title/" + url.PathEscape(payload.Data.ID))
	return payload.Data.toSource(pr.language), nil
}

// MangaDex permits 60 random requests/minute; retries use the same gate.
func (pr *Provider) paceRandomRequest(ctx context.Context) error {
	select {
	case pr.randomGate <- struct{}{}:
		defer func() { <-pr.randomGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := sleepWithContext(ctx, time.Until(pr.lastRandomRequest.Add(time.Second))); err != nil {
		return err
	}
	pr.lastRandomRequest = time.Now()
	return nil
}
