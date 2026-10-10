package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/evgen2571/mangate/internal/source"
)

type Deps struct {
	ProviderResolver ProviderResolver
	Cache            CoverCache
	Timeout          time.Duration
}

type Service struct {
	deps Deps
}

func New(deps Deps) Service {
	return Service{deps: deps}
}

func (s Service) SearchManga(ctx context.Context, query string) ([]*source.Manga, error) {
	provider, err := s.provider()
	if err != nil {
		return nil, err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return provider.Search(ctx, query)
}

func (s Service) Chapters(ctx context.Context, manga *source.Manga) ([]*source.Chapter, error) {
	if manga == nil {
		return nil, fmt.Errorf("load chapters: nil manga")
	}

	provider, err := s.provider()
	if err != nil {
		return nil, err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return provider.Chapters(ctx, manga)
}

func (s Service) ChaptersByID(ctx context.Context, id string) ([]*source.Chapter, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("load chapters: manga id cannot be empty")
	}

	return s.Chapters(ctx, &source.Manga{ID: id, Title: id})
}

func (s Service) CoverPath(ctx context.Context, manga *source.Manga) (string, error) {
	if manga == nil {
		return "", fmt.Errorf("load cover: nil manga")
	}

	if s.deps.Cache == nil {
		return "", fmt.Errorf("load cover: cache is not configured")
	}

	provider, err := s.provider()
	if err != nil {
		return "", err
	}

	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	return s.deps.Cache.Get(ctx, provider, manga)
}

func (s Service) provider() (Provider, error) {
	if s.deps.ProviderResolver == nil {
		return nil, fmt.Errorf("provider: resolver is not configured")
	}

	provider, err := s.deps.ProviderResolver.Provider()
	if err != nil {
		return nil, err
	}

	if provider == nil {
		return nil, fmt.Errorf("provider: resolver returned nil provider")
	}

	return provider, nil
}

func (s Service) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = ctxOrBackground(ctx)
	if s.deps.Timeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, s.deps.Timeout)
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}
