package setupstub

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsConsumesQuietFlags(t *testing.T) {
	parsed := ParseArgs([]string{"--quiet", "-InstallDir", `C:\Temp\Ariadne`, "/silent", "-NoShortcuts"})
	if !parsed.Quiet {
		t.Fatal("expected quiet mode")
	}
	if got := strings.Join(parsed.InstallArgs, " "); got != `-InstallDir C:\Temp\Ariadne -NoShortcuts` {
		t.Fatalf("unexpected install args: %q", got)
	}
}

func TestParseArgsConsumesUpdateDirectory(t *testing.T) {
	parsed := ParseArgs([]string{"--update-from", `C:\Apps\Ariadne`})
	if parsed.UpdateDir != `C:\Apps\Ariadne` {
		t.Fatalf("update dir = %q", parsed.UpdateDir)
	}
	if len(parsed.InstallArgs) != 0 {
		t.Fatalf("update metadata must not force quiet install: %#v", parsed.InstallArgs)
	}
}

func TestParseCommandOptionsAcceptsInstallerChoices(t *testing.T) {
	command, err := parseCommandOptions([]string{
		"-InstallDir", `C:\Apps\Ariadne`,
		"--no-desktop-shortcut",
		"--no-autostart",
		"--launch-after-install",
		"--no-file-search-service",
	})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if command.InstallDir != `C:\Apps\Ariadne` {
		t.Fatalf("install dir = %q", command.InstallDir)
	}
	if command.CreateDesktopShortcut {
		t.Fatal("desktop shortcut should be disabled")
	}
	if command.AutoStart {
		t.Fatal("autostart should be disabled")
	}
	if !command.LaunchAfterInstall {
		t.Fatal("launch-after-install should be enabled")
	}
	if command.InstallFileSearchService {
		t.Fatal("file search service should be disabled")
	}
}

func TestParseCommandOptionsLegacyNoShortcutsOnlyDisablesDesktop(t *testing.T) {
	for _, flag := range []string{"--no-shortcuts", "-NoShortcuts"} {
		command, err := parseCommandOptions([]string{flag})
		if err != nil {
			t.Fatal(err)
		}
		if command.CreateDesktopShortcut {
			t.Fatalf("%s should disable the optional desktop shortcut", flag)
		}
	}
}

func TestParseCommandOptionsRejectsDisablingStartMenu(t *testing.T) {
	for _, flag := range []string{
		"--no-start-menu-shortcut", "-NoStartMenuShortcut",
		"--create-start-menu-shortcut=false", "-CreateStartMenuShortcut=false",
	} {
		_, err := parseCommandOptions([]string{flag})
		if err == nil || !strings.Contains(err.Error(), "开始菜单入口为必需项") {
			t.Fatalf("%s must not allow an undiscoverable install: %v", flag, err)
		}
	}
}

func TestParseCommandOptionsDefaultsToAutostartAndFileSearchService(t *testing.T) {
	command, err := parseCommandOptions(nil)
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if !command.AutoStart {
		t.Fatal("autostart should be enabled by default")
	}
	if !command.InstallFileSearchService {
		t.Fatal("file search service should be enabled by default")
	}
}

func TestParseCommandOptionsAcceptsFileSearchServiceCommand(t *testing.T) {
	command, err := parseCommandOptions([]string{
		"--file-search-service-command", "install",
		"--file-search-service-exe", `C:\Apps\Ariadne\ariadne.exe`,
		"--settings-config", `C:\Users\luwei\AppData\Roaming\Ariadne\config.json`,
	})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if command.FileSearchServiceCommand != "install" {
		t.Fatalf("service command = %q", command.FileSearchServiceCommand)
	}
	if command.FileSearchServiceExePath != `C:\Apps\Ariadne\ariadne.exe` {
		t.Fatalf("service exe = %q", command.FileSearchServiceExePath)
	}
	if command.FileSearchSettingsConfig != `C:\Users\luwei\AppData\Roaming\Ariadne\config.json` {
		t.Fatalf("settings config = %q", command.FileSearchSettingsConfig)
	}
}

func TestParseCommandOptionsAcceptsFileSearchServiceInstallChoice(t *testing.T) {
	command, err := parseCommandOptions([]string{"--install-file-search-service"})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if !command.InstallFileSearchService {
		t.Fatal("file search service should be enabled by explicit install flag")
	}
}

func TestShouldRetryInstallElevatedForAccessDenied(t *testing.T) {
	if !shouldRetryInstallElevated(commandOptions{}, os.ErrPermission) {
		t.Fatal("access denied install replacement should request elevated retry")
	}
	if shouldRetryInstallElevated(commandOptions{ElevatedInstallRetry: true}, os.ErrPermission) {
		t.Fatal("elevated retry should not recurse")
	}
}

func TestShouldNotRetryInstallElevatedForFileInUse(t *testing.T) {
	err := errors.New("The process cannot access the file because it is being used by another process")
	if shouldRetryInstallElevated(commandOptions{}, err) {
		t.Fatal("file-in-use replacement should not request elevated retry")
	}
}

func TestProcessListContainsImage(t *testing.T) {
	output := "\r\nariadne.exe                 1234 Console                    1     42,000 K\r\n"
	if !processListContainsImage(output, "ariadne.exe") {
		t.Fatal("tasklist parser should detect matching process image")
	}
	if processListContainsImage("INFO: No tasks are running which match the specified criteria.\r\n", "ariadne.exe") {
		t.Fatal("tasklist parser should not report a process when tasklist found none")
	}
}

func TestProductProcessImageNamesIncludesNativeCaptureHost(t *testing.T) {
	names := productProcessImageNames("Ariadne")
	if !contains(names, "ariadne.exe") {
		t.Fatalf("product process list should include main exe: %#v", names)
	}
	if !contains(names, "Ariadne.CaptureHost.exe") {
		t.Fatalf("product process list should include native capture host: %#v", names)
	}
}

func TestExtractPayloadFindsPackageRootWithoutScripts(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"ariadne-dev-windows-x64/app/ariadne.exe": "fake exe",
		"ariadne-dev-windows-x64/app/logo.ico":    "fake icon",
		"ariadne-dev-windows-x64/manifest.json":   "{}",
	})
	root, err := ExtractPayload(payload, t.TempDir(), "Ariadne")
	if err != nil {
		t.Fatalf("extract payload: %v", err)
	}
	if filepath.Base(root) != "ariadne-dev-windows-x64" {
		t.Fatalf("unexpected package root: %s", root)
	}
	if _, err := os.Stat(filepath.Join(root, "app", "ariadne.exe")); err != nil {
		t.Fatalf("app exe missing after extraction: %v", err)
	}
}

func TestExtractPayloadRejectsZipSlip(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"ariadne-dev-windows-x64/app/ariadne.exe": "fake exe",
		"../escape.txt": "escape",
	})
	_, err := ExtractPayload(payload, t.TempDir(), "Ariadne")
	if err == nil || !strings.Contains(err.Error(), "unsafe zip entry") {
		t.Fatalf("expected unsafe zip entry error, got %v", err)
	}
}

func TestExtractPayloadRequiresSelectedProductExecutable(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"release/app/ariadne.exe": "production app",
	})
	_, err := ExtractPayload(payload, t.TempDir(), "AriadneReleaseCheck")
	if err == nil || !strings.Contains(err.Error(), "app/ariadnereleasecheck.exe") {
		t.Fatalf("isolated product must not use the real app executable: %v", err)
	}
}

func TestRunInstallsAndUninstallsPayloadWithoutScripts(t *testing.T) {
	base := t.TempDir()
	const productName = "AriadneSetupTest"
	installDir := filepath.Join(base, "install")
	startMenuDir := filepath.Join(base, "start-menu")
	desktopDir := filepath.Join(base, "desktop")
	payload := zipPayload(t, map[string]string{
		"ariadne-dev-windows-x64/app/ariadne.exe":          "fake exe",
		"ariadne-dev-windows-x64/app/ariadnesetuptest.exe": "fake test exe",
		"ariadne-dev-windows-x64/app/logo.ico":             "fake icon",
		"ariadne-dev-windows-x64/manifest.json":            "{}",
	})

	options := Options{
		ProductName: productName,
		Version:     "0.1.0-test",
		Args: []string{
			"-InstallDir", installDir, "-StartMenuDir", startMenuDir, "-DesktopDir", desktopDir,
			"-NoShortcuts", "-NoAutoStart", "-SkipProcessStop", "-SkipServiceChanges",
		},
	}
	result, err := Run(payload, options)
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if result.Action != ActionInstall {
		t.Fatalf("action = %s, want %s", result.Action, ActionInstall)
	}
	for _, path := range []string{
		filepath.Join(installDir, "ariadnesetuptest.exe"),
		filepath.Join(installDir, "logo.ico"),
		filepath.Join(installDir, productName+"Setup.exe"),
		filepath.Join(installDir, "install_receipt.json"),
		filepath.Join(startMenuDir, productName+".lnk"),
		filepath.Join(startMenuDir, "Uninstall "+productName+".lnk"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed file %s: %v", path, err)
		}
	}
	receipt := readFile(t, filepath.Join(installDir, "install_receipt.json"))
	if strings.Contains(strings.ToLower(receipt), "x-tools") {
		t.Fatalf("receipt should not mention x-tools: %s", receipt)
	}
	recorded := readReceipt(filepath.Join(installDir, "install_receipt.json"))
	if recorded.InstallOptions == nil || !recorded.InstallOptions.CreateStartMenuShortcut || recorded.InstallOptions.CreateDesktopShortcut {
		t.Fatalf("receipt must keep Start Menu enabled without creating a desktop link: %#v", recorded.InstallOptions)
	}
	if _, err := os.Stat(filepath.Join(desktopDir, productName+".lnk")); !os.IsNotExist(err) {
		t.Fatalf("desktop shortcut should remain optional: %v", err)
	}

	// Reproduce an older quiet install that recorded no shortcuts and left the app undiscoverable.
	for _, shortcut := range recorded.Shortcuts {
		if err := os.Remove(shortcut); err != nil {
			t.Fatal(err)
		}
	}
	recorded.Shortcuts = nil
	recorded.InstallOptions.CreateStartMenuShortcut = false
	if err := writeReceipt(filepath.Join(installDir, "install_receipt.json"), recorded); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(payload, options); err != nil {
		t.Fatalf("upgrade old no-shortcuts install: %v", err)
	}
	recorded = readReceipt(filepath.Join(installDir, "install_receipt.json"))
	if recorded.InstallOptions == nil || !recorded.InstallOptions.CreateStartMenuShortcut || len(recorded.Shortcuts) != 2 {
		t.Fatalf("upgrade must repair and record both Start Menu links: %#v", recorded)
	}
	for _, shortcut := range recorded.Shortcuts {
		if _, err := os.Stat(shortcut); err != nil {
			t.Fatalf("upgrade did not restore %s: %v", shortcut, err)
		}
	}

	result, err = Run(nil, Options{
		ProductName: productName,
		Version:     "0.1.0-test",
		Args: []string{
			"--uninstall", "--synchronous", "-InstallDir", installDir,
			"-StartMenuDir", startMenuDir, "-DesktopDir", desktopDir,
			"-SkipProcessStop", "-SkipServiceChanges",
		},
	})
	if err != nil {
		t.Fatalf("run uninstaller: %v", err)
	}
	if result.Action != ActionUninstall {
		t.Fatalf("action = %s, want %s", result.Action, ActionUninstall)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir should be removed, stat err=%v", err)
	}
	for _, shortcut := range recorded.Shortcuts {
		if _, err := os.Stat(shortcut); !os.IsNotExist(err) {
			t.Fatalf("uninstall should remove %s: %v", shortcut, err)
		}
	}
}

func TestInstallRequiresStartMenuDirectory(t *testing.T) {
	t.Setenv("APPDATA", "")
	packageRoot := t.TempDir()
	appSource := filepath.Join(packageRoot, "app")
	if err := os.Mkdir(appSource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appSource, "ariadne.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	installDir := filepath.Join(t.TempDir(), "install")
	_, err := install(packageRoot, Options{ProductName: "Ariadne"}, commandOptions{
		InstallDir: installDir, SkipProcessStop: true, SkipServiceChanges: true,
	})
	if err == nil || !strings.Contains(err.Error(), "无法确定开始菜单目录") {
		t.Fatalf("missing Start Menu directory must fail explicitly: %v", err)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("validation must happen before modifying the installation: %v", err)
	}
}

func TestRemoveShortcutsOnlyRemovesRecordedEntries(t *testing.T) {
	base := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(base, "appdata"))
	t.Setenv("USERPROFILE", filepath.Join(base, "user"))
	preserved := []string{
		filepath.Join(defaultStartMenuDir(), "Ariadne.lnk"),
		filepath.Join(defaultStartMenuDir(), "Uninstall Ariadne.lnk"),
		filepath.Join(defaultDesktopDir(), "Ariadne.lnk"),
	}
	recorded := filepath.Join(base, "test-install.lnk")
	for _, shortcut := range append(preserved, recorded) {
		if err := os.MkdirAll(filepath.Dir(shortcut), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(shortcut, []byte("shortcut"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	removeShortcuts(installReceipt{})
	removeShortcuts(installReceipt{Shortcuts: []string{recorded}})
	if _, err := os.Stat(recorded); !os.IsNotExist(err) {
		t.Fatalf("recorded shortcut should be removed: %v", err)
	}
	for _, shortcut := range preserved {
		if _, err := os.Stat(shortcut); err != nil {
			t.Fatalf("uninstall must not infer and delete another installation's shortcut %s: %v", shortcut, err)
		}
	}
}

func zipPayload(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		item, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := item.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
