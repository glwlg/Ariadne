package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"ariadne/internal/appdb"
	"ariadne/internal/securestore"
)

type GeneralSettings struct {
	Theme        string `json:"theme"`
	RunOnStartup bool   `json:"runOnStartup"`
	Language     string `json:"language"`
}

type Hotkeys struct {
	ToggleWindow string `json:"toggleWindow"`
	Screenshot   string `json:"screenshot"`
	PinClipboard string `json:"pinClipboard"`
}

type ScreenshotSettings struct {
	AutoCopy         bool   `json:"autoCopy"`
	AutoPin          bool   `json:"autoPin"`
	AutoSave         bool   `json:"autoSave"`
	SaveDir          string `json:"saveDir"`
	FilenameTemplate string `json:"filenameTemplate"`
	Quality          int    `json:"quality"`
}

type AISettings struct {
	Enabled                    bool   `json:"enabled"`
	Provider                   string `json:"provider"`
	BaseURL                    string `json:"baseUrl"`
	Model                      string `json:"model"`
	OCRModelEnabled            bool   `json:"ocrModelEnabled"`
	OCRProvider                string `json:"ocrProvider"`
	OCRBaseURL                 string `json:"ocrBaseUrl"`
	OCRModel                   string `json:"ocrModel"`
	EmbeddingEnabled           bool   `json:"embeddingEnabled"`
	EmbeddingProvider          string `json:"embeddingProvider"`
	EmbeddingBaseURL           string `json:"embeddingBaseUrl"`
	EmbeddingModel             string `json:"embeddingModel"`
	VectorStoreType            string `json:"vectorStoreType"`
	VectorStoreURI             string `json:"vectorStoreUri"`
	VectorCollection           string `json:"vectorCollection"`
	AgentsSDKEnabled           bool   `json:"agentsSdkEnabled"`
	AgentResponsesEnabled      bool   `json:"agentResponsesEnabled"`
	TraceMode                  string `json:"traceMode"`
	OpsCoreSyncEnabled         bool   `json:"opscoreSyncEnabled"`
	ExternalAgentEnabled       bool   `json:"externalAgentEnabled"`
	CodexCollaborationEnabled  bool   `json:"codexCollaborationEnabled"`
	ExternalAgentTaskDirectory string `json:"externalAgentTaskDirectory"`
}

type WorkMemorySettings struct {
	Enabled                    bool                          `json:"enabled"`
	TimeMachineEnabled         bool                          `json:"timeMachineEnabled"`
	AutoCaptureIntervalSeconds int                           `json:"autoCaptureIntervalSeconds"`
	WindowSwitchCaptureEnabled bool                          `json:"windowSwitchCaptureEnabled"`
	WindowSwitchCooldownSecs   int                           `json:"windowSwitchCooldownSeconds"`
	AppCaptureProfiles         []WorkMemoryAppCaptureProfile `json:"appCaptureProfiles"`
	CaptureScope               string                        `json:"captureScope"`
	ScreenshotQuality          int                           `json:"screenshotQuality"`
	MultiMonitor               string                        `json:"multiMonitor"`
	PrivacyMode                bool                          `json:"privacyMode"`
	PauseOnIdle                bool                          `json:"pauseOnIdle"`
	IdlePauseSeconds           int                           `json:"idlePauseSeconds"`
	PauseOnLock                bool                          `json:"pauseOnLock"`
	SourceClipboard            bool                          `json:"sourceClipboard"`
	SourceCaptureHistory       bool                          `json:"sourceCaptureHistory"`
	SourceManualNote           bool                          `json:"sourceManualNote"`
	SourceSearchFavorite       bool                          `json:"sourceSearchFavorite"`
	SourceActions              bool                          `json:"sourceActions"`
	AutoOCR                    bool                          `json:"autoOcr"`
	DraftScheduleEnabled       bool                          `json:"draftScheduleEnabled"`
	DraftScheduleIntervalMin   int                           `json:"draftScheduleIntervalMinutes"`
	DailyDraftScheduleEnabled  bool                          `json:"dailyDraftScheduleEnabled"`
	RetroDraftScheduleEnabled  bool                          `json:"retrospectiveDraftScheduleEnabled"`
	ExperienceScheduleEnabled  bool                          `json:"experienceScheduleEnabled"`
	ExperienceDiscoveryEnabled bool                          `json:"experienceDiscoveryEnabled"`
	ExperienceDiscoveryDays    int                           `json:"experienceDiscoveryDays"`
	SkillSuggestionEnabled     bool                          `json:"skillSuggestionEnabled"`
	WorkflowSuggestionEnabled  bool                          `json:"workflowSuggestionEnabled"`
	RetentionDays              int                           `json:"retentionDays"`
	ThumbnailRetentionDays     int                           `json:"thumbnailRetentionDays"`
	MaxStorageMB               int                           `json:"maxStorageMb"`
	KeepFavoritesForever       bool                          `json:"keepFavoritesForever"`
	ExcludeApps                []string                      `json:"excludeApps"`
	ExcludeWindowKeywords      []string                      `json:"excludeWindowKeywords"`
	ExcludePaths               []string                      `json:"excludePaths"`
	ExcludeURLs                []string                      `json:"excludeUrls"`
	ExcludeContentPatterns     []string                      `json:"excludeContentPatterns"`
	SensitiveRulesEnabled      bool                          `json:"sensitiveRulesEnabled"`
	AllowSensitiveExport       bool                          `json:"allowSensitiveExport"`
}

type WorkMemoryAppCaptureProfile struct {
	ID                       string `json:"id"`
	DisplayName              string `json:"displayName"`
	ProcessName              string `json:"processName"`
	Icon                     string `json:"icon,omitempty"`
	Enabled                  bool   `json:"enabled"`
	WindowSwitchDelaySeconds int    `json:"windowSwitchDelaySeconds"`
	ActiveIntervalSeconds    int    `json:"activeIntervalSeconds"`
}

type PluginSettings struct {
	Enabled map[string]bool `json:"enabled"`
}

const currentSettingsVersion = 14

type AppSettings struct {
	Version    int                `json:"version"`
	General    GeneralSettings    `json:"general"`
	Hotkeys    Hotkeys            `json:"hotkeys"`
	Screenshot ScreenshotSettings `json:"screenshot"`
	WorkMemory WorkMemorySettings `json:"workMemory"`
	AI         AISettings         `json:"ai"`
	Plugins    PluginSettings     `json:"plugins"`
}

type LegacyConfigStatus struct {
	Path         string   `json:"path"`
	Exists       bool     `json:"exists"`
	NeedsImport  bool     `json:"needsImport"`
	ImportedKeys []string `json:"importedKeys"`
	Notes        []string `json:"notes"`
}

type StorageStatus struct {
	Path              string   `json:"path"`
	Directory         string   `json:"directory"`
	DirectoryExists   bool     `json:"directoryExists"`
	Exists            bool     `json:"exists"`
	Bytes             int64    `json:"bytes"`
	ReadBackOK        bool     `json:"readBackOk"`
	ReadBackBytes     int64    `json:"readBackBytes"`
	ReadBackVersion   int      `json:"readBackVersion"`
	Entries           []string `json:"entries"`
	VirtualizedPath   string   `json:"virtualizedPath,omitempty"`
	VirtualizedExists bool     `json:"virtualizedExists"`
	VirtualizedBytes  int64    `json:"virtualizedBytes"`
	AppDataEnv        string   `json:"appDataEnv"`
	LocalAppDataEnv   string   `json:"localAppDataEnv"`
	UserConfigDir     string   `json:"userConfigDir"`
	WorkingDir        string   `json:"workingDir"`
	ExecutablePath    string   `json:"executablePath"`
	LastSaveError     string   `json:"lastSaveError,omitempty"`
	ReadBackError     string   `json:"readBackError,omitempty"`
}

type Service struct {
	mu              sync.RWMutex
	configPath      string
	legacyPath      string
	credentialStore securestore.Store
	settings        AppSettings
	saveError       string
	onChange        []func(AppSettings)
}

func NewService() *Service {
	return NewServiceWithPathsAndCredentialStore(defaultConfigPath(), defaultLegacyConfigPath(), securestore.DefaultStore{})
}

func NewServiceWithPaths(configPath string, legacyPath string) *Service {
	return NewServiceWithPathsAndCredentialStore(configPath, legacyPath, nil)
}

func NewServiceWithPathsAndCredentialStore(configPath string, legacyPath string, credentialStore securestore.Store) *Service {
	service := &Service{
		configPath:      configPath,
		legacyPath:      legacyPath,
		credentialStore: credentialStore,
		settings:        defaultSettings(),
	}
	service.load()
	return service
}

func (s *Service) GetSettings() AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Service) UpdateSettings(next AppSettings) AppSettings {
	s.mu.Lock()
	s.settings = normalizeSettings(next)
	s.saveError = ""
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
	}
	updated := s.settings
	callbacks := append([]func(AppSettings){}, s.onChange...)
	s.mu.Unlock()
	notifySettingsChanged(callbacks, updated)
	return updated
}

func (s *Service) ResetSettings() AppSettings {
	s.mu.Lock()
	s.settings = defaultSettings()
	s.saveError = ""
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
	}
	updated := s.settings
	callbacks := append([]func(AppSettings){}, s.onChange...)
	s.mu.Unlock()
	notifySettingsChanged(callbacks, updated)
	return updated
}

func RegisterChangeHandler(service *Service, callback func(AppSettings)) {
	if service == nil {
		return
	}
	service.registerChangeHandler(callback)
}

func (s *Service) registerChangeHandler(callback func(AppSettings)) {
	if callback == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = append(s.onChange, callback)
}

func (s *Service) LegacyConfigStatus() LegacyConfigStatus {
	return s.legacyConfigStatus(false)
}

func (s *Service) ImportLegacyConfig() AppSettings {
	s.mu.Lock()

	next, _ := importLegacyConfig(s.settings, s.legacyPath, s.credentialStore)
	s.settings = normalizeSettings(next)
	s.saveError = ""
	if err := s.saveLocked(); err != nil {
		s.saveError = err.Error()
	}
	updated := s.settings
	callbacks := append([]func(AppSettings){}, s.onChange...)
	s.mu.Unlock()
	notifySettingsChanged(callbacks, updated)
	return updated
}

func notifySettingsChanged(callbacks []func(AppSettings), settings AppSettings) {
	for _, callback := range callbacks {
		callback(settings)
	}
}

func (s *Service) StorageStatus() StorageStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storageStatusLocked()
}

func (s *Service) storageStatusLocked() StorageStatus {
	storagePath := firstNonEmpty(appdb.DatabasePathForPath(s.configPath), s.configPath)
	info, err := os.Stat(storagePath)
	legacyInfo, legacyErr := os.Stat(s.configPath)
	dir := filepath.Dir(storagePath)
	dirInfo, dirErr := os.Stat(dir)
	entries := []string{}
	if dirErr == nil && dirInfo.IsDir() {
		if items, readErr := os.ReadDir(dir); readErr == nil {
			for _, item := range items {
				entries = append(entries, item.Name())
			}
		}
	}
	var size int64
	if err == nil {
		size = info.Size()
	} else if legacyErr == nil {
		size = legacyInfo.Size()
	}
	readBackOK := false
	readBackBytes := int64(0)
	readBackVersion := 0
	readBackError := ""
	if persisted, ok, readErr := loadSettingsFromSQLite(s.configPath); readErr == nil && ok {
		readBackBytes = infoSize(info, err)
		if readBackBytes == 0 {
			readBackBytes = size
		}
		readBackOK = true
		readBackVersion = persisted.Version
	} else if readErr != nil {
		readBackError = readErr.Error()
	} else if raw, legacyReadErr := os.ReadFile(s.configPath); legacyReadErr == nil {
		var persisted AppSettings
		readBackBytes = int64(len(raw))
		if unmarshalErr := json.Unmarshal(raw, &persisted); unmarshalErr == nil {
			readBackOK = true
			readBackVersion = persisted.Version
		} else {
			readBackError = unmarshalErr.Error()
		}
	}
	userConfigDir, _ := os.UserConfigDir()
	workingDir, _ := os.Getwd()
	executablePath, _ := os.Executable()
	localAppData := os.Getenv("LOCALAPPDATA")
	virtualizedPath, virtualizedExists, virtualizedBytes := findVirtualizedConfigPath(storagePath, os.Getenv("APPDATA"), localAppData)
	return StorageStatus{
		Path:              storagePath,
		Directory:         dir,
		DirectoryExists:   dirErr == nil && dirInfo.IsDir(),
		Exists:            err == nil || legacyErr == nil,
		Bytes:             size,
		ReadBackOK:        readBackOK,
		ReadBackBytes:     readBackBytes,
		ReadBackVersion:   readBackVersion,
		Entries:           entries,
		VirtualizedPath:   virtualizedPath,
		VirtualizedExists: virtualizedExists,
		VirtualizedBytes:  virtualizedBytes,
		AppDataEnv:        os.Getenv("APPDATA"),
		LocalAppDataEnv:   localAppData,
		UserConfigDir:     userConfigDir,
		WorkingDir:        workingDir,
		ExecutablePath:    executablePath,
		LastSaveError:     s.saveError,
		ReadBackError:     readBackError,
	}
}

func (s *Service) legacyConfigStatus(includeKeys bool) LegacyConfigStatus {
	status := LegacyConfigStatus{
		Path:  s.legacyPath,
		Notes: []string{"旧版 x-tools 配置会导入安全用户偏好；旧版明文密钥如存在，只迁入安全凭据存储，不写入 Ariadne JSON。"},
	}
	raw, err := os.ReadFile(s.legacyPath)
	if err != nil {
		status.Exists = false
		return status
	}
	status.Exists = true
	if !includeKeys {
		var legacy map[string]any
		if json.Unmarshal(raw, &legacy) == nil {
			status.ImportedKeys = legacyImportedKeys(legacy)
			status.NeedsImport = legacyConfigNeedsImport(s.settings, legacy, s.credentialStore)
			appendLegacySecretStatus(&status, legacy, s.credentialStore)
			if status.Exists && !status.NeedsImport {
				status.Notes = append(status.Notes, "旧版配置已与 Ariadne 当前设置一致，迁移入口默认隐藏。")
			}
		}
		return status
	}
	return status
}

func (s *Service) load() {
	loaded, err := readSettingsFile(s.configPath)
	if err != nil {
		return
	}
	normalized := normalizeSettings(migrateSettings(loaded))
	s.settings = normalized
	if !reflect.DeepEqual(loaded, normalized) {
		s.saveError = ""
		if err := s.saveLocked(); err != nil {
			s.saveError = err.Error()
		}
	}
}

func (s *Service) saveLocked() error {
	if s.configPath == "" {
		return nil
	}
	if err := saveSettingsToSQLite(s.configPath, s.settings); err != nil {
		return err
	}
	if _, err := readSettingsFile(s.configPath); err != nil {
		return fmt.Errorf("settings write could not be read back: %w", err)
	}
	return nil
}

func readSettingsFile(path string) (AppSettings, error) {
	if loaded, ok, err := loadSettingsFromSQLite(path); err != nil {
		return AppSettings{}, err
	} else if ok {
		return loaded, nil
	}
	return AppSettings{}, os.ErrNotExist
}

func findVirtualizedConfigPath(configPath string, appData string, localAppData string) (string, bool, int64) {
	if configPath == "" || appData == "" || localAppData == "" {
		return "", false, 0
	}
	relative, ok := pathRelativeTo(configPath, appData)
	if !ok {
		return "", false, 0
	}
	pattern := filepath.Join(localAppData, "Packages", "*", "LocalCache", "Roaming", relative)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false, 0
	}
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil || rightErr != nil {
			return matches[i] < matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr == nil && !info.IsDir() {
			return match, true, info.Size()
		}
	}
	return "", false, 0
}

func pathRelativeTo(path string, base string) (string, bool) {
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)
	relative, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return relative, true
}

func importLegacyConfig(current AppSettings, path string, credentialStore securestore.Store) (AppSettings, LegacyConfigStatus) {
	next := current
	status := LegacyConfigStatus{
		Path:  path,
		Notes: []string{"旧版 x-tools 配置会导入安全用户偏好；旧版明文密钥如存在，只迁入安全凭据存储，不写入 Ariadne JSON。"},
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return next, status
	}
	status.Exists = true

	var legacy map[string]any
	if err := json.Unmarshal(raw, &legacy); err != nil {
		status.Notes = append(status.Notes, "旧配置 JSON 无法解析，已保留 Ariadne 当前设置。")
		return next, status
	}

	status.ImportedKeys = legacyImportedKeys(legacy)
	status.NeedsImport = legacyConfigNeedsImport(current, legacy, credentialStore)
	migrateLegacySecrets(&status, legacy, credentialStore)
	next = applyLegacyPreferences(next, legacy)
	return normalizeSettings(next), status
}

func applyLegacyPreferences(next AppSettings, legacy map[string]any) AppSettings {
	if value, ok := legacy["theme"]; ok {
		next.General.Theme = normalizeTheme(asString(value, next.General.Theme))
	}
	if value, ok := legacy["run_on_startup"]; ok {
		next.General.RunOnStartup = asBool(value, next.General.RunOnStartup)
	}
	if hotkeys, ok := legacy["hotkeys"].(map[string]any); ok {
		next.Hotkeys.ToggleWindow = asString(hotkeys["toggle_window"], next.Hotkeys.ToggleWindow)
		next.Hotkeys.Screenshot = asString(hotkeys["screenshot"], next.Hotkeys.Screenshot)
		next.Hotkeys.PinClipboard = asString(hotkeys["pin_clipboard"], next.Hotkeys.PinClipboard)
	}
	next.Screenshot.AutoCopy = asBool(legacy["screenshot_auto_copy"], next.Screenshot.AutoCopy)
	next.Screenshot.AutoPin = asBool(legacy["screenshot_auto_pin"], next.Screenshot.AutoPin)
	next.Screenshot.AutoSave = asBool(legacy["screenshot_auto_save"], next.Screenshot.AutoSave)
	next.Screenshot.SaveDir = asString(legacy["screenshot_save_dir"], next.Screenshot.SaveDir)
	next.Screenshot.FilenameTemplate = asString(legacy["screenshot_filename_template"], next.Screenshot.FilenameTemplate)

	if workMemory, ok := legacy["work_memory"].(map[string]any); ok {
		next.WorkMemory.Enabled = asBool(workMemory["enabled"], next.WorkMemory.Enabled)
		next.WorkMemory.TimeMachineEnabled = asBool(workMemory["time_machine_enabled"], next.WorkMemory.TimeMachineEnabled)
		next.WorkMemory.AutoCaptureIntervalSeconds = asInt(workMemory["auto_capture_interval_seconds"], next.WorkMemory.AutoCaptureIntervalSeconds)
		next.WorkMemory.WindowSwitchCaptureEnabled = asBool(workMemory["window_switch_capture_enabled"], next.WorkMemory.WindowSwitchCaptureEnabled)
		next.WorkMemory.WindowSwitchCooldownSecs = asInt(workMemory["window_switch_cooldown_seconds"], next.WorkMemory.WindowSwitchCooldownSecs)
		next.WorkMemory.CaptureScope = asString(workMemory["capture_scope"], next.WorkMemory.CaptureScope)
		next.WorkMemory.ScreenshotQuality = asInt(workMemory["screenshot_quality"], next.WorkMemory.ScreenshotQuality)
		next.WorkMemory.MultiMonitor = asString(workMemory["multi_monitor"], next.WorkMemory.MultiMonitor)
		next.WorkMemory.PrivacyMode = asBool(workMemory["privacy_mode"], next.WorkMemory.PrivacyMode)
		next.WorkMemory.PauseOnIdle = asBool(workMemory["pause_on_idle"], next.WorkMemory.PauseOnIdle)
		next.WorkMemory.IdlePauseSeconds = asInt(workMemory["idle_pause_seconds"], next.WorkMemory.IdlePauseSeconds)
		next.WorkMemory.PauseOnLock = asBool(workMemory["pause_on_lock"], next.WorkMemory.PauseOnLock)
		next.WorkMemory.SourceClipboard = asBool(workMemory["source_clipboard"], next.WorkMemory.SourceClipboard)
		next.WorkMemory.SourceCaptureHistory = asBool(workMemory["source_capture_history"], next.WorkMemory.SourceCaptureHistory)
		next.WorkMemory.SourceManualNote = asBool(workMemory["source_manual_note"], next.WorkMemory.SourceManualNote)
		next.WorkMemory.SourceSearchFavorite = asBool(workMemory["source_search_favorite"], next.WorkMemory.SourceSearchFavorite)
		next.WorkMemory.SourceActions = asBool(workMemory["source_actions"], next.WorkMemory.SourceActions)
		next.WorkMemory.AutoOCR = asBool(workMemory["auto_ocr"], next.WorkMemory.AutoOCR)
		next.WorkMemory.ExperienceDiscoveryEnabled = asBool(workMemory["experience_discovery_enabled"], next.WorkMemory.ExperienceDiscoveryEnabled)
		next.WorkMemory.ExperienceDiscoveryDays = asInt(workMemory["experience_discovery_period_days"], next.WorkMemory.ExperienceDiscoveryDays)
		next.WorkMemory.SkillSuggestionEnabled = asBool(workMemory["skill_suggestion_enabled"], next.WorkMemory.SkillSuggestionEnabled)
		next.WorkMemory.WorkflowSuggestionEnabled = asBool(workMemory["workflow_suggestion_enabled"], next.WorkMemory.WorkflowSuggestionEnabled)
		next.WorkMemory.RetentionDays = asInt(workMemory["retention_days"], next.WorkMemory.RetentionDays)
		next.WorkMemory.ThumbnailRetentionDays = asInt(workMemory["thumbnail_retention_days"], next.WorkMemory.ThumbnailRetentionDays)
		next.WorkMemory.MaxStorageMB = asInt(workMemory["max_storage_mb"], next.WorkMemory.MaxStorageMB)
		next.WorkMemory.KeepFavoritesForever = asBool(workMemory["keep_favorites_forever"], next.WorkMemory.KeepFavoritesForever)
		next.WorkMemory.ExcludeApps = asStringList(workMemory["exclude_apps"], next.WorkMemory.ExcludeApps)
		next.WorkMemory.ExcludeWindowKeywords = asStringList(workMemory["exclude_window_keywords"], next.WorkMemory.ExcludeWindowKeywords)
		next.WorkMemory.ExcludePaths = asStringList(workMemory["exclude_paths"], next.WorkMemory.ExcludePaths)
		next.WorkMemory.ExcludeURLs = asStringList(firstLegacyValue(workMemory, "exclude_urls", "exclude_url_patterns"), next.WorkMemory.ExcludeURLs)
		next.WorkMemory.ExcludeContentPatterns = asStringList(workMemory["exclude_content_patterns"], next.WorkMemory.ExcludeContentPatterns)
		next.WorkMemory.SensitiveRulesEnabled = asBool(workMemory["sensitive_rules_enabled"], next.WorkMemory.SensitiveRulesEnabled)
		next.WorkMemory.AllowSensitiveExport = asBool(workMemory["allow_sensitive_export"], next.WorkMemory.AllowSensitiveExport)

		next.AI.Enabled = asBool(workMemory["ai_enabled"], next.AI.Enabled)
		next.AI.Provider = asString(workMemory["ai_provider"], next.AI.Provider)
		next.AI.BaseURL = asString(workMemory["ai_base_url"], next.AI.BaseURL)
		next.AI.Model = asString(workMemory["ai_model"], next.AI.Model)
		next.AI.OCRModelEnabled = asBool(firstLegacyValue(workMemory, "ocr_model_enabled", "ai_ocr_enabled"), next.AI.OCRModelEnabled)
		next.AI.OCRProvider = asString(firstLegacyValue(workMemory, "ocr_provider", "ai_ocr_provider"), next.AI.OCRProvider)
		next.AI.OCRBaseURL = asString(firstLegacyValue(workMemory, "ocr_base_url", "ai_ocr_base_url"), next.AI.OCRBaseURL)
		next.AI.OCRModel = asString(firstLegacyValue(workMemory, "ocr_model", "ai_ocr_model"), next.AI.OCRModel)
		next.AI.EmbeddingEnabled = asBool(workMemory["embedding_enabled"], next.AI.EmbeddingEnabled)
		next.AI.EmbeddingProvider = asString(workMemory["embedding_provider"], next.AI.EmbeddingProvider)
		next.AI.EmbeddingBaseURL = asString(workMemory["embedding_base_url"], next.AI.EmbeddingBaseURL)
		next.AI.EmbeddingModel = asString(workMemory["embedding_model"], next.AI.EmbeddingModel)
		next.AI.VectorStoreType = asString(workMemory["vector_store_type"], next.AI.VectorStoreType)
		next.AI.VectorStoreURI = asString(workMemory["vector_store_uri"], next.AI.VectorStoreURI)
		next.AI.VectorCollection = asString(workMemory["vector_collection"], next.AI.VectorCollection)
		next.AI.AgentsSDKEnabled = asBool(workMemory["agents_sdk_enabled"], next.AI.AgentsSDKEnabled)
		next.AI.AgentResponsesEnabled = asBool(firstLegacyValue(workMemory, "agent_responses_enabled", "agents_responses_enabled", "agents_sdk_responses_enabled"), next.AI.AgentResponsesEnabled)
		next.AI.TraceMode = asString(workMemory["trace_mode"], next.AI.TraceMode)
		next.AI.OpsCoreSyncEnabled = asBool(workMemory["opscore_sync_enabled"], next.AI.OpsCoreSyncEnabled)
		next.AI.ExternalAgentEnabled = asBool(workMemory["external_agent_enabled"], next.AI.ExternalAgentEnabled)
		next.AI.CodexCollaborationEnabled = asBool(workMemory["codex_collaboration_enabled"], next.AI.CodexCollaborationEnabled)
		next.AI.ExternalAgentTaskDirectory = asString(workMemory["external_agent_task_dir"], next.AI.ExternalAgentTaskDirectory)
	}
	if plugins, ok := legacy["plugins_enabled"].(map[string]any); ok {
		next.Plugins.Enabled = map[string]bool{}
		for key, value := range plugins {
			next.Plugins.Enabled[key] = asBool(value, true)
		}
	}
	return normalizeSettings(next)
}

func legacyConfigNeedsImport(current AppSettings, legacy map[string]any, credentialStore securestore.Store) bool {
	if len(legacyImportedKeys(legacy)) == 0 {
		return false
	}
	current = normalizeSettings(current)
	next := normalizeSettings(applyLegacyPreferences(current, legacy))
	if !reflect.DeepEqual(current, next) {
		return true
	}
	for _, candidate := range legacySecretCandidates(legacy) {
		if credentialStore == nil || !credentialStore.Available() {
			return true
		}
		stored, ok, err := credentialStore.Read(candidate.target)
		if err != nil || !ok || stored != candidate.value {
			return true
		}
	}
	return false
}

func defaultSettings() AppSettings {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return AppSettings{
		Version: currentSettingsVersion,
		General: GeneralSettings{
			Theme:        "light",
			RunOnStartup: false,
			Language:     "zh-CN",
		},
		Hotkeys: Hotkeys{
			ToggleWindow: "alt+q",
			Screenshot:   "alt+a",
			PinClipboard: "alt+v",
		},
		Screenshot: ScreenshotSettings{
			AutoCopy:         false,
			AutoPin:          false,
			AutoSave:         false,
			SaveDir:          filepath.Join(home, "Pictures", "Ariadne"),
			FilenameTemplate: "ariadne_{date}_{time}",
			Quality:          90,
		},
		WorkMemory: WorkMemorySettings{
			Enabled:                    true,
			TimeMachineEnabled:         false,
			AutoCaptureIntervalSeconds: 60,
			WindowSwitchCaptureEnabled: true,
			WindowSwitchCooldownSecs:   3,
			CaptureScope:               "active_window",
			ScreenshotQuality:          90,
			MultiMonitor:               "combined",
			PrivacyMode:                false,
			PauseOnIdle:                true,
			IdlePauseSeconds:           600,
			PauseOnLock:                true,
			SourceClipboard:            true,
			SourceCaptureHistory:       true,
			SourceManualNote:           true,
			SourceSearchFavorite:       true,
			SourceActions:              true,
			AutoOCR:                    true,
			DraftScheduleEnabled:       true,
			DraftScheduleIntervalMin:   240,
			DailyDraftScheduleEnabled:  true,
			RetroDraftScheduleEnabled:  true,
			ExperienceScheduleEnabled:  true,
			ExperienceDiscoveryEnabled: true,
			ExperienceDiscoveryDays:    7,
			SkillSuggestionEnabled:     true,
			WorkflowSuggestionEnabled:  true,
			RetentionDays:              30,
			ThumbnailRetentionDays:     90,
			MaxStorageMB:               1024,
			KeepFavoritesForever:       true,
			ExcludeApps: []string{
				"1password.exe",
				"bitwarden.exe",
				"keepass.exe",
				"lastpass.exe",
				"credentialuibroker.exe",
				"lockapp.exe",
				"logonui.exe",
				"mstsc.exe",
				"remotehelp.exe",
			},
			ExcludeWindowKeywords: []string{
				"password",
				"passwd",
				"token",
				"secret",
				"otp",
				"验证码",
				"密码",
				"登录",
				"支付",
				"隐私",
				"无痕",
				"private",
				"incognito",
				"remote desktop",
				"远程桌面",
				"堡垒机",
				"vpn",
				"sso",
			},
			ExcludePaths:           []string{},
			ExcludeURLs:            []string{},
			ExcludeContentPatterns: []string{},
			SensitiveRulesEnabled:  true,
			AllowSensitiveExport:   false,
		},
		AI: AISettings{
			Enabled:                    false,
			Provider:                   "disabled",
			BaseURL:                    "",
			Model:                      "",
			OCRModelEnabled:            false,
			OCRProvider:                "openai-compatible",
			OCRBaseURL:                 "",
			OCRModel:                   "",
			EmbeddingEnabled:           false,
			EmbeddingProvider:          "disabled",
			EmbeddingBaseURL:           "",
			EmbeddingModel:             "",
			VectorStoreType:            "disabled",
			VectorStoreURI:             "",
			VectorCollection:           "ariadne_work_memory",
			AgentsSDKEnabled:           true,
			AgentResponsesEnabled:      true,
			TraceMode:                  "off",
			OpsCoreSyncEnabled:         false,
			ExternalAgentEnabled:       true,
			CodexCollaborationEnabled:  false,
			ExternalAgentTaskDirectory: filepath.Join(home, "Documents", "Ariadne", "agent_tasks"),
		},
		Plugins: PluginSettings{Enabled: map[string]bool{}},
	}
}

func normalizeSettings(value AppSettings) AppSettings {
	defaults := defaultSettings()
	if value.Version < defaults.Version {
		value.Version = defaults.Version
	}
	value.General.Theme = normalizeTheme(firstNonEmpty(value.General.Theme, defaults.General.Theme))
	value.General.Language = firstNonEmpty(value.General.Language, defaults.General.Language)
	value.Hotkeys.ToggleWindow = firstNonEmpty(value.Hotkeys.ToggleWindow, defaults.Hotkeys.ToggleWindow)
	value.Hotkeys.Screenshot = firstNonEmpty(value.Hotkeys.Screenshot, defaults.Hotkeys.Screenshot)
	value.Hotkeys.PinClipboard = firstNonEmpty(value.Hotkeys.PinClipboard, defaults.Hotkeys.PinClipboard)
	value.Screenshot.SaveDir = firstNonEmpty(value.Screenshot.SaveDir, defaults.Screenshot.SaveDir)
	value.Screenshot.FilenameTemplate = firstNonEmpty(value.Screenshot.FilenameTemplate, defaults.Screenshot.FilenameTemplate)
	value.Screenshot.Quality = clamp(value.Screenshot.Quality, 1, 100, defaults.Screenshot.Quality)

	value.WorkMemory.AutoCaptureIntervalSeconds = clamp(value.WorkMemory.AutoCaptureIntervalSeconds, 10, 86400, defaults.WorkMemory.AutoCaptureIntervalSeconds)
	value.WorkMemory.WindowSwitchCooldownSecs = clamp(value.WorkMemory.WindowSwitchCooldownSecs, 3, 3600, defaults.WorkMemory.WindowSwitchCooldownSecs)
	value.WorkMemory.AppCaptureProfiles = normalizeAppCaptureProfiles(value.WorkMemory.AppCaptureProfiles)
	value.WorkMemory.CaptureScope = oneOf(value.WorkMemory.CaptureScope, defaults.WorkMemory.CaptureScope, "all_screens", "active_window", "primary_screen")
	value.WorkMemory.ScreenshotQuality = clamp(value.WorkMemory.ScreenshotQuality, 1, 100, defaults.WorkMemory.ScreenshotQuality)
	value.WorkMemory.MultiMonitor = oneOf(value.WorkMemory.MultiMonitor, defaults.WorkMemory.MultiMonitor, "combined", "per_monitor", "primary_only")
	value.WorkMemory.IdlePauseSeconds = clamp(value.WorkMemory.IdlePauseSeconds, 30, 86400, defaults.WorkMemory.IdlePauseSeconds)
	value.WorkMemory.DraftScheduleIntervalMin = clamp(value.WorkMemory.DraftScheduleIntervalMin, 15, 1440, defaults.WorkMemory.DraftScheduleIntervalMin)
	value.WorkMemory.ExperienceDiscoveryDays = clamp(value.WorkMemory.ExperienceDiscoveryDays, 1, 365, defaults.WorkMemory.ExperienceDiscoveryDays)
	value.WorkMemory.RetentionDays = clamp(value.WorkMemory.RetentionDays, 1, 3650, defaults.WorkMemory.RetentionDays)
	value.WorkMemory.ThumbnailRetentionDays = clamp(value.WorkMemory.ThumbnailRetentionDays, 1, 3650, defaults.WorkMemory.ThumbnailRetentionDays)
	value.WorkMemory.MaxStorageMB = clamp(value.WorkMemory.MaxStorageMB, 128, 1024*1024, defaults.WorkMemory.MaxStorageMB)
	value.WorkMemory.ExcludeApps = cleanList(value.WorkMemory.ExcludeApps, defaults.WorkMemory.ExcludeApps)
	value.WorkMemory.ExcludeWindowKeywords = cleanList(value.WorkMemory.ExcludeWindowKeywords, defaults.WorkMemory.ExcludeWindowKeywords)
	value.WorkMemory.ExcludePaths = cleanList(value.WorkMemory.ExcludePaths, nil)
	value.WorkMemory.ExcludeURLs = cleanList(value.WorkMemory.ExcludeURLs, nil)
	value.WorkMemory.ExcludeContentPatterns = cleanList(value.WorkMemory.ExcludeContentPatterns, nil)

	value.AI.Provider = firstNonEmpty(value.AI.Provider, defaults.AI.Provider)
	value.AI.OCRProvider = firstNonEmpty(value.AI.OCRProvider, defaults.AI.OCRProvider)
	value.AI.OCRBaseURL = strings.TrimSpace(value.AI.OCRBaseURL)
	value.AI.OCRModel = strings.TrimSpace(value.AI.OCRModel)
	value.AI.EmbeddingProvider = firstNonEmpty(value.AI.EmbeddingProvider, defaults.AI.EmbeddingProvider)
	value.AI.VectorStoreType = firstNonEmpty(value.AI.VectorStoreType, defaults.AI.VectorStoreType)
	value.AI.VectorCollection = firstNonEmpty(value.AI.VectorCollection, defaults.AI.VectorCollection)
	value.AI.TraceMode = oneOf(value.AI.TraceMode, defaults.AI.TraceMode, "off", "local", "internal")
	value.AI.ExternalAgentTaskDirectory = firstNonEmpty(value.AI.ExternalAgentTaskDirectory, defaults.AI.ExternalAgentTaskDirectory)
	if value.Plugins.Enabled == nil {
		value.Plugins.Enabled = map[string]bool{}
	}
	value.Plugins.Enabled = normalizePluginEnabled(value.Plugins.Enabled)
	return value
}

func normalizePluginEnabled(enabled map[string]bool) map[string]bool {
	normalized := make(map[string]bool, len(enabled))
	for key, value := range enabled {
		id := strings.ToLower(strings.TrimSpace(key))
		if id == "" {
			continue
		}
		normalized[id] = value
	}
	return normalized
}

func normalizeAppCaptureProfiles(profiles []WorkMemoryAppCaptureProfile) []WorkMemoryAppCaptureProfile {
	if len(profiles) == 0 {
		return []WorkMemoryAppCaptureProfile{}
	}
	seen := map[string]bool{}
	normalized := make([]WorkMemoryAppCaptureProfile, 0, len(profiles))
	for _, profile := range profiles {
		processName := strings.TrimSpace(profile.ProcessName)
		displayName := strings.TrimSpace(profile.DisplayName)
		icon := strings.TrimSpace(profile.Icon)
		id := strings.TrimSpace(profile.ID)
		if processName == "" && displayName == "" && id == "" {
			continue
		}
		if processName == "" {
			processName = firstNonEmpty(displayName, id)
		}
		if displayName == "" {
			displayName = processName
		}
		key := normalizeAppProfileKey(firstNonEmpty(firstNonEmpty(processName, displayName), id))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if id == "" {
			id = key
		}
		normalized = append(normalized, WorkMemoryAppCaptureProfile{
			ID:                       key,
			DisplayName:              displayName,
			ProcessName:              processName,
			Icon:                     icon,
			Enabled:                  profile.Enabled,
			WindowSwitchDelaySeconds: clampAllowZero(profile.WindowSwitchDelaySeconds, 3600),
			ActiveIntervalSeconds:    clamp(profile.ActiveIntervalSeconds, 10, 86400, 120),
		})
	}
	return normalized
}

func clampAllowZero(value int, max int) int {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func normalizeAppProfileKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.Base(value)
	return strings.ToLower(value)
}

func migrateSettings(value AppSettings) AppSettings {
	if value.Version < currentSettingsVersion && legacyThemeNeedsLightReset(value.General.Theme) {
		value.General.Theme = "light"
	}
	if value.Version < 8 {
		defaults := defaultSettings()
		value.WorkMemory.DraftScheduleIntervalMin = defaults.WorkMemory.DraftScheduleIntervalMin
		value.WorkMemory.DailyDraftScheduleEnabled = defaults.WorkMemory.DailyDraftScheduleEnabled
		value.WorkMemory.RetroDraftScheduleEnabled = defaults.WorkMemory.RetroDraftScheduleEnabled
		value.WorkMemory.ExperienceScheduleEnabled = defaults.WorkMemory.ExperienceScheduleEnabled
	}
	if value.Version < 9 {
		defaults := defaultSettings()
		value.WorkMemory.WindowSwitchCaptureEnabled = defaults.WorkMemory.WindowSwitchCaptureEnabled
		value.WorkMemory.WindowSwitchCooldownSecs = defaults.WorkMemory.WindowSwitchCooldownSecs
	}
	if value.Version < 11 {
		defaults := defaultSettings()
		value.WorkMemory.DraftScheduleEnabled = defaults.WorkMemory.DraftScheduleEnabled
	}
	if value.Version < 13 {
		defaults := defaultSettings()
		switch strings.TrimSpace(value.WorkMemory.CaptureScope) {
		case "", "all_screens":
			value.WorkMemory.CaptureScope = defaults.WorkMemory.CaptureScope
		}
	}
	if value.Version < 14 {
		defaults := defaultSettings()
		value.AI.AgentResponsesEnabled = defaults.AI.AgentResponsesEnabled
	}
	return value
}

func legacyThemeNeedsLightReset(theme string) bool {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "dark", "system":
		return true
	default:
		return false
	}
}

func defaultConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "Ariadne", "config.json")
}

func defaultLegacyConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "x-tools", "config.json")
}

func legacyImportedKeys(legacy map[string]any) []string {
	candidates := []string{}
	for _, key := range []string{
		"theme",
		"run_on_startup",
		"hotkeys",
		"screenshot_auto_copy",
		"screenshot_auto_pin",
		"screenshot_auto_save",
		"screenshot_save_dir",
		"screenshot_filename_template",
		"plugins_enabled",
		"work_memory",
	} {
		if _, ok := legacy[key]; ok {
			candidates = append(candidates, key)
		}
	}
	for _, candidate := range legacySecretCandidates(legacy) {
		candidates = append(candidates, "credential:"+candidate.kind)
	}
	sort.Strings(candidates)
	return candidates
}

type legacySecretCandidate struct {
	kind   string
	label  string
	target string
	value  string
}

func legacySecretCandidates(legacy map[string]any) []legacySecretCandidate {
	specs := []struct {
		kind   string
		label  string
		target string
		paths  [][]string
	}{
		{
			kind:   "ai_api_key",
			label:  "AI API key",
			target: securestore.TargetOpenAIAPIKey,
			paths: [][]string{
				{"work_memory", "ai_api_key"},
				{"work_memory", "openai_api_key"},
				{"work_memory", "openai__api_key"},
				{"work_memory", "OPENAI__API_KEY"},
				{"work_memory", "OPENAI_API_KEY"},
				{"work_memory", "api_key"},
				{"ai", "api_key"},
				{"ai", "openai_api_key"},
				{"openai", "api_key"},
				{"ai_api_key"},
				{"openai_api_key"},
				{"openai__api_key"},
				{"OPENAI__API_KEY"},
				{"OPENAI_API_KEY"},
			},
		},
		{
			kind:   "embedding_api_key",
			label:  "Embedding API key",
			target: securestore.TargetEmbeddingAPIKey,
			paths: [][]string{
				{"work_memory", "embedding_api_key"},
				{"work_memory", "embed_api_key"},
				{"work_memory", "embed__api_key"},
				{"work_memory", "EMBED__API_KEY"},
				{"work_memory", "EMBED_API_KEY"},
				{"embedding", "api_key"},
				{"embedding_api_key"},
				{"embed_api_key"},
				{"embed__api_key"},
				{"EMBED__API_KEY"},
				{"EMBED_API_KEY"},
			},
		},
		{
			kind:   "milvus_token",
			label:  "Milvus token",
			target: securestore.TargetMilvusToken,
			paths: [][]string{
				{"work_memory", "milvus_token"},
				{"work_memory", "vector_store_token"},
				{"work_memory", "MILVUS__TOKEN"},
				{"work_memory", "MILVUS_TOKEN"},
				{"milvus", "token"},
				{"milvus_token"},
				{"vector_store_token"},
				{"MILVUS__TOKEN"},
				{"MILVUS_TOKEN"},
			},
		},
	}

	candidates := []legacySecretCandidate{}
	for _, spec := range specs {
		for _, path := range spec.paths {
			if value := legacySecretValue(legacy, path...); value != "" {
				candidates = append(candidates, legacySecretCandidate{
					kind:   spec.kind,
					label:  spec.label,
					target: spec.target,
					value:  value,
				})
				break
			}
		}
	}
	return candidates
}

func legacySecretValue(values map[string]any, path ...string) string {
	if len(path) == 0 {
		return ""
	}
	current := values
	for index, key := range path {
		value, ok := current[key]
		if !ok {
			return ""
		}
		if index == len(path)-1 {
			text, ok := value.(string)
			if !ok {
				return ""
			}
			text = strings.TrimSpace(text)
			if looksLikePlaceholderSecret(text) {
				return ""
			}
			return text
		}
		next, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func looksLikePlaceholderSecret(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return true
	}
	placeholders := []string{
		"changeme",
		"change-me",
		"your-api-key",
		"your_api_key",
		"your api key",
		"api-key",
		"api_key",
		"sk-xxxx",
		"sk-xxx",
		"todo",
	}
	for _, placeholder := range placeholders {
		if normalized == placeholder {
			return true
		}
	}
	return strings.Contains(normalized, "${") || strings.Contains(normalized, "<") || strings.Contains(normalized, ">")
}

func appendLegacySecretStatus(status *LegacyConfigStatus, legacy map[string]any, credentialStore securestore.Store) {
	candidates := legacySecretCandidates(legacy)
	if len(candidates) == 0 {
		return
	}
	if credentialStore == nil || !credentialStore.Available() {
		status.Notes = append(status.Notes, "检测到旧版明文密钥；当前安全凭据存储不可用，密钥不会迁移，也不会写入 Ariadne JSON。")
		return
	}
	for _, candidate := range candidates {
		_, stored, err := credentialStore.Read(candidate.target)
		switch {
		case err != nil:
			status.Notes = append(status.Notes, candidate.label+" 安全存储状态读取失败: "+shortLegacyError(err.Error()))
		case stored:
			status.Notes = append(status.Notes, "检测到旧版 "+candidate.label+"；Ariadne 安全存储中已有对应记录。")
		default:
			status.Notes = append(status.Notes, "检测到旧版 "+candidate.label+"；导入旧配置时会迁入安全凭据存储。")
		}
	}
}

func migrateLegacySecrets(status *LegacyConfigStatus, legacy map[string]any, credentialStore securestore.Store) {
	candidates := legacySecretCandidates(legacy)
	if len(candidates) == 0 {
		return
	}
	if credentialStore == nil || !credentialStore.Available() {
		status.Notes = append(status.Notes, "检测到旧版明文密钥；当前安全凭据存储不可用，密钥未迁移，也未写入 Ariadne JSON。")
		return
	}
	for _, candidate := range candidates {
		if err := credentialStore.Write(candidate.target, candidate.value); err != nil {
			status.Notes = append(status.Notes, "旧版 "+candidate.label+" 迁移失败: "+shortLegacyError(err.Error()))
			continue
		}
		status.Notes = append(status.Notes, "旧版 "+candidate.label+" 已迁入安全凭据存储。")
	}
}

func shortLegacyError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 120 {
		return message
	}
	return message[:117] + "..."
}

func asString(value any, fallback string) string {
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fallback
	}
	return text
}

func asBool(value any, fallback bool) bool {
	boolValue, ok := value.(bool)
	if !ok {
		return fallback
	}
	return boolValue
}

func asInt(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}

func asStringList(value any, fallback []string) []string {
	raw, ok := value.([]any)
	if !ok {
		return fallback
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			items = append(items, text)
		}
	}
	return items
}

func firstLegacyValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func cleanList(value []string, fallback []string) []string {
	if len(value) == 0 && len(fallback) > 0 {
		value = fallback
	}
	seen := map[string]bool{}
	result := []string{}
	for _, item := range value {
		text := strings.TrimSpace(item)
		if text == "" {
			continue
		}
		key := strings.ToLower(text)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, text)
	}
	return result
}

func firstNonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func infoSize(info os.FileInfo, err error) int64 {
	if err != nil || info == nil {
		return 0
	}
	return info.Size()
}

func normalizeTheme(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "light":
		return "light"
	case "professional-pink":
		return "professional-pink"
	case "light-graphite":
		return "light-graphite"
	case "cloud-blue":
		return "cloud-blue"
	case "dark":
		return "dark"
	default:
		return "light"
	}
}

func oneOf(value string, fallback string, allowed ...string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if value == item {
			return value
		}
	}
	return fallback
}

func clamp(value int, min int, max int, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
