package filesearch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FileSearchPolicy struct {
	ExcludeFolders  []string `json:"excludeFolders"`
	ExcludePatterns []string `json:"excludePatterns"`
}

type fileSearchFilter struct {
	folders  []string
	patterns []*regexp.Regexp
	errors   []string
}

func DefaultFileSearchPolicy() FileSearchPolicy {
	return FileSearchPolicy{
		ExcludeFolders: []string{defaultRecentFolder()},
	}
}

func NormalizeFileSearchPolicy(policy FileSearchPolicy) FileSearchPolicy {
	defaults := DefaultFileSearchPolicy()
	policy.ExcludeFolders = cleanPolicyList(policy.ExcludeFolders, defaults.ExcludeFolders)
	policy.ExcludePatterns = cleanPolicyList(policy.ExcludePatterns, nil)
	return policy
}

func newFileSearchFilter(policy FileSearchPolicy) fileSearchFilter {
	policy = NormalizeFileSearchPolicy(policy)
	filter := fileSearchFilter{
		folders: make([]string, 0, len(policy.ExcludeFolders)),
	}
	for _, folder := range policy.ExcludeFolders {
		cleaned := filepath.Clean(expandPathVariables(strings.TrimSpace(folder)))
		if cleaned == "" || cleaned == "." {
			continue
		}
		filter.folders = append(filter.folders, strings.ToLower(cleaned))
	}
	for _, pattern := range policy.ExcludePatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			filter.errors = append(filter.errors, pattern+": "+err.Error())
			continue
		}
		filter.patterns = append(filter.patterns, compiled)
	}
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
	return false
}

func (filter fileSearchFilter) Errors() []string {
	return append([]string(nil), filter.errors...)
}

func defaultRecentFolder() string {
	base := strings.TrimSpace(os.Getenv("APPDATA"))
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
	}
	if base == "" {
		return filepath.Join("Microsoft", "Windows", "Recent")
	}
	return filepath.Join(base, "Microsoft", "Windows", "Recent")
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
