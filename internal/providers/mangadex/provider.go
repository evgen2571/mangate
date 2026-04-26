package mangadex

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/evgen2571/mangate/internal/config"
)

const defaultAtHomeMinInterval = 1500 * time.Millisecond

type Provider struct {
	client     *http.Client
	siteURL    string
	baseURL    string
	uploadsURL string
	language   string

	atHomeMu          sync.Mutex
	lastAtHomeRequest time.Time
	atHomeMinInterval time.Duration
}

func New(cfg config.Config, client *http.Client) (*Provider, error) {
	return &Provider{
		client:            client,
		siteURL:           cfg.Providers.MangaDex.SiteURL,
		baseURL:           cfg.Providers.MangaDex.BaseURL,
		uploadsURL:        cfg.Providers.MangaDex.UploadsURL,
		language:          cfg.Language,
		atHomeMinInterval: defaultAtHomeMinInterval,
	}, nil
}

func (pr *Provider) Name() string {
	return "mangadex"
}

func (pr *Provider) api(path string) string {
	return strings.TrimRight(pr.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (pr *Provider) site(path string) string {
	return strings.TrimRight(pr.siteURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (pr *Provider) uploads(path string) string {
	return strings.TrimRight(pr.uploadsURL, "/") + "/" + strings.TrimLeft(path, "/")
}
