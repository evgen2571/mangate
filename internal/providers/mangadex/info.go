package mangadex

import "github.com/evgen2571/mangate/internal/source"

func (pr *Provider) Info() source.ProviderInfo {
	return source.ProviderInfo{
		ID: "mangadex", Name: "MangaDex", Version: "v5", Availability: "available",
		Description: "MangaDex public API adapter.", Authentication: "optional",
		Capabilities:      []string{"search", "title", "chapters", "pages", "download"},
		Restrictions:      []string{"Only download content you are authorized to access.", "Provider availability and terms may change."},
		DownloadPermitted: true,
	}
}
