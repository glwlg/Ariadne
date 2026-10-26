//go:build windows

package releasepack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Exercise the generated setup EXE, with no access to Ariadne's real shortcuts, services or data.
func TestPackagedSetupRepairsShortcutsAndPreservesData(t *testing.T) {
	base := t.TempDir()
	product := "AriadneReleaseCheck" + strconv.Itoa(os.Getpid())
	exe := filepath.Join(base, "bin", "ariadne.exe")
	writeFile(t, exe, "isolated test app")
	writeNativeCaptureHost(t, filepath.Dir(exe))
	root, err := findModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Build(Options{
		ProductName: product, Version: "0.0.0-releasecheck", ExePath: exe,
		IconPath: filepath.Join(root, "assets", "logo.ico"), OutputDir: filepath.Join(base, "release"),
	})
	if err != nil {
		t.Fatal(err)
	}
	installDir := filepath.Join(base, "install")
	startMenu := filepath.Join(base, "start-menu")
	desktop := filepath.Join(base, "desktop")
	appData := filepath.Join(base, "appdata")
	localData := filepath.Join(base, "local")
	sentinel := filepath.Join(appData, product, "user-data.json")
	writeFile(t, sentinel, `{"keep":"release acceptance"}`)
	arguments := []string{"--quiet", "-InstallDir", installDir, "-StartMenuDir", startMenu,
		"-DesktopDir", desktop, "-NoShortcuts", "-NoAutoStart", "-SkipProcessStop", "-SkipServiceChanges"}
	run := func(path string, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
		for _, item := range os.Environ() {
			name := strings.SplitN(item, "=", 2)[0]
			if !strings.EqualFold(name, "APPDATA") && !strings.EqualFold(name, "LOCALAPPDATA") {
				cmd.Env = append(cmd.Env, item)
			}
		}
		cmd.Env = append(cmd.Env, "APPDATA="+appData, "LOCALAPPDATA="+localData)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup command failed: %v\n%s", err, output)
		}
	}
	run(result.Manifest.SetupPath, arguments...)
	receiptPath := filepath.Join(installDir, "install_receipt.json")
	check := func() []string {
		t.Helper()
		var receipt struct {
			ExePath   string   `json:"exePath"`
			Shortcuts []string `json:"shortcuts"`
		}
		readJSON(t, receiptPath, &receipt)
		if len(receipt.Shortcuts) != 2 {
			t.Fatalf("both Start Menu entries are required: %+v", receipt)
		}
		expected := map[string]string{
			filepath.Join(startMenu, product+".lnk"):              filepath.Join(installDir, strings.ToLower(product)+".exe"),
			filepath.Join(startMenu, "Uninstall "+product+".lnk"): filepath.Join(installDir, product+"Setup.exe"),
		}
		for _, path := range receipt.Shortcuts {
			target, ok := expected[path]
			if !ok {
				t.Fatalf("shortcut escaped isolated Start Menu: %s", path)
			}
			cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command",
				`$ErrorActionPreference='Stop'; (New-Object -ComObject WScript.Shell).CreateShortcut($env:ARIADNE_TEST_SHORTCUT).TargetPath`)
			cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
			cmd.Env = append(os.Environ(), "ARIADNE_TEST_SHORTCUT="+path)
			output, err := cmd.CombinedOutput()
			if err != nil || !strings.EqualFold(strings.TrimSpace(string(output)), target) {
				t.Fatalf("shortcut %s must target %s, got %q: %v", path, target, output, err)
			}
		}
		if readFile(t, sentinel) != `{"keep":"release acceptance"}` {
			t.Fatal("setup changed user data")
		}
		if _, err := os.Stat(filepath.Join(desktop, product+".lnk")); !os.IsNotExist(err) {
			t.Fatalf("legacy no-shortcuts must only disable desktop: %v", err)
		}
		return receipt.Shortcuts
	}
	shortcuts := check()
	for _, path := range shortcuts {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	var legacy map[string]any
	readJSON(t, receiptPath, &legacy)
	legacy["shortcuts"] = []string{}
	legacy["installOptions"].(map[string]any)["createStartMenuShortcut"] = false
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	run(result.Manifest.SetupPath, arguments...)
	shortcuts = check()
	run(result.Manifest.SetupPath, append(arguments, "--uninstall", "--synchronous")...)
	for _, path := range append(shortcuts, installDir) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("uninstall left %s: %v", path, err)
		}
	}
	if readFile(t, sentinel) != `{"keep":"release acceptance"}` {
		t.Fatal("uninstall removed user data without consent")
	}
	t.Log(fmt.Sprintf("isolated setup install, legacy upgrade repair, shortcut targets and data preservation passed for %s", product))
}
