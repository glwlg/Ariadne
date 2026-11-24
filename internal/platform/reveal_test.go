package platform

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

type fakeRevealFileInfo struct {
	name string
	dir  bool
}

func (f fakeRevealFileInfo) Name() string       { return f.name }
func (f fakeRevealFileInfo) Size() int64        { return 0 }
func (f fakeRevealFileInfo) Mode() fs.FileMode  { return 0o644 }
func (f fakeRevealFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeRevealFileInfo) IsDir() bool        { return f.dir }
func (f fakeRevealFileInfo) Sys() interface{}   { return nil }

func TestNormalizeRevealPathHandlesSpacesSlashesQuotesAndUNC(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: `  "C:/Program Files/My App/readme.txt"  `, want: `C:\Program Files\My App\readme.txt`},
		{raw: `d:\workspace\a, b\folder\`, want: `D:\workspace\a, b\folder`},
		{raw: `c:`, want: `C:\`},
		{raw: `c:\`, want: `C:\`},
		{raw: `//wsl.localhost/Ubuntu-26.04/home/luwei/file.go`, want: `\\wsl.localhost\Ubuntu-26.04\home\luwei\file.go`},
		{raw: `P:\foo\..\bar\doc.pdf`, want: `P:\bar\doc.pdf`},
	}
	for _, tc := range cases {
		if got := normalizeRevealPath(tc.raw); got != tc.want {
			t.Fatalf("normalizeRevealPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestResolveRevealTargetSelectsExistingOrFallsBackToExistingParent(t *testing.T) {
	existing := map[string]bool{
		`C:\Program Files\My App`:             true,
		`C:\Program Files\My App\a, b.txt`:    false,
		`C:\Users\luwei\OneDrive\项目`:          true,
		`C:\`:                                 true,
		`\\wsl.localhost\Ubuntu-26.04`:        true,
		`\\wsl.localhost\Ubuntu-26.04\file.c`: false,
	}
	statFn := func(p string) (os.FileInfo, error) {
		isDir, ok := existing[p]
		if !ok {
			return nil, os.ErrNotExist
		}
		return fakeRevealFileInfo{name: p, dir: isDir}, nil
	}

	target, err := resolveRevealTarget(`C:/Program Files/My App/a, b.txt`, statFn)
	if err != nil {
		t.Fatalf("expected existing file to resolve, got %v", err)
	}
	if target.Mode != revealModeSelect || target.Path != `C:\Program Files\My App\a, b.txt` {
		t.Fatalf("unexpected target: %+v", target)
	}
	if cmdLine := buildExplorerCmdLine(target); cmdLine != `explorer.exe /select,"C:\Program Files\My App\a, b.txt"` {
		t.Fatalf("unexpected explorer command line: %s", cmdLine)
	}

	missingLeaf, err := resolveRevealTarget(`C:\Users\luwei\OneDrive\项目\已删除目录\gone.pdf`, statFn)
	if err != nil {
		t.Fatalf("expected missing leaf to fall back to nearest existing parent, got %v", err)
	}
	if missingLeaf.Mode != revealModeOpenDir || missingLeaf.Path != `C:\Users\luwei\OneDrive\项目` {
		t.Fatalf("unexpected parent fallback target: %+v", missingLeaf)
	}
	if cmdLine := buildExplorerCmdLine(missingLeaf); cmdLine != `explorer.exe "C:\Users\luwei\OneDrive\项目"` {
		t.Fatalf("unexpected dir explorer command line: %s", cmdLine)
	}

	rootTarget, err := resolveRevealTarget(`C:\`, statFn)
	if err != nil || rootTarget.Mode != revealModeOpenDir || rootTarget.Path != `C:\` {
		t.Fatalf("expected drive root to open directly, got %+v err=%v", rootTarget, err)
	}

	uncFile, err := resolveRevealTarget(`\\wsl.localhost\Ubuntu-26.04\file.c`, statFn)
	if err != nil || uncFile.Mode != revealModeSelect || uncFile.Path != `\\wsl.localhost\Ubuntu-26.04\file.c` {
		t.Fatalf("expected UNC file to select, got %+v err=%v", uncFile, err)
	}

	if _, err := resolveRevealTarget(`Z:\no\such\drive\file.txt`, statFn); err == nil {
		t.Fatal("expected error when neither target nor any parent directory exists")
	}
}
