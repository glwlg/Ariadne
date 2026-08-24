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
	t.Setenv("APPDATA", `C:\Users\luwei\AppData\Roaming`)
	filter := newFileSearchFilter(FileSearchPolicy{
		ExcludeFolders:  []string{`%APPDATA%\Microsoft\Windows\Recent`},
		ExcludePatterns: []string{`\.tmp$`},
	})

	if !filter.Excludes(`C:\Users\luwei\AppData\Roaming\Microsoft\Windows\Recent\搜索测试.txt.lnk`) {
		t.Fatal("Recent child path should be excluded")
	}
	if !filter.Excludes(`P:\workspace\scratch.tmp`) {
		t.Fatal("regex-matched path should be excluded")
	}
	if filter.Excludes(`C:\Users\luwei\OneDrive\桌面\搜索测试.txt`) {
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
	if !filter.Excludes(`P:\workspace\logs\ariadne.log`) {
		t.Fatal("extension denylist should exclude files not on allowlist")
	}
}

func TestDefaultFileSearchFilterExcludesCommonRuntimeNoise(t *testing.T) {
	t.Setenv("APPDATA", `C:\Users\luwei\AppData\Roaming`)
	t.Setenv("LOCALAPPDATA", `C:\Users\luwei\AppData\Local`)
	t.Setenv("TEMP", `C:\Users\luwei\AppData\Local\Temp`)
	t.Setenv("TMP", `C:\Users\luwei\AppData\Local\Temp`)
	t.Setenv("WINDIR", `C:\Windows`)
	t.Setenv("ProgramData", `C:\ProgramData`)
	filter := newFileSearchFilter(DefaultFileSearchPolicy())

	excluded := []string{
		`C:\Users\luwei\AppData\Local\D3DSCache\4921e6f431b0944a\F4EB2D6C-ED2B-4BDD-AD9D-F913287E6768.dxcache-shm`,
		`C:\Users\luwei\AppData\Local\Temp\edge-local-state.tmp`,
		`C:\Users\luwei\AppData\Roaming\ariadne.exe\EBWebView\Default\Code Cache\js\index-dir\temp-index\the-real-index`,
		`C:\Users\luwei\AppData\Local\Google\Chrome\User Data\Default\Service Worker\CacheStorage\2fb1\index-dir\the-real-index`,
		`C:\Users\luwei\AppData\Roaming\Mozilla\Firefox\Profiles\profile.default-release\cache2\entries\A1B2C3`,
		`C:\Users\luwei\AppData\Local\Packages\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\LocalState\EBWebView\Default\Code Cache\js\index-dir\56a2206c.tmp`,
		`C:\Users\luwei\AppData\Roaming\Tencent\wechat\radium\web\profiles\default\Network\TransportSecurity\transportsecurity~rf18504f85.tmp`,
		`C:\Users\luwei\AppData\Roaming\utForpc\24772217\MjQ3NzIyMTc=\UTForPC.db-journal`,
		`C:\Users\luwei\AppData\Roaming\Tencent\xwechat\net\kvcomm\key_2961520020_4065598005_1_1783301876_7742_3600_ready.statistic`,
		`C:\Users\luwei\AppData\Local\Microsoft\OneDrive\logs\ListSync\Consumer_9cf4ddf2cc2d756a\NucleusLocal-2026-07-06.0001.57016.27.odlgz`,
		`C:\Users\luwei\AppData\Local\Microsoft\Edge Beta\User Data\Default\Network\TransportSecurity`,
		`C:\Users\luwei\AppData\Roaming\Ariadne\capture_images\capture-20260706-094138.970034400-535c8e18a2ac.png`,
		`C:\Users\luwei\.gemini\cache\state.json`,
		`C:\Users\luwei\.m2\repository\org\example\artifact\1.0.0\artifact-1.0.0.jar`,
		`C:\Windows\System32\ntdll.dll`,
		`C:\pagefile.sys`,
		`P:\App\Telegram Desktop\tdata\user_data`,
		`P:\AppData\wechat\xwechat_files\wxid_abc\FileStorage\Cache\temp.dat`,
		`P:\DevApp\Unity\Editor\Data\PlaybackEngines\AndroidPlayer\Tools\gradle\cache.bin`,
		`P:\Program Files\Tencent\WeChat\WeChatAppEx.exe`,
		`P:\workspace\env\npm\_cacache\content-v2\sha512\aa\bb`,
		`P:\workspace\glwlg\app\Ariadne\.venv\Lib\site-packages\pkg\__init__.py`,
		`P:\workspace\glwlg\app\Ariadne\.git\refs\codex\turn-diffs\captures\1783301386046\base`,
		`P:\workspace\glwlg\app\Ariadne\frontend\node_modules\.pnpm\vue\index.js`,
	}
	for _, path := range excluded {
		if !filter.Excludes(path) {
			t.Fatalf("default filter should exclude runtime noise path %q", path)
		}
	}

	if filter.Excludes(`P:\workspace\glwlg\app\Ariadne\internal\filesearch\policy.go`) {
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
