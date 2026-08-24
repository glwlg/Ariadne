//go:build windows

package filesearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedServiceExecutableLivesOutsideUserInstall(t *testing.T) {
	programFiles := t.TempDir()
	t.Setenv("ProgramFiles", programFiles)
	source := filepath.Join(t.TempDir(), "ariadne.exe")
	if err := os.WriteFile(source, []byte("service binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	target, err := installProtectedServiceExecutable(source)
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join(programFiles, "Ariadne", "Service", "Ariadne.Service.exe") {
		t.Fatalf("target = %q", target)
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != "service binary" {
		t.Fatalf("protected copy = %q, %v", raw, err)
	}
	if err := removeProtectedServiceExecutable(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(target)); !os.IsNotExist(err) {
		t.Fatalf("protected service directory should be removed: %v", err)
	}
}
