package releasepack

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsResourceConfigUsesFancunBrand(t *testing.T) {
	configPath := filepath.Join("..", "..", "winres", "winres.json")
	var config map[string]interface{}
	readJSON(t, configPath, &config)

	version := config["RT_VERSION"].(map[string]interface{})["#1"].(map[string]interface{})["0000"].(map[string]interface{})
	info := version["info"].(map[string]interface{})["0409"].(map[string]interface{})
	if info["ProductName"] != "Fancun" || info["OriginalFilename"] != "fancun.exe" || info["InternalName"] != "fancun" {
		t.Fatalf("Windows version resource should use Fancun identity, got %#v", info)
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
	if manifest["description"] != "Fancun desktop productivity tools" || manifest["long-path-aware"] != true {
		t.Fatalf("Windows manifest should expose Fancun description and long-path support, got %#v", manifest)
	}

	icon := config["RT_GROUP_ICON"].(map[string]interface{})["APP"].(map[string]interface{})["0000"]
	if icon != "../assets/logo.ico" {
		t.Fatalf("Windows resources should embed the multi-resolution Fancun icon, got %#v", icon)
	}
}

func TestSetupUsesConfiguredIconInsteadOfOriginalLogo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "assets", "logo.png"), "oversized source artwork")
	iconPath := filepath.Join("assets", "custom.ico")
	if got := setupIconSource(Options{IconPath: iconPath}, root); got != filepath.Join(root, iconPath) {
		t.Fatalf("setup must use the configured icon, got %s", got)
	}
	absolute := filepath.Join(root, "absolute.ico")
	if got := setupIconSource(Options{IconPath: absolute}, root); got != absolute {
		t.Fatalf("setup must preserve the absolute icon path, got %s", got)
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(setupWinresConfig(Options{ProductName: "Fancun", Version: "dev"}, "setup-logo.ico", "FancunSetup.exe")), &config); err != nil {
		t.Fatal(err)
	}
	icon := config["RT_GROUP_ICON"].(map[string]interface{})["APP"].(map[string]interface{})["0000"]
	if icon != "setup-logo.ico" {
		t.Fatalf("setup must load the ICO as a complete icon group, got %#v", icon)
	}
}

func TestWindowsResourceTaskEmitsGoLinkableSyso(t *testing.T) {
	taskfile := readFile(t, filepath.Join("..", "..", "Taskfile.yml"))
	assertContains(t, taskfile, "windows:resources")
	assertContains(t, taskfile, "--out fancun_resource.syso --no-suffix")
}

func TestWindowsBuildInjectsReleaseVersionIntoApplication(t *testing.T) {
	taskfile := readFile(t, filepath.Join("..", "..", "Taskfile.yml"))
	assertContains(t, taskfile, "APP_VERSION: '{{.FANCUN_VERSION | default \"dev\"}}'")
	assertContains(t, taskfile, "-X main.appVersion={{.APP_VERSION}}")
}

func TestSetupStubPassesUpdateDirectoryToInteractiveInstaller(t *testing.T) {
	source := setupMainSource(Options{ProductName: "Fancun", Version: "1.2.3"})
	assertContains(t, source, "UpdateDir: parsed.UpdateDir")
}

func TestBuildCreatesReleasePackageWithSetupPayload(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, readFile(t, filepath.Join("..", "..", "assets", "logo.ico")))
	writeNativeCaptureHost(t, filepath.Dir(exePath))

	result, err := Build(Options{
		Version:   "0.1.0-test",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
		KeepZip:   true,
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	manifest := result.Manifest
	if manifest.ProductName != "Fancun" || manifest.Version != "0.1.0-test" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.ZipPath == "" || manifest.PackageDir == "" || len(manifest.Files) != 8 {
		t.Fatalf("manifest should describe package paths and files: %#v", manifest)
	}
	if manifest.SetupPath == "" || manifest.SetupFile == nil || manifest.SetupFile.Path != "FancunSetup-0.1.0-test-windows-x64.exe" {
		t.Fatalf("manifest should describe setup exe: %#v", manifest)
	}
	if _, err := os.Stat(manifest.SetupPath); err != nil {
		t.Fatalf("setup exe missing: %v", err)
	}
	assertContains(t, strings.Join(manifest.RollbackNotes, "\n"), "Fancun.previous")
	readme := readFile(t, filepath.Join(manifest.PackageDir, "README.txt"))
	assertContains(t, readme, "FancunSetup-0.1.0-test-windows-x64.exe")
	for _, forbidden := range []string{"install.ps1", "uninstall.ps1", "x-tools"} {
		if strings.Contains(strings.ToLower(readme), forbidden) {
			t.Fatalf("package README should not contain %q:\n%s", forbidden, readme)
		}
	}

	var manifestFile Manifest
	readJSON(t, filepath.Join(manifest.PackageDir, "manifest.json"), &manifestFile)
	if manifestFile.Files[0].SHA256 == "" || manifestFile.Files[0].Bytes <= 0 {
		t.Fatalf("manifest file should include hashes and sizes: %#v", manifestFile.Files)
	}
	manifestRaw := readFile(t, filepath.Join(manifest.PackageDir, "manifest.json"))
	for _, forbidden := range []string{"legacyName", "legacyInstallDir", "coexistenceNotes", "scripts", "x-tools"} {
		if strings.Contains(strings.ToLower(manifestRaw), strings.ToLower(forbidden)) {
			t.Fatalf("manifest should not contain %q:\n%s", forbidden, manifestRaw)
		}
	}
	if _, err := os.Stat(filepath.Join(manifest.PackageDir, "scripts")); !os.IsNotExist(err) {
		t.Fatalf("scripts directory should not be generated, stat err=%v", err)
	}

	entries := zipEntries(t, manifest.ZipPath)
	packageRoot := filepath.Base(manifest.PackageDir)
	for _, name := range []string{
		packageRoot + "/app/fancun.exe",
		packageRoot + "/app/native-capture/Fancun.CaptureHost.exe",
		packageRoot + "/app/uxplay/uxplay.exe",
		packageRoot + "/app/uxplay/lib/gstreamer-1.0/libgstd3d11.dll",
		packageRoot + "/app/uxplay/LICENSE-UxPlay",
		packageRoot + "/app/iphone-control/Fancun.iPhoneControl.exe",
		packageRoot + "/app/iphone-control/Vendor/BleHid/LICENSE",
		packageRoot + "/app/logo.ico",
		packageRoot + "/manifest.json",
		packageRoot + "/README.txt",
	} {
		if !entries[name] {
			t.Fatalf("missing zip entry %s; entries=%#v", name, entries)
		}
	}
	for _, name := range []string{
		packageRoot + "/scripts/install.ps1",
		packageRoot + "/scripts/uninstall.ps1",
	} {
		if entries[name] {
			t.Fatalf("zip should not contain %s", name)
		}
	}
}

func TestBuildRejectsMissingAirPlayRuntime(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "logo.ico")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")
	writeFile(t, filepath.Join(base, "bin", "native-capture", "Fancun.CaptureHost.exe"), "fake capture host")
	_, err := Build(Options{ExePath: exePath, IconPath: iconPath, OutputDir: filepath.Join(base, "release"), SkipSetup: true})
	if err == nil || !strings.Contains(err.Error(), "uxplay.exe") {
		t.Fatalf("release must include the receiver runtime: %v", err)
	}
}

func TestBuildRejectsMissingPhoneControl(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "logo.ico")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")
	writeNativeCaptureHost(t, filepath.Dir(exePath))
	if err := os.Remove(filepath.Join(base, "bin", "iphone-control", "Fancun.iPhoneControl.exe")); err != nil {
		t.Fatal(err)
	}
	_, err := Build(Options{ExePath: exePath, IconPath: iconPath, OutputDir: filepath.Join(base, "release"), SkipSetup: true})
	if err == nil || !strings.Contains(err.Error(), "Fancun.iPhoneControl.exe") {
		t.Fatalf("release must include phone control: %v", err)
	}
}

func TestBuildDefaultsToSetupOnlyAndWritesUpdaterChecksum(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, readFile(t, filepath.Join("..", "..", "assets", "logo.ico")))
	writeNativeCaptureHost(t, filepath.Dir(exePath))

	result, err := Build(Options{
		Version:   "1.2.3",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
	})
	if err != nil {
		t.Fatalf("build setup-only package: %v", err)
	}
	manifest := result.Manifest
	if manifest.ZipPath != "" {
		t.Fatalf("setup-only package should not expose a zip path: %#v", manifest)
	}
	zipPath := filepath.Join(outputDir, "fancun-1.2.3-windows-x64.zip")
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Fatalf("setup-only package should remove temporary zip, stat err=%v", err)
	}
	if manifest.SetupFile == nil || manifest.ChecksumPath == "" {
		t.Fatalf("setup-only package should expose setup checksum metadata: %#v", manifest)
	}
	wantChecksum := manifest.SetupFile.SHA256 + "  " + filepath.Base(manifest.SetupPath) + "\n"
	if got := readFile(t, manifest.ChecksumPath); got != wantChecksum {
		t.Fatalf("unexpected updater checksum file: got %q want %q", got, wantChecksum)
	}
}

func TestBuildRejectsNonSemanticReleaseVersion(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")
	writeNativeCaptureHost(t, filepath.Dir(exePath))

	_, err := Build(Options{
		Version:   "20260904-color-fix",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: filepath.Join(base, "out"),
	})
	if err == nil || !strings.Contains(err.Error(), "semantic version") {
		t.Fatalf("expected semantic version validation error, got %v", err)
	}
}

func TestSetupInstallerRequiresAdministratorAndEmbedsPayload(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, readFile(t, filepath.Join("..", "..", "assets", "logo.ico")))
	writeNativeCaptureHost(t, filepath.Dir(exePath))

	result, err := Build(Options{
		Version:   "0.1.0-setup-smoke",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
	})
	if err != nil {
		t.Fatalf("build package: %v", err)
	}

	if _, err := os.Stat(result.Manifest.SetupPath); err != nil {
		t.Fatalf("setup exe missing: %v", err)
	}
	config := setupWinresConfig(Options{ProductName: "Fancun", Version: "0.1.0-setup-smoke"}, "setup-logo.ico", "FancunSetup.exe")
	assertContains(t, config, `"execution-level": "as invoker"`)
	assertContains(t, strings.Join(result.Manifest.VerificationNotes, "\n"), "administrator permission only when installing the search service")
	assertContains(t, strings.Join(result.Manifest.VerificationNotes, "\n"), "FancunFileSearch")
	if result.Manifest.SetupFile == nil || result.Manifest.SetupFile.Bytes <= 0 || result.Manifest.SetupFile.SHA256 == "" {
		t.Fatalf("setup file should include size and hash: %#v", result.Manifest.SetupFile)
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

func TestBuildDoesNotRequireEverythingRuntimeDLL(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")
	writeNativeCaptureHost(t, filepath.Dir(exePath))

	_, err := Build(Options{
		Version:   "0.1.0-test",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: filepath.Join(base, "out"),
		SkipSetup: true,
	})
	if err != nil {
		t.Fatalf("build should not require Everything runtime DLL: %v", err)
	}
}

func TestBuildRejectsMissingNativeCaptureHost(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "fancun.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")

	_, err := Build(Options{
		Version:   "0.1.0-test",
		ExePath:   exePath,
		IconPath:  iconPath,
		OutputDir: filepath.Join(base, "out"),
		SkipSetup: true,
	})
	if err == nil || !strings.Contains(err.Error(), "Fancun.CaptureHost.exe") {
		t.Fatalf("expected missing native capture host error, got %v", err)
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

func writeNativeCaptureHost(t *testing.T, exeDir string) {
	t.Helper()
	writeFile(t, filepath.Join(exeDir, "native-capture", "Fancun.CaptureHost.exe"), "fake native capture host")
	writeFile(t, filepath.Join(exeDir, "uxplay", "uxplay.exe"), "fake airplay receiver")
	writeFile(t, filepath.Join(exeDir, "uxplay", "lib", "gstreamer-1.0", "libgstd3d11.dll"), "fake video sink")
	writeFile(t, filepath.Join(exeDir, "uxplay", "LICENSE-UxPlay"), "GPL-3.0")
	writeFile(t, filepath.Join(exeDir, "iphone-control", "Fancun.iPhoneControl.exe"), "fake phone control")
	writeFile(t, filepath.Join(exeDir, "iphone-control", "Vendor", "BleHid", "LICENSE"), "MIT")
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
