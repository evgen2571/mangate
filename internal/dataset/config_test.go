package dataset

import (
	"path/filepath"
	"testing"

	"github.com/evgen2571/mangate/internal/archive"
)

func TestConfigValidationAndStableHash(t *testing.T) {
	cfg := DefaultConfig(filepath.Join(t.TempDir(), "set"), "fake")
	cfg.Limits.MaxPages = 10
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	first, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	second, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("unstable hash: %s != %s", first, second)
	}
	cfg.Output.Format = archive.Format("rar")
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected invalid format")
	}
}

func TestConfigRequiresStoppingCondition(t *testing.T) {
	cfg := DefaultConfig(t.TempDir(), "fake")
	cfg.Sampling.MaxTitles, cfg.Limits.MaxPages, cfg.Limits.MaxBytes = 0, 0, 0
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected stopping condition error")
	}
}
