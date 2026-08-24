package main

import (
	"path/filepath"
	"testing"

	"ariadne/internal/settings"
)

func TestFileSearchPolicyProviderReadsSettingsConfigScope(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	service := settings.NewServiceWithPaths(configPath, "")
	next := service.GetSettings()
	next.Search.FileIncludeExtensions = []string{".md"}
	next.Search.FileExcludeExtensions = []string{".py"}
	service.UpdateSettings(next)

	policy := fileSearchPolicyProvider([]string{"--settings-config", configPath})()
	if len(policy.IncludeExtensions) != 1 || policy.IncludeExtensions[0] != ".md" {
		t.Fatalf("include extensions not loaded from config scope: %#v", policy.IncludeExtensions)
	}
	if len(policy.ExcludeExtensions) != 1 || policy.ExcludeExtensions[0] != ".py" {
		t.Fatalf("exclude extensions not loaded from config scope: %#v", policy.ExcludeExtensions)
	}
}

func TestFileSearchSettingsConfigPathMapsSQLiteStorageToConfigScope(t *testing.T) {
	sqlitePath := filepath.Join(`C:\Users\luwei\AppData\Roaming\Ariadne`, "ariadne.sqlite")
	got := fileSearchSettingsConfigPath([]string{"--settings-config", sqlitePath})
	want := filepath.Join(filepath.Dir(sqlitePath), "config.json")
	if got != want {
		t.Fatalf("settings config path = %q, want %q", got, want)
	}
}
