package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyMigrationExcludesRemovedMemorySource(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "work_memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "work_memory", "entries.json"), []byte(`[{"id":"retired"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithRoot(root, nil, nil)
	status := service.Status()
	if len(status.Sources) != 2 {
		t.Fatalf("expected only clipboard and capture sources: %#v", status)
	}
	for _, source := range status.Sources {
		if source.Source == "work_memory" {
			t.Fatal("memory import must not be advertised")
		}
	}
	result := service.ImportLegacyData(LegacyImportRequest{Sources: []string{"work_memory"}})
	if result.OK {
		t.Fatalf("removed source must not import data: %#v", result)
	}
}
