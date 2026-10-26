package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/appdb"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestLegacySettingsGetDefaultSearchExcludes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	raw := []byte(`{"version":16,"general":{"theme":"light"},"hotkeys":{"toggleWindow":"alt+q","screenshot":"alt+a","pinClipboard":"alt+v"},"screenshot":{},"workMemory":{},"ai":{},"plugins":{"enabled":{}}}`)
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded := NewServiceWithPaths(configPath, "").GetSettings()

	if loaded.Version != currentSettingsVersion {
		t.Fatalf("legacy settings should upgrade, got %d", loaded.Version)
	}
	if !containsPathSuffix(loaded.Search.FileExcludeFolders, "Microsoft/Windows/Recent") || !contains(loaded.Search.FileExcludePatterns, `(?i)\.(tmp|temp|part|crdownload|download)$`) {
		t.Fatalf("legacy settings should receive default search excludes: %#v", loaded.Search)
	}
}

func TestVersion19SettingsMergeExpandedSearchExcludes(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	legacy := defaultSettings()
	legacy.Version = 19
	legacy.Search.FileExcludeFolders = []string{`C:\CustomScratch`}
	legacy.Search.FileExcludePatterns = []string{`(?i)\.scratch$`}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	migrated := NewServiceWithPaths(configPath, "").GetSettings()

	if migrated.Version != currentSettingsVersion {
		t.Fatalf("settings should upgrade to current version, got %d", migrated.Version)
	}
	if !contains(migrated.Search.FileExcludeFolders, `C:\CustomScratch`) || !containsPathSuffix(migrated.Search.FileExcludeFolders, "Microsoft/Windows/Recent") || !containsPathSuffix(migrated.Search.FileExcludeFolders, "AppData/Local") {
		t.Fatalf("search exclude folder migration should keep custom paths and add defaults: %#v", migrated.Search.FileExcludeFolders)
	}
	if !contains(migrated.Search.FileExcludePatterns, `(?i)\.scratch$`) || !contains(migrated.Search.FileExcludePatterns, `(?i)^[a-z]:[\\/]users[\\/][^\\/]+[\\/]\.(cache|config|local|m2|gradle|nuget|npm|pnpm|yarn|cargo|rustup|wox|gemini|confirmo|switchhosts|antigravity|antigravity_cockpit|marscode)([\\/]|$)`) || !contains(migrated.Search.FileExcludePatterns, `(?i)\.(log|journal|db-journal|sqlite-wal|sqlite-shm|db-wal|db-shm|odlgz|statistic|dxcache-shm|dxcache-wal)$`) {
		t.Fatalf("search exclude pattern migration should keep custom patterns and add defaults: %#v", migrated.Search.FileExcludePatterns)
	}
	if !contains(migrated.Search.FileExcludeExtensions, `.tmp`) || !contains(migrated.Search.FileExcludeExtensions, `.sqlite-wal`) {
		t.Fatalf("search exclude extension migration should add defaults: %#v", migrated.Search.FileExcludeExtensions)
	}
}

func TestAdditionalLightThemesPersist(t *testing.T) {
	service := NewServiceWithPaths("", "")
	settings := service.GetSettings()
	for _, theme := range []string{"professional-pink", "light-graphite", "cloud-blue"} {
		settings.General.Theme = theme
		updated := service.UpdateSettings(settings)
		if updated.General.Theme != theme {
			t.Fatalf("theme %q should persist, got %q", theme, updated.General.Theme)
		}
		settings = updated
	}

	settings.General.Theme = "unknown-theme"
	updated := service.UpdateSettings(settings)
	if updated.General.Theme != "cloud-blue" {
		t.Fatalf("unknown theme should normalize to cloud-blue, got %q", updated.General.Theme)
	}
}

func TestStorageStatusReportsInvalidPersistedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithPaths(configPath, "")
	storage := service.StorageStatus()

	if !storage.Exists {
		t.Fatal("invalid config file should still be detected on disk")
	}
	if storage.ReadBackOK {
		t.Fatalf("invalid config should not pass readback: %#v", storage)
	}
	if storage.ReadBackError == "" {
		t.Fatalf("invalid config should report a readback error: %#v", storage)
	}
}

func TestFindVirtualizedConfigPathDetectsMSIXLocalCache(t *testing.T) {
	dir := t.TempDir()
	appData := filepath.Join(dir, "Roaming")
	localAppData := filepath.Join(dir, "Local")
	configPath := filepath.Join(appData, "Ariadne", "config.json")
	virtualizedPath := filepath.Join(localAppData, "Packages", "OpenAI.Codex_2p2nqsd0c76g0", "LocalCache", "Roaming", "Ariadne", "config.json")
	if err := os.MkdirAll(filepath.Dir(virtualizedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(virtualizedPath, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	path, exists, size := findVirtualizedConfigPath(configPath, appData, localAppData)

	if !exists {
		t.Fatal("expected virtualized config to be detected")
	}
	if path != virtualizedPath {
		t.Fatalf("unexpected virtualized path: %q", path)
	}
	if size <= 0 {
		t.Fatalf("expected virtualized file size, got %d", size)
	}
}

func TestImportLegacyConfigSkipsSecretsWhenCredentialStoreUnavailable(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ariadne.json")
	legacyPath := filepath.Join(dir, "x-tools.json")
	legacy := map[string]any{
		"work_memory": map[string]any{
			"ai_api_key": "legacy-ai-key",
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &fakeCredentialStore{available: false, values: map[string]string{}}
	service := NewServiceWithPathsAndCredentialStore(configPath, legacyPath, store)

	service.ImportLegacyConfig()
	if len(store.values) != 0 {
		t.Fatalf("unavailable secure store should not receive secrets: %#v", store.values)
	}
	persistedText := readPersistedSettingsText(t, configPath)
	if strings.Contains(persistedText, "legacy-ai-key") {
		t.Fatalf("legacy secret leaked into Ariadne settings storage: %s", persistedText)
	}
	status := service.LegacyConfigStatus()
	if !noteContains(status.Notes, "安全凭据存储不可用") {
		t.Fatalf("status should explain skipped secret migration, got %#v", status.Notes)
	}
}

type fakeCredentialStore struct {
	available bool
	values    map[string]string
}

func (f *fakeCredentialStore) Available() bool { return f.available }
func (f *fakeCredentialStore) Backend() string { return "test" }
func (f *fakeCredentialStore) Read(target string) (string, bool, error) {
	value, ok := f.values[target]
	return value, ok, nil
}
func (f *fakeCredentialStore) Write(target string, secret string) error {
	f.values[target] = secret
	return nil
}
func (f *fakeCredentialStore) Delete(target string) error {
	delete(f.values, target)
	return nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsPathSuffix(items []string, suffix string) bool {
	for _, item := range items {
		if strings.HasSuffix(filepath.ToSlash(item), suffix) {
			return true
		}
	}
	return false
}

func noteContains(items []string, text string) bool {
	for _, item := range items {
		if strings.Contains(item, text) {
			return true
		}
	}
	return false
}

func readPersistedSettingsText(t *testing.T, path string) string {
	t.Helper()
	conn, err := appdb.OpenForPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := ensureSettingsSchema(conn); err != nil {
		t.Fatal(err)
	}
	scope := settingsScope(path)
	var text strings.Builder
	queries := []string{
		`SELECT text_value FROM settings2_values WHERE scope = ?1`,
		`SELECT value FROM settings2_string_lists WHERE scope = ?1`,
		`SELECT plugin_id FROM settings2_plugins WHERE scope = ?1`,
	}
	for _, query := range queries {
		if err := sqlitex.Execute(conn, query, &sqlitex.ExecOptions{
			Args: []any{scope},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				text.WriteString(stmt.ColumnText(0))
				text.WriteByte('\n')
				return nil
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if text.Len() == 0 {
		t.Fatalf("expected persisted settings rows in sqlite for %s", path)
	}
	return text.String()
}
