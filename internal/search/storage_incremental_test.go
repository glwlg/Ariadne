package search

import (
	"path/filepath"
	"testing"

	"ariadne/internal/appdb"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestUsageWritesOnlyAffectedRowAndPreservesStateOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_state.json")
	store := NewStateStore(path)
	if _, err := store.RecordUse("unrelated"); err != nil {
		t.Fatal(err)
	}
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := sqlitex.ExecScript(conn, `
CREATE TRIGGER protect_usage BEFORE DELETE ON search_usage WHEN old.result_id='unrelated'
BEGIN SELECT RAISE(ABORT, 'unrelated record was rewritten'); END;
CREATE TRIGGER fail_usage BEFORE INSERT ON search_usage WHEN new.result_id='failed'
BEGIN SELECT RAISE(ABORT, 'save rejected'); END;
`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetFavorite("target", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetFavorite("target", false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordUse("failed"); err == nil {
		t.Fatal("expected storage failure")
	}
	if _, err := store.SetFavorite("failed", true); err == nil {
		t.Fatal("expected storage failure")
	}
	if record := store.Get("failed"); record.UseCount != 0 || record.Favorite {
		t.Fatalf("failed write changed memory: %#v", record)
	}
	reloaded := NewStateStore(path)
	if reloaded.Status().Count != 1 || reloaded.Get("unrelated").UseCount != 1 {
		t.Fatalf("unexpected persisted usage: %#v", reloaded.Status())
	}
}
