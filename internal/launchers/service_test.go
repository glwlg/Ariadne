package launchers

import (
	"os"
	"path/filepath"
	"testing"

	"ariadne/internal/contracts"
)

func TestSearchReturnsCustomLauncherResult(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "launchers.json"))
	service.Upsert(Launcher{
		ID:       "docs",
		Name:     "项目文档",
		Kind:     LauncherFolder,
		Target:   `P:\workspace\glwlg\app\x-tools\docs`,
		Keywords: []string{"docs", "plan"},
		Tags:     []string{"文档"},
		Enabled:  true,
	})

	results := service.Search("docs")

	if len(results) != 1 {
		t.Fatalf("expected launcher result, got %#v", results)
	}
	result := results[0]
	if result.Type != contracts.ResultCommand {
		t.Fatalf("expected command result, got %s", result.Type)
	}
	if result.Actions[0].Label != "打开" {
		t.Fatalf("expected open action, got %#v", result.Actions[0])
	}
	if err := contracts.ValidateActionSurface(result); err != nil {
		t.Fatalf("invalid action surface: %v", err)
	}
}

func TestCommandLaunchersRequireConfirmation(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "launchers.json"))
	service.Upsert(Launcher{
		ID:        "flush-dns",
		Name:      "刷新 DNS",
		Kind:      LauncherCommand,
		Target:    "ipconfig",
		Arguments: "/flushdns",
		Keywords:  []string{"dns"},
		Enabled:   true,
	})

	results := service.Search("dns")

	if len(results) != 1 {
		t.Fatalf("expected command launcher result, got %#v", results)
	}
	action := results[0].Actions[0]
	if action.Kind != contracts.ActionDanger {
		t.Fatalf("command launcher should be marked dangerous, got %#v", action)
	}
	if action.Payload["requiresConfirmation"] != true {
		t.Fatalf("command launcher should require confirmation: %#v", action.Payload)
	}
	if action.Feedback == nil || action.Feedback.SuccessLabel != "已启动" {
		t.Fatalf("command launcher should report confirmed launch feedback: %#v", action.Feedback)
	}
}

func TestLaunchersPersistCustomItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launchers.json")
	service := NewServiceWithPath(path)
	status := service.Upsert(Launcher{
		ID:       "readme",
		Name:     "README",
		Kind:     LauncherFile,
		Target:   `P:\workspace\glwlg\app\x-tools\README.md`,
		Keywords: []string{"readme"},
		Enabled:  true,
	})
	if status.LastSaveError != "" {
		t.Fatalf("unexpected save error: %s", status.LastSaveError)
	}

	reloaded := NewServiceWithPath(path)
	results := reloaded.Search("readme")

	if len(results) != 1 || results[0].Title != "README" {
		t.Fatalf("expected persisted launcher, got %#v", results)
	}
}

func TestSaveErrorIsReportedInStatus(t *testing.T) {
	path := t.TempDir()
	service := NewServiceWithPath(path)

	status := service.Upsert(Launcher{
		ID:      "invalid-save-target",
		Name:    "Invalid Save Target",
		Kind:    LauncherFile,
		Target:  `P:\workspace\glwlg\app\x-tools\README.md`,
		Enabled: true,
	})

	if status.LastSaveError == "" {
		t.Fatalf("expected save error for directory path")
	}
}

func TestDisabledLaunchersDoNotSearch(t *testing.T) {
	service := NewServiceWithPath(filepath.Join(t.TempDir(), "launchers.json"))
	service.Upsert(Launcher{
		ID:      "disabled-docs",
		Name:    "Disabled Docs",
		Kind:    LauncherFolder,
		Target:  `P:\workspace\glwlg\app\x-tools\docs`,
		Enabled: false,
	})

	results := service.Search("disabled")

	if len(results) != 0 {
		t.Fatalf("disabled launchers should not be searchable: %#v", results)
	}
}

func TestRemovePersistsDefaultLauncherTombstone(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "AppData", "Roaming")
	if err := os.MkdirAll(appData, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", appData)
	path := filepath.Join(t.TempDir(), "launchers.json")
	service := NewServiceWithPath(path)

	service.Remove("ariadne-config-dir")
	reloaded := NewServiceWithPath(path)
	results := reloaded.Search("ariadne config")

	for _, result := range results {
		if result.ID == "launcher-ariadne-config-dir" {
			t.Fatalf("removed default launcher should stay removed after reload: %#v", results)
		}
	}
}
