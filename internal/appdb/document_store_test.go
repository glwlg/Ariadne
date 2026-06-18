package appdb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"
)

func TestLegacyLoadJSONReadsLegacyJSONFile(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "work_memory.json")
	if err := os.WriteFile(legacyPath, []byte(`{"version":1,"entries":[{"id":"old"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var state struct {
		Version int `json:"version"`
		Entries []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}

	ok, err := LegacyLoadJSON(legacyPath, "work_memory", &state)
	if err != nil || !ok {
		t.Fatalf("expected legacy JSON state, ok=%v err=%v", ok, err)
	}
	if state.Version != 1 || len(state.Entries) != 1 || state.Entries[0].ID != "old" {
		t.Fatalf("unexpected legacy state: %#v", state)
	}
	if _, err := os.Stat(DatabasePathForPath(legacyPath)); !os.IsNotExist(err) {
		t.Fatalf("reading a legacy file should not create sqlite by itself, err=%v", err)
	}
}

func TestLegacyLoadJSONReadsAndDropsStateDocumentsBlob(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "work_memory.json")
	conn, err := OpenForPath(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := ensureSchema(conn); err != nil {
		t.Fatal(err)
	}
	if err := sqlitex.Execute(conn, `INSERT INTO state_documents(key, legacy_path, payload, updated_at) VALUES (?1, ?2, ?3, ?4)`, &sqlitex.ExecOptions{
		Args: []any{"work_memory", legacyPath, []byte(`{"version":2,"entries":[{"id":"blob"}]}`), time.Now().UnixMilli()},
	}); err != nil {
		t.Fatal(err)
	}

	var state struct {
		Version int `json:"version"`
		Entries []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	ok, err := LegacyLoadJSON(legacyPath, "work_memory", &state)
	if err != nil || !ok {
		t.Fatalf("expected state_documents fallback, ok=%v err=%v", ok, err)
	}
	if state.Version != 2 || len(state.Entries) != 1 || state.Entries[0].ID != "blob" {
		t.Fatalf("unexpected blob state: %#v", state)
	}
	if err := DropLegacyDocument(legacyPath, "work_memory"); err != nil {
		t.Fatal(err)
	}
	raw, ok, err := readStateDocumentPayload(conn, "work_memory")
	if err != nil || ok || len(raw) != 0 {
		t.Fatalf("expected dropped blob, ok=%v len=%d err=%v", ok, len(raw), err)
	}
}
