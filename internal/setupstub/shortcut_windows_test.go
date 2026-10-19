//go:build windows

package setupstub

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-ole/go-ole"
)

func TestCreateShortcutOnInitializedCOMThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !initializeInstallerCOM() {
		t.Fatal("initialize COM for test")
	}
	defer ole.CoUninitialize()

	dir := t.TempDir()
	for _, name := range []string{"Ariadne.lnk", "Uninstall Ariadne.lnk"} {
		shortcut := filepath.Join(dir, name)
		if err := createShortcut(filepath.Join(dir, "ariadne.exe"), shortcut, "", ""); err != nil {
			t.Fatalf("create %s on initialized COM thread: %v", name, err)
		}
		if _, err := os.Stat(shortcut); err != nil {
			t.Fatalf("shortcut missing: %v", err)
		}
	}
}
