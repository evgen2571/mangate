package providers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/evgen2571/manga-downloader/internal/config"
	"github.com/evgen2571/manga-downloader/internal/providers/mangadex"
)

type Registry struct {
	factories map[string]Factory
}

func NewDefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register("mangadex", func(cfg config.Config, client *http.Client) (Provider, error) {
		return mangadex.New(cfg, client)
	})

	return r
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

func (r *Registry) Register(name string, factory Factory) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		panic("providers: register called with empty name")
	}
	if factory == nil {
		panic("providers: register called with nil factory")
	}
	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("providers: provider %q already registered", name))
	}

	r.factories[name] = factory
}

func (r *Registry) New(name string, cfg config.Config, client *http.Client) (Provider, error) {
	name = strings.TrimSpace(strings.ToLower(name))

	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %v (available: %s)", name, strings.Join(r.Names(), ", "))
	}

	return factory(cfg, client)
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
