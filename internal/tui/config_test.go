package tui

import (
	"testing"
	"time"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/tuiapp"
)

func TestConfigModelUpdateDraftFromCurrentFieldParsesValues(t *testing.T) {
	m := newConfigModel(configStateFromConfig(config.DefaultConfig()))

	cases := []struct {
		field configField
		value string
		want  func(tuiapp.ConfigState) bool
	}{
		{configFieldLanguage, "ru", func(c tuiapp.ConfigState) bool { return c.Language == "ru" }},
		{configFieldHTTPTimeout, "45s", func(c tuiapp.ConfigState) bool { return c.HTTPTimeout == 45*time.Second }},
		{configFieldPageDownloads, "3", func(c tuiapp.ConfigState) bool { return c.PageDownloads == 3 }},
		{configFieldSearchHistoryMax, "25", func(c tuiapp.ConfigState) bool { return c.SearchHistoryMax == 25 }},
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
	m := newConfigModel(configStateFromConfig(config.DefaultConfig()))
	original := m.draft.PageDownloads
	m.cursor = int(configFieldPageDownloads)
	m.input.SetValue("not-an-int")

	err := m.updateDraftFromInput()
	if err == nil {
		t.Fatal("updateDraftFromInput() error = nil, want error")
	}
	if m.draft.PageDownloads != original {
		t.Fatalf("PageDownloads = %d, want original %d", m.draft.PageDownloads, original)
	}
}

func TestConfigModelUpdateDraftRejectsInvalidValidatedValueWithoutMutatingDraft(t *testing.T) {
	m := newConfigModel(configStateFromConfig(config.DefaultConfig()))
	original := m.draft.PageDownloads
	m.cursor = int(configFieldPageDownloads)
	m.input.SetValue("0")

	err := m.updateDraftFromInput()
	if err == nil {
		t.Fatal("updateDraftFromInput() error = nil, want validation error")
	}
	if m.draft.PageDownloads != original {
		t.Fatalf("PageDownloads = %d, want original %d", m.draft.PageDownloads, original)
	}
}
