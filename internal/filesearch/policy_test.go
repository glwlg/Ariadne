package filesearch

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPolicyExcludesWindowsRecentFolder(t *testing.T) {
	policy := DefaultFileSearchPolicy()

	if !containsPathSuffix(policy.ExcludeFolders, "Microsoft/Windows/Recent") {
		t.Fatalf("default excluded folder should target Windows Recent, got %#v", policy.ExcludeFolders)
	}
	if !containsPathSuffix(policy.ExcludeFolders, "AppData/Local/Temp") && !containsPathSuffix(policy.ExcludeFolders, "Temp") {
		t.Fatalf("default excluded folders should include a temp directory, got %#v", policy.ExcludeFolders)
	}
	if !containsString(policy.ExcludePatterns, `(?i)\.(tmp|temp|part|crdownload|download)$`) {
		t.Fatalf("default excluded patterns should include temporary file extensions, got %#v", policy.ExcludePatterns)
	}
	if !containsString(policy.ExcludeExtensions, `.tmp`) || !containsString(policy.ExcludeExtensions, `.sqlite-wal`) {
		t.Fatalf("default excluded extensions should include runtime suffixes, got %#v", policy.ExcludeExtensions)
	}
	if !containsString(policy.ExcludePatterns, `(?i)[\\/]ebwebview([\\/]|$)`) {
		t.Fatalf("default excluded patterns should include WebView runtime state, got %#v", policy.ExcludePatterns)
	}
	if !containsString(policy.ExcludePatterns, `(?i)[\\/]users[\\/][^\\/]+[\\/]appdata[\\/](local|locallow|roaming)([\\/]|$)`) {
		t.Fatalf("default excluded patterns should include user AppData trees, got %#v", policy.ExcludePatterns)
	}
	if !containsString(policy.ExcludePatterns, `(?i)^[a-z]:[\\/](app|appdata|program files|program files \(x86\)|programdata|devapp)([\\/]|$)`) {
		t.Fatalf("default excluded patterns should include drive-level app/system data roots, got %#v", policy.ExcludePatterns)
	}
	if !containsString(policy.ExcludePatterns, `(?i)(^|[\\/])(\$mft|\$logfile|\$bitmap|\$boot|\$badclus|\$secure|\$upcase|\$volume|\$attrdef|pagefile\.sys|hiberfil\.sys|swapfile\.sys|dumpstack\.log\.tmp|thumbs\.db|desktop\.ini)$`) {
		t.Fatalf("default excluded patterns should include common system files, got %#v", policy.ExcludePatterns)
	}
	if !containsString(policy.ExcludePatterns, `(?i)(^|[\\/])(\.git|\.hg|\.svn|node_modules|\.pnpm-store|__pycache__|\.pytest_cache|\.ruff_cache|\.mypy_cache|\.gradle|\.idea|\.vscode|\.cache|\.codex|\.codex-audit|coverage|dist|build|target|out|bin|obj|\.next|\.nuxt|\.vite|\.turbo|\.parcel-cache|\.svelte-kit|\.angular|\.vercel)([\\/]|$)`) {
		t.Fatalf("default excluded patterns should include generated project directories, got %#v", policy.ExcludePatterns)
	}
}

func TestFileSearchFilterExcludesFolderChildrenAndRegex(t *testing.T) {
	root := t.TempDir()
	appData := filepath.Join(root, "AppData", "Roaming")
	t.Setenv("APPDATA", appData)
	filter := newFileSearchFilter(FileSearchPolicy{
		ExcludeFolders:  []string{filepath.Join("%APPDATA%", "Microsoft", "Windows", "Recent")},
		ExcludePatterns: []string{`\.tmp$`},
	})

	if !filter.Excludes(filepath.Join(appData, "Microsoft", "Windows", "Recent", "搜索测试.txt.lnk")) {
		t.Fatal("Recent child path should be excluded")
	}
	if !filter.Excludes(filepath.Join(root, "workspace", "scratch.tmp")) {
		t.Fatal("regex-matched path should be excluded")
	}
	if filter.Excludes(filepath.Join(root, "OneDrive", "桌面", "搜索测试.txt")) {
		t.Fatal("ordinary desktop file should not be excluded")
	}
}

func TestFileSearchFilterUsesExtensionAllowlistFirst(t *testing.T) {
	filter := newFileSearchFilter(FileSearchPolicy{
		ExcludeFolders:    []string{`C:\Users\luwei\AppData`},
		ExcludePatterns:   []string{`(?i)\.pdf$`},
		IncludeExtensions: []string{` PDF `, `*.md`},
		ExcludeExtensions: []string{`pdf`, `.md`, `.log`},
	})

	if filter.Excludes(`C:\Users\luwei\AppData\Local\Microsoft\Edge\User Data\Manual.PDF`) {
		t.Fatal("extension allowlist should override folder, regex, and extension denylist")
	}
	if filter.Excludes(`P:\workspace\notes\README.md`) {
		t.Fatal("extension allowlist should accept normalized wildcard extensions")
	}
	if !filter.Excludes(`P:\workspace\logs\fancun.log`) {
		t.Fatal("extension denylist should exclude files not on allowlist")
	}
}

func TestDefaultFileSearchFilterExcludesCommonRuntimeNoise(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "fancun-test-root")
	appData := filepath.Join(root, "Users", "luwei", "AppData", "Roaming")
	localAppData := filepath.Join(root, "Users", "luwei", "AppData", "Local")
	tempDir := filepath.Join(localAppData, "Temp")
	windir := filepath.Join(root, "Windows")
	programData := filepath.Join(root, "ProgramData")

	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("TEMP", tempDir)
	t.Setenv("TMP", tempDir)
	t.Setenv("WINDIR", windir)
	t.Setenv("ProgramData", programData)
	filter := newFileSearchFilter(DefaultFileSearchPolicy())

	excluded := []string{
		filepath.Join(localAppData, "D3DSCache", "4921e6f431b0944a", "F4EB2D6C.dxcache-shm"),
		filepath.Join(tempDir, "edge-local-state.tmp"),
		filepath.Join(appData, "fancun.exe", "EBWebView", "Default", "Code Cache", "js", "the-real-index"),
		filepath.Join(appData, "Fancun", "capture_images", "capture-20260706.png"),
		filepath.Join(windir, "System32", "ntdll.dll"),
		filepath.Join(programData, "cache", "state.bin"),
		filepath.Join(root, "pagefile.sys"),
		filepath.Join(root, "workspace", "Fancun", ".venv", "Lib", "site-packages", "pkg", "__init__.py"),
		filepath.Join(root, "workspace", "Fancun", ".git", "refs", "heads", "main"),
		filepath.Join(root, "workspace", "Fancun", "frontend", "node_modules", ".pnpm", "vue", "index.js"),
	}
	for _, path := range excluded {
		if !filter.Excludes(path) {
			t.Fatalf("default filter should exclude runtime noise path %q", path)
		}
	}

	if filter.Excludes(filepath.Join(root, "workspace", "Fancun", "internal", "filesearch", "policy.go")) {
		t.Fatal("default filter should not exclude ordinary source files")
	}
}

func containsPathSuffix(values []string, suffix string) bool {
	for _, value := range values {
		if strings.HasSuffix(filepath.ToSlash(value), suffix) {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
