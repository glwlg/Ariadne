package apps

import (
	"os"
	"path/filepath"
	"testing"

	"ariadne/internal/contracts"
)

func TestSearchReturnsStartMenuShortcutResults(t *testing.T) {
	root := t.TempDir()
	shortcutPath := filepath.Join(root, "Tools", "Ariadne Notes.lnk")
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shortcutPath, []byte("shortcut"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithRoots(root)

	results := service.Search("ariadne")

	if len(results) != 1 {
		t.Fatalf("expected one app result, got %#v", results)
	}
	result := results[0]
	if result.Type != contracts.ResultApp {
		t.Fatalf("expected app result, got %s", result.Type)
	}
	if result.Title != "Ariadne Notes" {
		t.Fatalf("unexpected app title: %q", result.Title)
	}
	if result.Score <= 0 {
		t.Fatalf("expected positive score: %#v", result)
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
	if hasActionKind(result, contracts.ActionOpenParent) {
		t.Fatal("app result must not expose open_parent")
	}
	if result.Actions[0].Label != "打开应用" {
		t.Fatalf("expected app launch action, got %#v", result.Actions[0])
	}
}

func TestSearchDoesNotReturnAppsForEmptyQuery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Calculator.lnk"), []byte("shortcut"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithRoots(root)

	results := service.Search("")

	if len(results) != 0 {
		t.Fatalf("empty query should not return app shortcuts: %#v", results)
	}
}

func TestSearchSortsExactAppNameBeforePathMatches(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Calculator.lnk", "Utility Calc Helper.lnk"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("shortcut"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service := NewServiceWithRoots(root)

	results := service.Search("calculator")

	if len(results) == 0 || results[0].Title != "Calculator" {
		t.Fatalf("expected exact app name first: %#v", results)
	}
}

func hasActionKind(result contracts.SearchResult, kind contracts.PreviewActionKind) bool {
	for _, action := range result.Actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}
