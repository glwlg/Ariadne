package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ariadne/internal/securestore"
)

func TestDefaultSettingsEnableAutonomousFlowWithoutStartingCapture(t *testing.T) {
	service := NewServiceWithPaths("", "")
	settings := service.GetSettings()

	if settings.General.Theme != "light" {
		t.Fatalf("expected light theme, got %q", settings.General.Theme)
	}
	if settings.WorkMemory.TimeMachineEnabled {
		t.Fatal("time machine should not start enabled by default")
	}
	if !settings.WorkMemory.WindowSwitchCaptureEnabled || settings.WorkMemory.WindowSwitchCooldownSecs != 3 || settings.WorkMemory.AutoCaptureIntervalSeconds != 30 {
		t.Fatalf("window capture strategy should default to delayed 3s and 30s active interval: %#v", settings.WorkMemory)
	}
	if settings.WorkMemory.CaptureScope != "active_window" {
		t.Fatalf("time machine should default to active-window capture to avoid background noise, got %q", settings.WorkMemory.CaptureScope)
	}
	if !settings.WorkMemory.DraftScheduleEnabled {
		t.Fatal("draft schedule should run by default so Flow can autonomously settle low-risk artifacts")
	}
	if settings.WorkMemory.DraftScheduleIntervalMin != 240 || !settings.WorkMemory.DailyDraftScheduleEnabled || !settings.WorkMemory.RetroDraftScheduleEnabled || !settings.WorkMemory.ExperienceScheduleEnabled {
		t.Fatalf("unexpected draft schedule defaults: %#v", settings.WorkMemory)
	}
	if !settings.WorkMemory.SensitiveRulesEnabled {
		t.Fatal("sensitive rules should be enabled by default")
	}
	if settings.WorkMemory.AllowSensitiveExport {
		t.Fatal("sensitive export should be disabled by default")
	}
	if !contains(settings.WorkMemory.ExcludeApps, "mstsc.exe") {
		t.Fatalf("default excluded apps should include remote desktop: %#v", settings.WorkMemory.ExcludeApps)
	}
	if settings.AI.Enabled || settings.AI.EmbeddingEnabled {
		t.Fatal("AI and embedding should be opt-in")
	}
	if settings.AI.OCRModelEnabled || settings.AI.OCRModel != "" || settings.AI.OCRProvider != "openai-compatible" {
		t.Fatalf("large-model OCR should be opt-in and keep a safe provider default: %#v", settings.AI)
	}
	if !settings.AI.ExternalAgentEnabled {
		t.Fatal("external agent task package generation should remain available")
	}
}

func TestUpdateSettingsNormalizesAndPersists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	service := NewServiceWithPaths(configPath, "")

	next := service.GetSettings()
	next.General.Theme = "Light"
	next.Screenshot.Quality = 150
	next.WorkMemory.AutoCaptureIntervalSeconds = 1
	next.WorkMemory.WindowSwitchCooldownSecs = 1
	next.WorkMemory.DraftScheduleIntervalMin = 1
	next.WorkMemory.ExcludeApps = []string{" bitwarden.exe ", "BITWARDEN.exe", ""}
	next.WorkMemory.ExcludeWindowKeywords = nil
	next.WorkMemory.ExcludeURLs = []string{" https://internal.example.com/private ", "HTTPS://INTERNAL.EXAMPLE.COM/PRIVATE", ""}
	next.WorkMemory.AppCaptureProfiles = []WorkMemoryAppCaptureProfile{
		{DisplayName: "微信", ProcessName: " Weixin.exe ", Enabled: true, WindowSwitchDelaySeconds: -1, ActiveIntervalSeconds: 1},
		{DisplayName: "重复微信", ProcessName: "weixin.EXE", Enabled: true, WindowSwitchDelaySeconds: 30, ActiveIntervalSeconds: 300},
		{DisplayName: "", ProcessName: "", Enabled: true},
	}
	next.AI.TraceMode = "external"
	next.AI.OCRProvider = ""
	next.AI.OCRBaseURL = " http://vision.internal/v1 "
	next.AI.OCRModel = " vision-model "

	updated := service.UpdateSettings(next)
	if updated.General.Theme != "light" {
		t.Fatalf("theme should normalize to light, got %q", updated.General.Theme)
	}
	if updated.Screenshot.Quality != 100 {
		t.Fatalf("quality should clamp to 100, got %d", updated.Screenshot.Quality)
	}
	if updated.WorkMemory.AutoCaptureIntervalSeconds != 10 {
		t.Fatalf("interval should clamp to 10, got %d", updated.WorkMemory.AutoCaptureIntervalSeconds)
	}
	if updated.WorkMemory.WindowSwitchCooldownSecs != 3 {
		t.Fatalf("window switch cooldown should clamp to 3, got %d", updated.WorkMemory.WindowSwitchCooldownSecs)
	}
	if updated.WorkMemory.DraftScheduleIntervalMin != 15 {
		t.Fatalf("draft schedule interval should clamp to 15, got %d", updated.WorkMemory.DraftScheduleIntervalMin)
	}
	if len(updated.WorkMemory.ExcludeApps) != 1 || updated.WorkMemory.ExcludeApps[0] != "bitwarden.exe" {
		t.Fatalf("exclude apps should be trimmed and deduplicated: %#v", updated.WorkMemory.ExcludeApps)
	}
	if len(updated.WorkMemory.ExcludeWindowKeywords) == 0 {
		t.Fatal("empty exclusion keywords should fall back to conservative defaults")
	}
	if len(updated.WorkMemory.ExcludeURLs) != 1 || updated.WorkMemory.ExcludeURLs[0] != "https://internal.example.com/private" {
		t.Fatalf("exclude URLs should be trimmed and deduplicated: %#v", updated.WorkMemory.ExcludeURLs)
	}
	if len(updated.WorkMemory.AppCaptureProfiles) != 1 {
		t.Fatalf("app capture profiles should be deduplicated: %#v", updated.WorkMemory.AppCaptureProfiles)
	}
	profile := updated.WorkMemory.AppCaptureProfiles[0]
	if profile.ID != "weixin.exe" || profile.ProcessName != "Weixin.exe" || profile.DisplayName != "微信" || !profile.Enabled {
		t.Fatalf("unexpected app capture profile identity: %#v", profile)
	}
	if profile.WindowSwitchDelaySeconds != 0 || profile.ActiveIntervalSeconds != 10 {
		t.Fatalf("app capture profile timings should use safe defaults: %#v", profile)
	}
	if updated.AI.TraceMode != "off" {
		t.Fatalf("unknown trace mode should normalize to off, got %q", updated.AI.TraceMode)
	}
	if updated.AI.OCRProvider != "openai-compatible" || updated.AI.OCRBaseURL != "http://vision.internal/v1" || updated.AI.OCRModel != "vision-model" {
		t.Fatalf("OCR model settings should normalize independently: %#v", updated.AI)
	}

	reloaded := NewServiceWithPaths(configPath, "").GetSettings()
	if reloaded.General.Theme != "light" {
		t.Fatalf("persisted settings not reloaded: %#v", reloaded.General)
	}

	storage := service.StorageStatus()
	if !storage.Exists || !storage.ReadBackOK || storage.Path != configPath || storage.LastSaveError != "" {
		t.Fatalf("unexpected storage status: %#v", storage)
	}
	if storage.ReadBackBytes <= 0 || storage.ReadBackVersion != currentSettingsVersion {
		t.Fatalf("storage readback should expose persisted bytes and version: %#v", storage)
	}
}

func TestLegacyThemeMigratesToLightWithoutRemovingDarkMode(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	legacy := defaultSettings()
	legacy.Version = 10
	legacy.General.Theme = "dark"
	legacy.WorkMemory.CaptureScope = "all_screens"
	legacy.WorkMemory.DraftScheduleEnabled = false
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	migrated := NewServiceWithPaths(configPath, "").GetSettings()
	if migrated.Version != currentSettingsVersion {
		t.Fatalf("legacy settings should upgrade to current version, got %d", migrated.Version)
	}
	if migrated.General.Theme != "light" {
		t.Fatalf("legacy experimental dark should reset to light, got %q", migrated.General.Theme)
	}
	if migrated.WorkMemory.CaptureScope != "active_window" {
		t.Fatalf("legacy capture scope should migrate to active window, got %q", migrated.WorkMemory.CaptureScope)
	}
	if !migrated.WorkMemory.DraftScheduleEnabled || migrated.WorkMemory.DraftScheduleIntervalMin != 240 || !migrated.WorkMemory.DailyDraftScheduleEnabled || !migrated.WorkMemory.RetroDraftScheduleEnabled || !migrated.WorkMemory.ExperienceScheduleEnabled {
		t.Fatalf("legacy settings should get safe draft schedule defaults: %#v", migrated.WorkMemory)
	}

	persisted := NewServiceWithPaths(configPath, "").GetSettings()
	if persisted.Version != currentSettingsVersion || persisted.General.Theme != "light" {
		t.Fatalf("migrated settings should be written back, got %#v", persisted.General)
	}

	systemPath := filepath.Join(dir, "system.json")
	legacySystem := defaultSettings()
	legacySystem.Version = 10
	legacySystem.General.Theme = "system"
	systemRaw, err := json.Marshal(legacySystem)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(systemPath, systemRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	loadedSystem := NewServiceWithPaths(systemPath, "").GetSettings()
	if loadedSystem.General.Theme != "light" {
		t.Fatalf("legacy system theme should reset to light, got %q", loadedSystem.General.Theme)
	}

	currentPath := filepath.Join(dir, "current.json")
	currentDark := defaultSettings()
	currentDark.Version = currentSettingsVersion
	currentDark.General.Theme = "dark"
	currentRaw, err := json.Marshal(currentDark)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, currentRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	loadedCurrent := NewServiceWithPaths(currentPath, "").GetSettings()
	if loadedCurrent.General.Theme != "dark" {
		t.Fatalf("current dark mode should load as explicit dark, got %q", loadedCurrent.General.Theme)
	}

	current := migrated
	current.General.Theme = "dark"
	updated := NewServiceWithPaths("", "").UpdateSettings(current)
	if updated.General.Theme != "dark" {
		t.Fatalf("current dark mode should remain available, got %q", updated.General.Theme)
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

func TestImportLegacyConfigMapsSafeUserPreferences(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ariadne.json")
	legacyPath := filepath.Join(dir, "x-tools.json")
	legacy := map[string]any{
		"theme":                        "Light",
		"run_on_startup":               true,
		"screenshot_auto_copy":         true,
		"screenshot_filename_template": "x-tools_{date}_{time}",
		"hotkeys": map[string]any{
			"toggle_window": "ctrl+space",
			"screenshot":    "alt+s",
			"pin_clipboard": "alt+p",
		},
		"plugins_enabled": map[string]any{
			"JSON": false,
			"UUID": true,
		},
		"work_memory": map[string]any{
			"time_machine_enabled":             true,
			"auto_capture_interval_seconds":    120.0,
			"window_switch_capture_enabled":    true,
			"window_switch_cooldown_seconds":   45.0,
			"exclude_apps":                     []any{"safe.exe", "SAFE.exe"},
			"exclude_window_keywords":          []any{"secret", "内网"},
			"exclude_urls":                     []any{"https://private.example.com", "HTTPS://PRIVATE.EXAMPLE.COM"},
			"ai_enabled":                       true,
			"embedding_enabled":                true,
			"vector_store_type":                "milvus",
			"vector_collection":                "x_tools_work_memory",
			"codex_collaboration_enabled":      true,
			"allow_sensitive_export":           true,
			"experience_discovery_period_days": 14.0,
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewServiceWithPaths(configPath, legacyPath)
	status := service.LegacyConfigStatus()
	if !status.Exists {
		t.Fatal("legacy status should detect config file")
	}
	if !status.NeedsImport {
		t.Fatalf("legacy status should require import before settings match: %#v", status)
	}
	if !contains(status.ImportedKeys, "work_memory") {
		t.Fatalf("legacy status should list importable keys: %#v", status.ImportedKeys)
	}

	imported := service.ImportLegacyConfig()
	if imported.General.Theme != "light" || !imported.General.RunOnStartup {
		t.Fatalf("general settings were not imported: %#v", imported.General)
	}
	if imported.Hotkeys.ToggleWindow != "ctrl+space" {
		t.Fatalf("hotkeys were not imported: %#v", imported.Hotkeys)
	}
	if !imported.Screenshot.AutoCopy || imported.Screenshot.FilenameTemplate != "x-tools_{date}_{time}" {
		t.Fatalf("screenshot settings were not imported: %#v", imported.Screenshot)
	}
	if imported.WorkMemory.AutoCaptureIntervalSeconds != 120 || !imported.WorkMemory.TimeMachineEnabled {
		t.Fatalf("work memory settings were not imported: %#v", imported.WorkMemory)
	}
	if !imported.WorkMemory.WindowSwitchCaptureEnabled || imported.WorkMemory.WindowSwitchCooldownSecs != 45 {
		t.Fatalf("window switch capture settings were not imported: %#v", imported.WorkMemory)
	}
	if len(imported.WorkMemory.ExcludeApps) != 1 || imported.WorkMemory.ExcludeApps[0] != "safe.exe" {
		t.Fatalf("legacy exclusion apps should be deduplicated: %#v", imported.WorkMemory.ExcludeApps)
	}
	if len(imported.WorkMemory.ExcludeURLs) != 1 || imported.WorkMemory.ExcludeURLs[0] != "https://private.example.com" {
		t.Fatalf("legacy exclusion URLs should be imported and deduplicated: %#v", imported.WorkMemory.ExcludeURLs)
	}
	if !imported.AI.Enabled || !imported.AI.EmbeddingEnabled || imported.AI.VectorStoreType != "milvus" {
		t.Fatalf("AI settings were not imported: %#v", imported.AI)
	}
	if !imported.AI.CodexCollaborationEnabled {
		t.Fatal("external agent collaboration setting should import")
	}
	if !imported.Plugins.Enabled["uuid"] || imported.Plugins.Enabled["json"] {
		t.Fatalf("plugin flags were not imported: %#v", imported.Plugins.Enabled)
	}
	quietStatus := service.LegacyConfigStatus()
	if quietStatus.NeedsImport {
		t.Fatalf("legacy status should be quiet after imported preferences match: %#v", quietStatus)
	}
}

func TestImportLegacyConfigMigratesSecretsToCredentialStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ariadne.json")
	legacyPath := filepath.Join(dir, "x-tools.json")
	legacy := map[string]any{
		"work_memory": map[string]any{
			"ai_api_key":        "legacy-ai-key",
			"embedding_api_key": "legacy-embedding-key",
			"milvus_token":      "legacy-milvus-token",
			"ai_base_url":       "http://legacy-ai/v1",
			"embedding_model":   "legacy-embedding-model",
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &fakeCredentialStore{available: true, values: map[string]string{}}
	service := NewServiceWithPathsAndCredentialStore(configPath, legacyPath, store)

	status := service.LegacyConfigStatus()
	if !contains(status.ImportedKeys, "credential:ai_api_key") || !contains(status.ImportedKeys, "credential:embedding_api_key") || !contains(status.ImportedKeys, "credential:milvus_token") {
		t.Fatalf("legacy status should expose migratable credential keys: %#v", status)
	}
	if !noteContains(status.Notes, "导入旧配置时会迁入安全凭据存储") {
		t.Fatalf("legacy status should explain secure migration, got %#v", status.Notes)
	}

	imported := service.ImportLegacyConfig()
	if imported.AI.BaseURL != "http://legacy-ai/v1" || imported.AI.EmbeddingModel != "legacy-embedding-model" {
		t.Fatalf("non-secret AI settings should still import: %#v", imported.AI)
	}
	if store.values[securestore.TargetOpenAIAPIKey] != "legacy-ai-key" {
		t.Fatalf("AI key was not migrated to secure store: %#v", store.values)
	}
	if store.values[securestore.TargetEmbeddingAPIKey] != "legacy-embedding-key" {
		t.Fatalf("embedding key was not migrated to secure store: %#v", store.values)
	}
	if store.values[securestore.TargetMilvusToken] != "legacy-milvus-token" {
		t.Fatalf("Milvus token was not migrated to secure store: %#v", store.values)
	}
	nextStatus := service.LegacyConfigStatus()
	if nextStatus.NeedsImport {
		t.Fatalf("legacy secret status should be quiet after secure migration: %#v", nextStatus)
	}
	for _, note := range []string{
		"检测到旧版 AI API key；Ariadne 安全存储中已有对应记录",
		"检测到旧版 Embedding API key；Ariadne 安全存储中已有对应记录",
		"检测到旧版 Milvus token；Ariadne 安全存储中已有对应记录",
	} {
		if !noteContains(nextStatus.Notes, note) {
			t.Fatalf("legacy status should retain migration note %q, got %#v", note, nextStatus.Notes)
		}
	}

	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	persistedText := string(persisted)
	for _, secret := range []string{"legacy-ai-key", "legacy-embedding-key", "legacy-milvus-token"} {
		if strings.Contains(persistedText, secret) {
			t.Fatalf("legacy secret %q leaked into Ariadne JSON: %s", secret, persistedText)
		}
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
	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "legacy-ai-key") {
		t.Fatalf("legacy secret leaked into Ariadne JSON: %s", string(persisted))
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

func noteContains(items []string, text string) bool {
	for _, item := range items {
		if strings.Contains(item, text) {
			return true
		}
	}
	return false
}
