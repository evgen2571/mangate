package dataset

import (
	"context"
	"encoding/json"
	"os"
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
	cfg.Output.Format = archive.FormatCBZ
	if err := cfg.Normalize(); err == nil {
		t.Fatal("archive dataset format was accepted")
	}
}

func TestStorePersistsStateAsJSON(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig(root, "fake")
	cfg.Limits.MaxPages = 1
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Initialize(context.Background(), cfg, false); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplacePlan(context.Background(), []Title{{ID: "id", Name: "Title"}}, []Title{{ID: "id", Name: "Title", SampleRank: 0}}, []Chapter{{ID: "chapter", TitleID: "id", Number: "1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimChapter(context.Background(), "chapter", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "dataset-state.json")); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenExisting(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := Verify(context.Background(), reopened, true); err != nil {
		t.Fatal(err)
	}
	reopened.mu.RLock()
	chapter := reopened.data.Chapters["chapter"]
	attempts := len(reopened.data.Attempts)
	reopened.mu.RUnlock()
	if chapter.State != "partial" || chapter.ClaimOwner != "" || attempts != 1 {
		t.Fatalf("repaired chapter = %#v, attempts=%d", chapter, attempts)
	}
}

func TestOpenExistingDoesNotCreateMissingDataset(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	if _, err := OpenExisting(root); err == nil {
		t.Fatal("expected missing dataset error")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("OpenExisting created root: %v", err)
	}
}

func TestDatasetInfoUsesStableJSONFieldNames(t *testing.T) {
	data, err := json.Marshal(DatasetInfo{ConfigHash: "hash", StoppingReason: "limit", Counters: Counters{ValidPages: 2}})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value["configurationHash"] != "hash" || value["stoppingReason"] != "limit" {
		t.Fatalf("dataset info JSON = %#v", value)
	}
}
