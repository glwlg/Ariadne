package filesearch

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPolicyExcludesWindowsRecentFolder(t *testing.T) {
	policy := DefaultFileSearchPolicy()

	if len(policy.ExcludeFolders) != 1 {
		t.Fatalf("expected one default excluded folder, got %#v", policy.ExcludeFolders)
	}
	if !strings.HasSuffix(filepath.ToSlash(policy.ExcludeFolders[0]), "Microsoft/Windows/Recent") {
		t.Fatalf("default excluded folder should target Windows Recent, got %#v", policy.ExcludeFolders)
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
