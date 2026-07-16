package tui

import (
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
)

func TestConfigModelUpdateDraftFromCurrentFieldParsesValues(t *testing.T) {
	cfg := config.DefaultConfig()
	m := newConfigModel(cfg)

	cases := []struct {
		field configField
		value string
		want  func(config.Config) bool
	}{
		{configFieldLanguage, "ru", func(c config.Config) bool { return c.Language == "ru" }},
		{configFieldDownloadFormat, "cbz", func(c config.Config) bool { return c.Download.Format == "cbz" }},
		{configFieldExistingFiles, "replace", func(c config.Config) bool { return c.Download.ExistingFileMode == "replace" }},
		{configFieldRetainSource, "false", func(c config.Config) bool { return !c.Download.RetainSource }},
		{configFieldHTTPTimeout, "45s", func(c config.Config) bool { return c.HTTP.Timeout == 45*time.Second }},
		{configFieldPageDownloads, "3", func(c config.Config) bool { return c.Concurrency.PageDownloads == 3 }},
		{configFieldSearchHistoryMax, "25", func(c config.Config) bool { return c.Search.HistoryMax == 25 }},
	}

	for _, tc := range cases {
		m.cursor = int(tc.field)
		m.input.SetValue(tc.value)
		if err := m.updateDraftFromInput(); err != nil {
			t.Fatalf("updateDraftFromInput(%v, %q) error = %v", tc.field, tc.value, err)
		}
		if !tc.want(m.draft) {
			t.Fatalf("draft after field %v value %q = %+v", tc.field, tc.value, m.draft)
		}
	}
}

func TestConfigModelUpdateDraftRejectsInvalidInt(t *testing.T) {
	m := newConfigModel(config.DefaultConfig())
	original := m.draft.Concurrency.PageDownloads
	m.cursor = int(configFieldPageDownloads)
	m.input.SetValue("not-an-int")

	err := m.updateDraftFromInput()
	if err == nil {
		t.Fatal("updateDraftFromInput() error = nil, want error")
	}
	if m.draft.Concurrency.PageDownloads != original {
		t.Fatalf("PageDownloads = %d, want original %d", m.draft.Concurrency.PageDownloads, original)
	}
}

func TestConfigModelUpdateDraftRejectsInvalidValidatedValueWithoutMutatingDraft(t *testing.T) {
	m := newConfigModel(config.DefaultConfig())
	original := m.draft.Concurrency.PageDownloads
	m.cursor = int(configFieldPageDownloads)
	m.input.SetValue("0")

	err := m.updateDraftFromInput()
	if err == nil {
		t.Fatal("updateDraftFromInput() error = nil, want validation error")
	}
	if m.draft.Concurrency.PageDownloads != original {
		t.Fatalf("PageDownloads = %d, want original %d", m.draft.Concurrency.PageDownloads, original)
	}
}
