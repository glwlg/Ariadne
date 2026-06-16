package releasepack

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWindowsResourceConfigUsesAriadneBrand(t *testing.T) {
	configPath := filepath.Join("..", "..", "winres", "winres.json")
	var config map[string]interface{}
	readJSON(t, configPath, &config)

	version := config["RT_VERSION"].(map[string]interface{})["#1"].(map[string]interface{})["0000"].(map[string]interface{})
	info := version["info"].(map[string]interface{})["0409"].(map[string]interface{})
	if info["ProductName"] != "Ariadne" || info["OriginalFilename"] != "ariadne.exe" || info["InternalName"] != "ariadne" {
		t.Fatalf("Windows version resource should use Ariadne identity, got %#v", info)
	}
	if strings.Contains(strings.ToLower(strings.Join([]string{
		stringValue(info["ProductName"]),
		stringValue(info["FileDescription"]),
		stringValue(info["InternalName"]),
		stringValue(info["OriginalFilename"]),
	}, " ")), "x-tools") {
		t.Fatalf("Windows version resource should not expose legacy x-tools identity: %#v", info)
	}

	manifest := config["RT_MANIFEST"].(map[string]interface{})["#1"].(map[string]interface{})["0409"].(map[string]interface{})
	if manifest["description"] != "Ariadne command launcher and work memory center" || manifest["long-path-aware"] != true {
		t.Fatalf("Windows manifest should expose Ariadne description and long-path support, got %#v", manifest)
	}

	icon := config["RT_GROUP_ICON"].(map[string]interface{})["APP"].(map[string]interface{})["0000"].([]interface{})
	if len(icon) != 1 || icon[0] != "../assets/logo.png" {
		t.Fatalf("Windows resources should embed generated Ariadne logo PNG, got %#v", icon)
	}
}

func TestWindowsResourceTaskEmitsGoLinkableSyso(t *testing.T) {
	taskfile := readFile(t, filepath.Join("..", "..", "Taskfile.yml"))
	assertContains(t, taskfile, "windows:resources")
	assertContains(t, taskfile, "--out ariadne_resource.syso --no-suffix")
}

func TestBuildCreatesReleasePackageWithInstallerScripts(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "ariadne.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")

	result, err := Build(Options{
		Version:   "0.1.0-test",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	manifest := result.Manifest
	if manifest.ProductName != "Ariadne" || manifest.LegacyName != "x-tools" || manifest.Version != "0.1.0-test" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.ZipPath == "" || manifest.PackageDir == "" || len(manifest.Files) != 2 {
		t.Fatalf("manifest should describe package paths and files: %#v", manifest)
	}
	assertContains(t, strings.Join(manifest.CoexistenceNotes, "\n"), "does not remove legacy")
	assertContains(t, strings.Join(manifest.CoexistenceNotes, "\n"), "retry Alt+Q")
	assertContains(t, strings.Join(manifest.RollbackNotes, "\n"), "Ariadne.previous")
	assertContains(t, strings.Join(manifest.RollbackNotes, "\n"), "pre_restore")

	installScript := readFile(t, filepath.Join(manifest.PackageDir, "scripts", "install.ps1"))
	for _, text := range []string{"NoShortcuts", "SkipProcessStop", "StartMenuDir", "DesktopDir", "install_receipt.json", "Programs\\Ariadne", "x-tools", "Ariadne.lnk", "Uninstall Ariadne.lnk", "Ariadne.previous"} {
		assertContains(t, installScript, text)
	}
	uninstallScript := readFile(t, filepath.Join(manifest.PackageDir, "scripts", "uninstall.ps1"))
	for _, text := range []string{"RemoveUserData", "NoShortcuts", "SkipProcessStop", "Synchronous", "StartMenuDir", "DesktopDir", "install_receipt.json", "Ariadne.lnk", "Uninstall Ariadne.lnk", "CurrentVersion\\Run"} {
		assertContains(t, uninstallScript, text)
	}
	readme := readFile(t, filepath.Join(manifest.PackageDir, "README.txt"))
	assertContains(t, readme, "confirmed handoff")
	assertContains(t, readme, "pre_restore")
	assertContains(t, readme, "Temporary install smoke")

	var manifestFile Manifest
	readJSON(t, filepath.Join(manifest.PackageDir, "manifest.json"), &manifestFile)
	if manifestFile.Files[0].SHA256 == "" || manifestFile.Files[0].Bytes <= 0 {
		t.Fatalf("manifest file should include hashes and sizes: %#v", manifestFile.Files)
	}

	entries := zipEntries(t, manifest.ZipPath)
	packageRoot := filepath.Base(manifest.PackageDir)
	for _, name := range []string{
		packageRoot + "/app/ariadne.exe",
		packageRoot + "/app/logo.ico",
		packageRoot + "/scripts/install.ps1",
		packageRoot + "/scripts/uninstall.ps1",
		packageRoot + "/manifest.json",
		packageRoot + "/README.txt",
	} {
		if !entries[name] {
			t.Fatalf("missing zip entry %s; entries=%#v", name, entries)
		}
	}
}

func TestInstallAndUninstallScriptsSupportTempDirectorySmoke(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell install smoke only runs on Windows")
	}
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		t.Skip("powershell.exe not available")
	}

	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "ariadne.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	installDir := filepath.Join(base, "install target")
	startMenuDir := filepath.Join(base, "start menu")
	desktopDir := filepath.Join(base, "desktop")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")

	result, err := Build(Options{
		Version:   "0.1.0-smoke",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	installScript := filepath.Join(result.Manifest.PackageDir, "scripts", "install.ps1")
	runPowerShell(t, installScript, "-InstallDir", installDir, "-StartMenuDir", startMenuDir, "-DesktopDir", desktopDir, "-CreateDesktopShortcut", "-SkipLegacyCheck", "-SkipProcessStop")
	if _, err := os.Stat(filepath.Join(installDir, "ariadne.exe")); err != nil {
		t.Fatalf("installed exe missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "uninstall.ps1")); err != nil {
		t.Fatalf("installed uninstall script missing: %v", err)
	}
	for _, path := range []string{
		filepath.Join(startMenuDir, "Ariadne.lnk"),
		filepath.Join(startMenuDir, "Uninstall Ariadne.lnk"),
		filepath.Join(desktopDir, "Ariadne.lnk"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("shortcut missing after install %s: %v", path, err)
		}
	}
	receipt := readFile(t, filepath.Join(installDir, "install_receipt.json"))
	assertContains(t, receipt, "0.1.0-smoke")
	assertContains(t, receipt, "Ariadne.lnk")

	runPowerShell(t, installScript, "-InstallDir", installDir, "-StartMenuDir", startMenuDir, "-DesktopDir", desktopDir, "-CreateDesktopShortcut", "-SkipLegacyCheck", "-SkipProcessStop")
	previous, err := filepath.Glob(filepath.Join(base, "Ariadne.previous-*"))
	if err != nil {
		t.Fatalf("glob previous installs: %v", err)
	}
	if len(previous) != 1 {
		t.Fatalf("expected one previous install directory, got %#v", previous)
	}

	uninstallScript := filepath.Join(result.Manifest.PackageDir, "scripts", "uninstall.ps1")
	runPowerShell(t, uninstallScript, "-InstallDir", installDir, "-SkipProcessStop", "-Synchronous")
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir should be removed, stat err=%v", err)
	}
	for _, path := range []string{
		filepath.Join(startMenuDir, "Ariadne.lnk"),
		filepath.Join(startMenuDir, "Uninstall Ariadne.lnk"),
		filepath.Join(desktopDir, "Ariadne.lnk"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("shortcut should be removed after uninstall %s, stat err=%v", path, err)
		}
	}
}

func TestBuildRejectsMissingExecutable(t *testing.T) {
	base := t.TempDir()
	iconPath := filepath.Join(base, "assets", "logo.ico")
	writeFile(t, iconPath, "fake icon")

	_, err := Build(Options{
		ExePath:   filepath.Join(base, "missing.exe"),
		IconPath:  iconPath,
		OutputDir: filepath.Join(base, "out"),
	})
	if err == nil || !strings.Contains(err.Error(), "executable not found") {
		t.Fatalf("expected missing executable error, got %v", err)
	}
}

func runPowerShell(t *testing.T, script string, args ...string) {
	t.Helper()
	commandArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}
	commandArgs = append(commandArgs, args...)
	cmd := exec.Command("powershell.exe", commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell %s failed: %v\n%s", script, err, output)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func readJSON(t *testing.T, path string, target interface{}) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected %q to contain %q", text, want)
	}
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func zipEntries(t *testing.T, path string) map[string]bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer reader.Close()

	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	return entries
}
