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
	AutoCopy         bool     `json:"autoCopy"`
	AutoPin          bool     `json:"autoPin"`
	AutoSave         bool     `json:"autoSave"`
	SaveDir          string   `json:"saveDir"`
	FilenameTemplate string   `json:"filenameTemplate"`
	Quality          int      `json:"quality"`
	AutoRedact       bool     `json:"autoRedact"`
	RedactPhones     bool     `json:"redactPhones"`
	RedactKeywords   []string `json:"redactKeywords"`
}

type AISettings struct {
	OCRModelEnabled bool   `json:"ocrModelEnabled"`
	OCRProvider     string `json:"ocrProvider"`
	OCRBaseURL      string `json:"ocrBaseUrl"`
	OCRModel        string `json:"ocrModel"`
}

type PluginSettings struct {
	Enabled map[string]bool `json:"enabled"`
}

type SearchSettings struct {
	FileExcludeFolders    []string `json:"fileExcludeFolders"`
	FileExcludePatterns   []string `json:"fileExcludePatterns"`
	FileIncludeExtensions []string `json:"fileIncludeExtensions"`
	FileExcludeExtensions []string `json:"fileExcludeExtensions"`
}

type HistorySettings struct {
	ClipboardEnabled     bool `json:"clipboardEnabled"`
	PrivacyMode          bool `json:"privacyMode"`
	RetentionDays        int  `json:"retentionDays"`
	MaxStorageMB         int  `json:"maxStorageMb"`
	KeepFavoritesForever bool `json:"keepFavoritesForever"`
}

const currentSettingsVersion = 23

type AppSettings struct {
	Version    int                `json:"version"`
	General    GeneralSettings    `json:"general"`
	Hotkeys    Hotkeys            `json:"hotkeys"`
	Screenshot ScreenshotSettings `json:"screenshot"`
	History    HistorySettings    `json:"history"`
	AI         AISettings         `json:"ai"`
	Plugins    PluginSettings     `json:"plugins"`
	Search     SearchSettings     `json:"search"`
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

func (s *Service) ConfigPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configPath
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

		next.AI.OCRModelEnabled = asBool(firstLegacyValue(workMemory, "ocr_model_enabled", "ai_ocr_enabled"), next.AI.OCRModelEnabled)
		next.AI.OCRProvider = asString(firstLegacyValue(workMemory, "ocr_provider", "ai_ocr_provider"), next.AI.OCRProvider)
		next.AI.OCRBaseURL = asString(firstLegacyValue(workMemory, "ocr_base_url", "ai_ocr_base_url"), next.AI.OCRBaseURL)
		next.AI.OCRModel = asString(firstLegacyValue(workMemory, "ocr_model", "ai_ocr_model"), next.AI.OCRModel)
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
			Theme:        "cloud-blue",
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
			AutoRedact:       false,
			RedactPhones:     true,
			RedactKeywords:   []string{},
		},
		History: HistorySettings{ClipboardEnabled: true, RetentionDays: 30, MaxStorageMB: 1024, KeepFavoritesForever: true},
		AI: AISettings{
			OCRModelEnabled: false,
			OCRProvider:     "openai-compatible",
			OCRBaseURL:      "",
			OCRModel:        "",
		},
		Plugins: PluginSettings{Enabled: map[string]bool{}},
		Search: SearchSettings{
			FileExcludeFolders:    defaultSearchExcludeFolders(),
			FileExcludePatterns:   defaultSearchExcludePatterns(),
			FileIncludeExtensions: []string{},
			FileExcludeExtensions: defaultSearchExcludeExtensions(),
		},
	}
}

func defaultSearchExcludeFolders() []string {
	return cleanList([]string{
		defaultRecentFolder(),
		defaultRoamingAppDataFolder(),
		strings.TrimSpace(os.Getenv("TEMP")),
		strings.TrimSpace(os.Getenv("TMP")),
		defaultLocalAppDataFolder(),
		defaultLocalLowAppDataFolder(),
		defaultLocalTempFolder(),
		defaultWindowsTempFolder(),
		defaultWindowsFolder(),
		strings.TrimSpace(os.Getenv("ProgramFiles")),
		strings.TrimSpace(os.Getenv("ProgramFiles(x86)")),
		strings.TrimSpace(os.Getenv("ProgramW6432")),
		strings.TrimSpace(os.Getenv("ProgramData")),
	}, nil)
}

func defaultSearchExcludePatterns() []string {
	return []string{
		`(?i)\.(tmp|temp|part|crdownload|download)$`,
		`(?i)(^|[\\/])~\$[^\\/]*$`,
		`(?i)(^|[\\/])\$recycle\.bin([\\/]|$)`,
		`(?i)(^|[\\/])system volume information([\\/]|$)`,
		`(?i)(^|[\\/])(tmp|temp|temp-index|tmp-index)([\\/]|$)`,
		`(?i)[\\/]ebwebview([\\/]|$)`,
		`(?i)(^|[\\/])\..*\.tmp[-.][^\\/]*$`,
		`(?i)[\\/]users[\\/][^\\/]+[\\/]appdata[\\/](local|locallow|roaming)([\\/]|$)`,
		`(?i)^[a-z]:[\\/](app|appdata|program files|program files \(x86\)|programdata|devapp)([\\/]|$)`,
		`(?i)^[a-z]:[\\/]workspace[\\/]env([\\/]|$)`,
		`(?i)^[a-z]:[\\/]users[\\/][^\\/]+[\\/]\.(cache|config|local|m2|gradle|nuget|npm|pnpm|yarn|cargo|rustup|wox|gemini|confirmo|switchhosts|antigravity|antigravity_cockpit|marscode)([\\/]|$)`,
		`(?i)[\\/]appdata[\\/](local|locallow|roaming)[\\/].*[\\/](cache|cache2|cachestorage|code cache|gpucache|shadercache|crashpad|dawncache|blob_storage|inetcache|webcache|startupcache|browsermetrics|media cache|service worker|thumbnails)([\\/]|$)`,
		`(?i)^[a-z]:[\\/](recovery|\$winreagent|config\.msi|windows\.old|msocache)([\\/]|$)`,
		`(?i)(^|[\\/])(d3dscache|dxcache|gpucache|grshadercache|shadercache|dawncache)([\\/]|$)`,
		`(?i)(^|[\\/])(\.m2|\.npm|\.pnpm-store|pnpm-store|npm-cache|pnpm-cache|go-build|pip-cache|ms-playwright|ms-playwright-go|package cache)([\\/]|$)`,
		`(?i)(^|[\\/])(nuget[\\/]packages|\.cargo|\.rustup)([\\/]|$)`,
		`(?i)(^|[\\/])(\.venv|venv|envs|virtualenv|__pypackages__)([\\/]|$)`,
		`(?i)(^|[\\/])(site-packages|dist-packages)([\\/]|$)`,
		`(?i)(^|[\\/])(jetbrains|trae cn|trae|antigravity|zed|qoder)([\\/]|$)`,
		`(?i)(^|[\\/])(logs?|log)([\\/]|$)`,
		`(?i)(^|[\\/])ariadne[\\/](capture_images|capture_thumbnails)([\\/]|$)`,
		`(?i)\.(log|journal|db-journal|sqlite-wal|sqlite-shm|db-wal|db-shm|odlgz|statistic|dxcache-shm|dxcache-wal)$`,
		`(?i)(^|[\\/])(\$mft|\$logfile|\$bitmap|\$boot|\$badclus|\$secure|\$upcase|\$volume|\$attrdef|pagefile\.sys|hiberfil\.sys|swapfile\.sys|dumpstack\.log\.tmp|thumbs\.db|desktop\.ini)$`,
		`(?i)(^|[\\/])(\.git|\.hg|\.svn|node_modules|\.pnpm-store|__pycache__|\.pytest_cache|\.ruff_cache|\.mypy_cache|\.gradle|\.idea|\.vscode|\.cache|\.codex|\.codex-audit|coverage|dist|build|target|out|bin|obj|\.next|\.nuxt|\.vite|\.turbo|\.parcel-cache|\.svelte-kit|\.angular|\.vercel)([\\/]|$)`,
	}
}

func defaultSearchExcludeExtensions() []string {
	return cleanExtensionList([]string{
		".tmp",
		".temp",
		".part",
		".crdownload",
		".download",
		".log",
		".journal",
		".db-journal",
		".sqlite-wal",
		".sqlite-shm",
		".db-wal",
		".db-shm",
		".odlgz",
		".statistic",
		".dxcache-shm",
		".dxcache-wal",
	}, nil)
}

func defaultRecentFolder() string {
	base := defaultRoamingAppDataFolder()
	if base == "" {
		return filepath.Join("Microsoft", "Windows", "Recent")
	}
	return filepath.Join(base, "Microsoft", "Windows", "Recent")
}

func defaultLocalTempFolder() string {
	base := defaultLocalAppDataFolder()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Temp")
}

func defaultRoamingAppDataFolder() string {
	base := strings.TrimSpace(os.Getenv("APPDATA"))
	if base == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
	}
	return base
}

func defaultLocalAppDataFolder() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	return base
}

func defaultLocalLowAppDataFolder() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "AppData", "LocalLow")
}

func defaultWindowsTempFolder() string {
	base := defaultWindowsFolder()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Temp")
}

func defaultWindowsFolder() string {
	base := strings.TrimSpace(os.Getenv("WINDIR"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("SystemRoot"))
	}
	return base
}

func normalizeSettings(value AppSettings) AppSettings {
	defaults := defaultSettings()
	previousVersion := value.Version
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
	value.Screenshot.RedactKeywords = cleanList(value.Screenshot.RedactKeywords, nil)

	value.History.RetentionDays = clamp(value.History.RetentionDays, 1, 3650, defaults.History.RetentionDays)
	value.History.MaxStorageMB = clamp(value.History.MaxStorageMB, 128, 1024*1024, defaults.History.MaxStorageMB)
	value.AI.OCRProvider = firstNonEmpty(value.AI.OCRProvider, defaults.AI.OCRProvider)
	value.AI.OCRBaseURL = strings.TrimSpace(value.AI.OCRBaseURL)
	value.AI.OCRModel = strings.TrimSpace(value.AI.OCRModel)
	if value.Plugins.Enabled == nil {
		value.Plugins.Enabled = map[string]bool{}
	}
	value.Plugins.Enabled = normalizePluginEnabled(value.Plugins.Enabled)
	if previousVersion < 17 && len(value.Search.FileExcludeFolders) == 0 {
		value.Search.FileExcludeFolders = defaults.Search.FileExcludeFolders
	}
	if previousVersion < 20 {
		value.Search.FileExcludeFolders = mergeDefaultList(defaults.Search.FileExcludeFolders, value.Search.FileExcludeFolders)
		value.Search.FileExcludePatterns = mergeDefaultList(defaults.Search.FileExcludePatterns, value.Search.FileExcludePatterns)
	}
	if previousVersion < 21 {
		value.Search.FileExcludeExtensions = mergeDefaultList(defaults.Search.FileExcludeExtensions, value.Search.FileExcludeExtensions)
	}
	value.Search.FileExcludeFolders = cleanList(value.Search.FileExcludeFolders, nil)
	value.Search.FileExcludePatterns = cleanList(value.Search.FileExcludePatterns, nil)
	value.Search.FileIncludeExtensions = cleanExtensionList(value.Search.FileIncludeExtensions, nil)
	value.Search.FileExcludeExtensions = cleanExtensionList(value.Search.FileExcludeExtensions, nil)
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

func migrateSettings(value AppSettings) AppSettings {
	if value.Version < currentSettingsVersion && legacyThemeNeedsDefaultReset(value.General.Theme) {
		value.General.Theme = "cloud-blue"
	}
	if value.Version < 16 {
		value.Screenshot.RedactPhones = true
	}
	return value
}

func legacyThemeNeedsDefaultReset(theme string) bool {
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

func mergeDefaultList(defaults []string, values []string) []string {
	merged := make([]string, 0, len(defaults)+len(values))
	merged = append(merged, defaults...)
	merged = append(merged, values...)
	return cleanList(merged, nil)
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

func cleanExtensionList(value []string, fallback []string) []string {
	if len(value) == 0 && len(fallback) > 0 {
		value = fallback
	}
	seen := map[string]bool{}
	result := []string{}
	for _, item := range value {
		extension := normalizeExtension(item)
		if extension == "" || seen[extension] {
			continue
		}
		seen[extension] = true
		result = append(result, extension)
	}
	return result
}

func normalizeExtension(value string) string {
	item := strings.ToLower(strings.TrimSpace(value))
	item = strings.TrimPrefix(item, "*")
	item = strings.TrimSpace(item)
	if item == "" || item == "." {
		return ""
	}
	if !strings.HasPrefix(item, ".") {
		item = "." + item
	}
	if strings.ContainsAny(item, `/\`) || strings.Contains(item, " ") {
		return ""
	}
	return item
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
	case "professional-pink":
		return "professional-pink"
	case "light-graphite":
		return "light-graphite"
	case "cloud-blue":
		return "cloud-blue"
	default:
		return "cloud-blue"
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
