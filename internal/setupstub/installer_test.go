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
	parsed := ParseArgs([]string{"--quiet", "-InstallDir", `C:\Temp\Fancun`, "/silent", "-NoShortcuts"})
	if !parsed.Quiet {
		t.Fatal("expected quiet mode")
	}
	if got := strings.Join(parsed.InstallArgs, " "); got != `-InstallDir C:\Temp\Fancun -NoShortcuts` {
		t.Fatalf("unexpected install args: %q", got)
	}
}

func TestParseArgsConsumesUpdateDirectory(t *testing.T) {
	parsed := ParseArgs([]string{"--update-from", `C:\Apps\Fancun`})
	if parsed.UpdateDir != `C:\Apps\Fancun` {
		t.Fatalf("update dir = %q", parsed.UpdateDir)
	}
	if len(parsed.InstallArgs) != 0 {
		t.Fatalf("update metadata must not force quiet install: %#v", parsed.InstallArgs)
	}
}

func TestParseCommandOptionsAcceptsInstallerChoices(t *testing.T) {
	command, err := parseCommandOptions([]string{
		"-InstallDir", `C:\Apps\Fancun`,
		"--no-desktop-shortcut",
		"--no-autostart",
		"--launch-after-install",
		"--no-file-search-service",
		"--legacy-roaming-source", `C:\Users\luwei\AppData\Roaming\Ariadne`,
	})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if command.InstallDir != `C:\Apps\Fancun` {
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
	if command.LegacyRoamingSource != `C:\Users\luwei\AppData\Roaming\Ariadne` {
		t.Fatalf("legacy roaming source = %q", command.LegacyRoamingSource)
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
		"--file-search-service-exe", `C:\Apps\Fancun\fancun.exe`,
		"--settings-config", `C:\Users\luwei\AppData\Roaming\Fancun\config.json`,
	})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if command.FileSearchServiceCommand != "install" {
		t.Fatalf("service command = %q", command.FileSearchServiceCommand)
	}
	if command.FileSearchServiceExePath != `C:\Apps\Fancun\fancun.exe` {
		t.Fatalf("service exe = %q", command.FileSearchServiceExePath)
	}
	if command.FileSearchSettingsConfig != `C:\Users\luwei\AppData\Roaming\Fancun\config.json` {
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
	output := "\r\nfancun.exe                 1234 Console                    1     42,000 K\r\n"
	if !processListContainsImage(output, "fancun.exe") {
		t.Fatal("tasklist parser should detect matching process image")
	}
	if processListContainsImage("INFO: No tasks are running which match the specified criteria.\r\n", "fancun.exe") {
		t.Fatal("tasklist parser should not report a process when tasklist found none")
	}
}

func TestProductProcessImageNamesIncludesNativeCaptureHost(t *testing.T) {
	names := productProcessImageNames("Fancun")
	if !contains(names, "fancun.exe") {
		t.Fatalf("product process list should include main exe: %#v", names)
	}
	if !contains(names, "Fancun.CaptureHost.exe") {
		t.Fatalf("product process list should include native capture host: %#v", names)
	}
	if !contains(names, "Fancun.iPhoneControl.exe") {
		t.Fatalf("product process list should include native iPhone control: %#v", names)
	}
}

func TestExtractPayloadFindsPackageRootWithoutScripts(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"fancun-dev-windows-x64/app/fancun.exe": "fake exe",
		"fancun-dev-windows-x64/app/logo.ico":   "fake icon",
		"fancun-dev-windows-x64/manifest.json":  "{}",
	})
	root, err := ExtractPayload(payload, t.TempDir(), "Fancun")
	if err != nil {
		t.Fatalf("extract payload: %v", err)
	}
	if filepath.Base(root) != "fancun-dev-windows-x64" {
		t.Fatalf("unexpected package root: %s", root)
	}
	if _, err := os.Stat(filepath.Join(root, "app", "fancun.exe")); err != nil {
		t.Fatalf("app exe missing after extraction: %v", err)
	}
}

func TestExtractPayloadRejectsZipSlip(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"fancun-dev-windows-x64/app/fancun.exe": "fake exe",
		"../escape.txt":                         "escape",
	})
	_, err := ExtractPayload(payload, t.TempDir(), "Fancun")
	if err == nil || !strings.Contains(err.Error(), "unsafe zip entry") {
		t.Fatalf("expected unsafe zip entry error, got %v", err)
	}
}

func TestExtractPayloadRequiresSelectedProductExecutable(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"release/app/fancun.exe": "production app",
	})
	_, err := ExtractPayload(payload, t.TempDir(), "FancunReleaseCheck")
	if err == nil || !strings.Contains(err.Error(), "app/fancunreleasecheck.exe") {
		t.Fatalf("isolated product must not use the real app executable: %v", err)
	}
}

func TestRunInstallsAndUninstallsPayloadWithoutScripts(t *testing.T) {
	base := t.TempDir()
	const productName = "FancunSetupTest"
	installDir := filepath.Join(base, "install")
	startMenuDir := filepath.Join(base, "start-menu")
	desktopDir := filepath.Join(base, "desktop")
	payload := zipPayload(t, map[string]string{
		"fancun-dev-windows-x64/app/fancun.exe":          "fake exe",
		"fancun-dev-windows-x64/app/fancunsetuptest.exe": "fake test exe",
		"fancun-dev-windows-x64/app/logo.ico":            "fake icon",
		"fancun-dev-windows-x64/manifest.json":           "{}",
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
		filepath.Join(installDir, "fancunsetuptest.exe"),
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
	if err := os.WriteFile(filepath.Join(appSource, "fancun.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	installDir := filepath.Join(t.TempDir(), "install")
	_, err := install(packageRoot, Options{ProductName: "Fancun"}, commandOptions{
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
		filepath.Join(defaultStartMenuDir(), "Fancun.lnk"),
		filepath.Join(defaultStartMenuDir(), "Uninstall Fancun.lnk"),
		filepath.Join(defaultDesktopDir(), "Fancun.lnk"),
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

func TestElevatedInstallResolvesCallerPathsAppliesAutostartAndLaunchesUnelevated(t *testing.T) {
	base := t.TempDir()
	userAppData := filepath.Join(base, "user-roaming")
	userLocalAppData := filepath.Join(base, "user-local")
	userProfile := filepath.Join(base, "user-profile")
	t.Setenv("APPDATA", userAppData)
	t.Setenv("LOCALAPPDATA", userLocalAppData)
	t.Setenv("USERPROFILE", userProfile)

	const productName = "FancunElevationTest"
	expectedInstallDir := filepath.Join(userLocalAppData, "Programs", productName)
	expectedStartMenu := filepath.Join(userAppData, "Microsoft", "Windows", "Start Menu", "Programs")
	expectedDesktop := filepath.Join(userProfile, "Desktop")
	expectedPictures := filepath.Join(userProfile, "Pictures")
	expectedSettings := filepath.Join(userAppData, productName, "config.json")
	expectedExe := filepath.Join(expectedInstallDir, strings.ToLower(productName)+".exe")
	legacySource := filepath.Join(userAppData, "Ariadne")

	payload := zipPayload(t, map[string]string{
		"fancun-dev-windows-x64/app/fancunelevationtest.exe": "fake exe",
		"fancun-dev-windows-x64/app/logo.ico":            "fake icon",
	})

	origElevated := processElevatedFn
	origElevate := elevateInstallFn
	origLaunch := launchAppFn
	origSetAutostart := setAutostartFn
	origCleanupAutostart := cleanupAutostartFn
	t.Cleanup(func() {
		processElevatedFn = origElevated
		elevateInstallFn = origElevate
		launchAppFn = origLaunch
		setAutostartFn = origSetAutostart
		cleanupAutostartFn = origCleanupAutostart
	})

	var elevatedCmd commandOptions
	var elevatedCalls int
	var autostartCalls []string
	var launched []string

	processElevatedFn = func() bool { return false }
	setAutostartFn = func(prod string, exe string) error {
		autostartCalls = append(autostartCalls, prod+"="+exe)
		return nil
	}
	cleanupAutostartFn = func(string) {}
	launchAppFn = func(exe string) error {
		launched = append(launched, exe)
		return nil
	}
	elevateInstallFn = func(cmd commandOptions) error {
		elevatedCalls++
		elevatedCmd = cmd
		// Simulate elevated child running under a DIFFERENT admin profile environment.
		adminBase := t.TempDir()
		t.Setenv("APPDATA", filepath.Join(adminBase, "admin-roaming"))
		t.Setenv("LOCALAPPDATA", filepath.Join(adminBase, "admin-local"))
		t.Setenv("USERPROFILE", filepath.Join(adminBase, "admin-profile"))

		// Elevated child must receive caller's resolved paths and must NOT run HKCU autostart or launch app.
		pkgRoot, err := ExtractPayload(payload, t.TempDir(), productName)
		if err != nil {
			return err
		}
		childCmd := cmd
		childCmd.SkipProcessStop = true
		childCmd.SkipServiceChanges = true
		_, err = install(pkgRoot, Options{ProductName: productName, Version: "0.1.0-test"}, childCmd)
		return err
	}

	result, err := Run(payload, Options{
		ProductName: productName,
		Version:     "0.1.0-test",
		Args: []string{
			"--launch-after-install",
			"--legacy-roaming-source", legacySource,
		},
	})
	if err != nil {
		t.Fatalf("elevated install failed: %v", err)
	}
	if elevatedCalls != 1 {
		t.Fatalf("expected 1 elevated call, got %d", elevatedCalls)
	}
	if result.Action != ActionInstall || result.InstallDir != expectedInstallDir {
		t.Fatalf("unexpected result: %#v (want InstallDir=%q)", result, expectedInstallDir)
	}
	if !elevatedCmd.ElevatedInstallRetry || elevatedCmd.LaunchAfterInstall {
		t.Fatalf("elevated helper must set ElevatedInstallRetry=true and LaunchAfterInstall=false: %#v", elevatedCmd)
	}
	if elevatedCmd.InstallDir != expectedInstallDir ||
		elevatedCmd.StartMenuDir != expectedStartMenu ||
		elevatedCmd.DesktopDir != expectedDesktop ||
		elevatedCmd.RoamingBase != userAppData ||
		elevatedCmd.LocalBase != userLocalAppData ||
		elevatedCmd.PicturesBase != expectedPictures ||
		elevatedCmd.FileSearchSettingsConfig != expectedSettings ||
		elevatedCmd.LegacyRoamingSource != legacySource {
		t.Fatalf("caller paths not resolved before elevation: %#v", elevatedCmd)
	}
	if _, err := os.Stat(filepath.Join(expectedStartMenu, productName+".lnk")); err != nil {
		t.Fatalf("Start Menu shortcut must be written to invoking user's directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(expectedDesktop, productName+".lnk")); err != nil {
		t.Fatalf("Desktop shortcut must be written to invoking user's directory: %v", err)
	}
	if len(autostartCalls) != 1 || autostartCalls[0] != productName+"="+expectedExe {
		t.Fatalf("HKCU autostart must run once in unelevated caller, got %#v", autostartCalls)
	}
	if len(launched) != 1 || launched[0] != expectedExe {
		t.Fatalf("app must be launched once unelevated in caller, got %#v", launched)
	}
}

func TestElevatedInstallChildMigratesToPassedUserBasesWithoutCredentialMarker(t *testing.T) {
	base := t.TempDir()
	userRoaming := filepath.Join(base, "user-roaming")
	userLocal := filepath.Join(base, "user-local")
	userPictures := filepath.Join(base, "user-pictures")
	adminRoaming := filepath.Join(base, "admin-roaming")
	t.Setenv("APPDATA", adminRoaming)
	t.Setenv("LOCALAPPDATA", filepath.Join(base, "admin-local"))
	t.Setenv("USERPROFILE", filepath.Join(base, "admin-profile"))

	if err := os.MkdirAll(filepath.Join(userRoaming, "Ariadne"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoaming, "Ariadne", "note.txt"), []byte("user-note"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotWithCreds []bool
	var gotCommand commandOptions
	origMigrate := migrateBrandFn
	origSetAutostart := setAutostartFn
	origLaunch := launchAppFn
	t.Cleanup(func() {
		migrateBrandFn = origMigrate
		setAutostartFn = origSetAutostart
		launchAppFn = origLaunch
	})
	setAutostartFn = func(string, string) error {
		t.Fatal("elevated child must not write HKCU autostart")
		return nil
	}
	launchAppFn = func(string) error {
		t.Fatal("elevated child must not launch app")
		return nil
	}
	migrateBrandFn = func(command commandOptions, withCredentials bool) error {
		gotCommand = command
		gotWithCreds = append(gotWithCreds, withCredentials)
		return origMigrate(command, withCredentials)
	}

	installDir := filepath.Join(base, "install")
	startMenuDir := filepath.Join(base, "start-menu")
	command, err := parseCommandOptions([]string{
		"--elevated-install-retry",
		"-InstallDir", installDir,
		"-StartMenuDir", startMenuDir,
		"--no-desktop-shortcut",
		"--roaming-base", userRoaming,
		"--local-base", userLocal,
		"--pictures-base", userPictures,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !command.ElevatedInstallRetry || command.RoamingBase != userRoaming || command.LocalBase != userLocal || command.PicturesBase != userPictures {
		t.Fatalf("unexpected parsed command: %#v", command)
	}
	// Verify migrateBrandFn receives withCredentials=false when ElevatedInstallRetry=true.
	if err := migrateBrandFn(command, !command.ElevatedInstallRetry); err != nil {
		t.Fatalf("migrateBrandFn failed: %v", err)
	}
	if len(gotWithCreds) != 1 || gotWithCreds[0] || gotCommand.RoamingBase != userRoaming {
		t.Fatalf("expected withCredentials=false and userRoaming=%q, got %#v %#v", userRoaming, gotWithCreds, gotCommand)
	}
	if raw, err := os.ReadFile(filepath.Join(userRoaming, "Fancun", "note.txt")); err != nil || string(raw) != "user-note" {
		t.Fatalf("migrated data missing from invoking user's roaming dir: %q err=%v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(adminRoaming, "Fancun")); !os.IsNotExist(err) {
		t.Fatalf("elevated admin profile must not receive migrated user data: %v", err)
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
