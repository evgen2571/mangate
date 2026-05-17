package tui

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestModelGoDoesNotContainRootUpdateMethod(t *testing.T) {
	hasMethod, err := modelGoDeclaresMethod("Update", "model")
	if err != nil {
		t.Fatalf("modelGoDeclaresMethod(Update, model) error = %v", err)
	}
	if hasMethod {
		t.Fatal("model.go still contains the root Update method")
	}
}

func modelGoDeclaresMethod(methodName, receiverName string) (bool, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return false, errors.New("runtime.Caller(0) failed")
	}

	modelPath := filepath.Join(filepath.Dir(currentFile), "model.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, modelPath, nil, 0)
	if err != nil {
		return false, err
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != methodName || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		if receiverTypeName(fn.Recv.List[0].Type) == receiverName {
			return true, nil
		}
	}

	return false, nil
}

func receiverTypeName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return receiverTypeName(node.X)
	default:
		return ""
	}
}

func TestModelFullMangaDownloadStartsDownloadAfterMatchingChaptersLoad(t *testing.T) {
	manga := tuiapp.MangaDetails{ID: "manga-a", Title: "Manga A"}
	chapters := []tuiapp.ChapterItem{
		{ID: "chapter-a", Index: "1", DisplayText: "Chapter 1"},
		{},
		{ID: "chapter-b", Index: "2", DisplayText: "Chapter 2"},
	}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: manga,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: chapters})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateDownloading {
		t.Fatalf("state = %v, want stateDownloading", got.state)
	}
	if got.pendingFullMangaDownload != (tuiapp.MangaDetails{}) {
		t.Fatalf("pendingFullMangaDownload = %#v, want zero value", got.pendingFullMangaDownload)
	}
	if cmd == nil {
		t.Fatalf("Update() returned nil command")
	}
	if got.downloading.detail != "2 chapters selected" {
		t.Fatalf("downloading detail = %q, want %q", got.downloading.detail, "2 chapters selected")
	}
}

func TestModelFullMangaDownloadIgnoresStaleChaptersLoad(t *testing.T) {
	pending := tuiapp.MangaDetails{ID: "manga-pending", Title: "Pending"}
	stale := tuiapp.MangaDetails{ID: "manga-stale", Title: "Stale"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: pending,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{
		Manga:    stale,
		Chapters: []tuiapp.ChapterItem{{ID: "stale-chapter", Index: "1", DisplayText: "Chapter 1"}},
	})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateChapters {
		t.Fatalf("state = %v, want stateChapters", got.state)
	}
	if got.pendingFullMangaDownload != pending {
		t.Fatalf("pendingFullMangaDownload = %#v, want pending manga %#v", got.pendingFullMangaDownload, pending)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command for stale chapters load")
	}
}

func TestModelFullMangaDownloadShowsStatusWhenNoChaptersLoad(t *testing.T) {
	manga := tuiapp.MangaDetails{ID: "manga-a", Title: "Manga A"}
	m := model{
		state:                    stateLoading,
		pendingFullMangaDownload: manga,
	}

	updated, cmd := m.Update(chaptersLoadedMsg{Manga: manga, Chapters: []tuiapp.ChapterItem{{}}})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.state != stateChapters {
		t.Fatalf("state = %v, want stateChapters", got.state)
	}
	if got.pendingFullMangaDownload != (tuiapp.MangaDetails{}) {
		t.Fatalf("pendingFullMangaDownload = %#v, want zero value", got.pendingFullMangaDownload)
	}
	if got.chapters.status != "no chapters to download" {
		t.Fatalf("chapters status = %q, want %q", got.chapters.status, "no chapters to download")
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command")
	}
}

func TestModelPlainChaptersOpenClearsPendingFullDownload(t *testing.T) {
	manga := tuiapp.MangaDetails{ID: "manga-a", Title: "Manga A"}
	pending := tuiapp.MangaDetails{ID: "pending", Title: "Pending"}
	m := model{pendingFullMangaDownload: pending}

	updated, _ := m.Update(chaptersOpenRequestedMsg{Result: tuiapp.SearchResult{ID: manga.ID, Title: manga.Title, URL: manga.URL}})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}

	if got.pendingFullMangaDownload != (tuiapp.MangaDetails{}) {
		t.Fatalf("pendingFullMangaDownload = %#v, want zero value", got.pendingFullMangaDownload)
	}
	if got.state != stateLoading {
		t.Fatalf("state = %v, want stateLoading", got.state)
	}
}

func TestNewModelLoadsSearchHistoryFromService(t *testing.T) {
	svc := fakeTUIService{history: []string{"Dandadan", "Puniru"}}

	got, ok := newModel(svc).(*model)
	if !ok {
		t.Fatalf("newModel() returned %T, want *model", newModel(svc))
	}

	if len(got.search.history) != 2 || got.search.history[0] != "Dandadan" || got.search.history[1] != "Puniru" {
		t.Fatalf("search history = %#v, want service history", got.search.history)
	}
}

func TestModelConfigSaveUsesTUIAppServiceStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	svc := fakeTUIService{
		configState: configStateFromConfig(cfg),
		saveConfigFn: func(_ context.Context, state tuiapp.ConfigState) (tuiapp.ConfigState, error) {
			state.Language = "ru"
			return state, nil
		},
	}
	m := model{svc: svc, config: newConfigModel(configStateFromConfig(cfg))}

	updated, cmd := m.Update(configSaveRequestedMsg{Config: configStateFromConfig(cfg)})
	got, ok := updated.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", updated)
	}
	if cmd != nil {
		t.Fatalf("Update() returned unexpected command")
	}
	if got.config.status != "saved and applied" {
		t.Fatalf("config status = %q, want %q", got.config.status, "saved and applied")
	}
	if got.config.draft.Language != "ru" {
		t.Fatalf("draft config = %#v, want updated language", got.config.draft)
	}
}

type fakeTUIService struct {
	searchResults  []tuiapp.SearchResult
	searchErr      error
	history        []string
	historyErr     error
	configState    tuiapp.ConfigState
	loadChaptersFn func(context.Context, tuiapp.SearchResult) (tuiapp.MangaDetails, []tuiapp.ChapterItem, error)
	downloadFn     func(context.Context, tuiapp.DownloadRequest, func(tuiapp.DownloadProgress)) error
	applyConfigFn  func(context.Context, tuiapp.ConfigState) (tuiapp.ConfigState, error)
	saveConfigFn   func(context.Context, tuiapp.ConfigState) (tuiapp.ConfigState, error)
}

func (f fakeTUIService) Search(context.Context, string) ([]tuiapp.SearchResult, error) {
	return f.searchResults, f.searchErr
}

func (f fakeTUIService) SearchHistory(context.Context) ([]string, error) {
	return f.history, f.historyErr
}

func (f fakeTUIService) LoadChapters(ctx context.Context, result tuiapp.SearchResult) (tuiapp.MangaDetails, []tuiapp.ChapterItem, error) {
	if f.loadChaptersFn != nil {
		return f.loadChaptersFn(ctx, result)
	}
	return tuiapp.MangaDetails{}, nil, nil
}

func (f fakeTUIService) LoadCover(context.Context, tuiapp.SearchResult, tuiapp.CoverSize) (tuiapp.CoverResult, error) {
	return tuiapp.CoverResult{}, nil
}

func (f fakeTUIService) Download(ctx context.Context, req tuiapp.DownloadRequest, notify func(tuiapp.DownloadProgress)) error {
	if f.downloadFn != nil {
		return f.downloadFn(ctx, req, notify)
	}
	return nil
}

func (f fakeTUIService) Config() tuiapp.ConfigState {
	return f.configState
}

func (f fakeTUIService) ApplyConfig(ctx context.Context, state tuiapp.ConfigState) (tuiapp.ConfigState, error) {
	if f.applyConfigFn != nil {
		return f.applyConfigFn(ctx, state)
	}
	return state, nil
}

func (f fakeTUIService) SaveConfig(ctx context.Context, state tuiapp.ConfigState) (tuiapp.ConfigState, error) {
	if f.saveConfigFn != nil {
		return f.saveConfigFn(ctx, state)
	}
	return state, nil
}
