package filesearch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileSearchPolicy struct {
	ExcludeFolders    []string `json:"excludeFolders"`
	ExcludePatterns   []string `json:"excludePatterns"`
	IncludeExtensions []string `json:"includeExtensions"`
	ExcludeExtensions []string `json:"excludeExtensions"`
}

type fileSearchFilter struct {
	folders           []string
	patterns          []*regexp.Regexp
	includeExtensions []string
	excludeExtensions []string
	errors            []string
}

func DefaultFileSearchPolicy() FileSearchPolicy {
	return FileSearchPolicy{
		ExcludeFolders:    defaultFileExcludeFolders(),
		ExcludePatterns:   defaultFileExcludePatterns(),
		IncludeExtensions: []string{},
		ExcludeExtensions: defaultFileExcludeExtensions(),
	}
}

func NormalizeFileSearchPolicy(policy FileSearchPolicy) FileSearchPolicy {
	defaults := DefaultFileSearchPolicy()
	policy.ExcludeFolders = cleanPolicyList(policy.ExcludeFolders, defaults.ExcludeFolders)
	policy.ExcludePatterns = cleanPolicyList(policy.ExcludePatterns, defaults.ExcludePatterns)
	policy.IncludeExtensions = cleanExtensionList(policy.IncludeExtensions, defaults.IncludeExtensions)
	policy.ExcludeExtensions = cleanExtensionList(policy.ExcludeExtensions, defaults.ExcludeExtensions)
	return policy
}

func newFileSearchFilter(policy FileSearchPolicy) fileSearchFilter {
	policy = NormalizeFileSearchPolicy(policy)
	filter := fileSearchFilter{
		folders: make([]string, 0, len(policy.ExcludeFolders)),
	}
	seenFolders := map[string]bool{}
	for _, folder := range policy.ExcludeFolders {
		cleaned := filepath.Clean(expandPathVariables(strings.TrimSpace(folder)))
		if cleaned == "" || cleaned == "." {
			continue
		}
		key := strings.ToLower(cleaned)
		if seenFolders[key] {
			continue
		}
		seenFolders[key] = true
		filter.folders = append(filter.folders, key)
	}
	for _, pattern := range policy.ExcludePatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			filter.errors = append(filter.errors, pattern+": "+err.Error())
			continue
		}
		filter.patterns = append(filter.patterns, compiled)
	}
	filter.includeExtensions = append(filter.includeExtensions, policy.IncludeExtensions...)
	filter.excludeExtensions = append(filter.excludeExtensions, policy.ExcludeExtensions...)
	return filter
}

func expandPathVariables(value string) string {
	value = os.ExpandEnv(value)
	return regexp.MustCompile(`%([^%]+)%`).ReplaceAllStringFunc(value, func(match string) string {
		name := strings.Trim(match, "%")
		if name == "" {
			return match
		}
		if expanded := os.Getenv(name); expanded != "" {
			return expanded
		}
		return match
	})
}

func (filter fileSearchFilter) Excludes(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return false
	}
	if pathHasListedExtension(path, filter.includeExtensions) {
		return false
	}
	lowerPath := strings.ToLower(path)
	for _, folder := range filter.folders {
		prefix := folder
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if lowerPath == folder || strings.HasPrefix(lowerPath, prefix) {
			return true
		}
	}
	for _, pattern := range filter.patterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	if pathHasListedExtension(path, filter.excludeExtensions) {
		return true
	}
	return false
}

func (filter fileSearchFilter) Errors() []string {
	return append([]string(nil), filter.errors...)
}

func defaultFileExcludeFolders() []string {
	return cleanPolicyList([]string{
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

func defaultFileExcludePatterns() []string {
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

func defaultFileExcludeExtensions() []string {
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
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
	}
	return base
}

func defaultLocalAppDataFolder() string {
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	return base
}

func defaultLocalLowAppDataFolder() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
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

func cleanPolicyList(values []string, defaults []string) []string {
	if len(values) == 0 {
		values = defaults
	}
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func cleanExtensionList(values []string, defaults []string) []string {
	if len(values) == 0 {
		values = defaults
	}
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		item := normalizeExtension(value)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		cleaned = append(cleaned, item)
	}
	return cleaned
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

func pathHasListedExtension(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(path))
	for _, extension := range extensions {
		if len(base) > len(extension) && strings.HasSuffix(base, extension) {
			return true
		}
	}
	return false
}
