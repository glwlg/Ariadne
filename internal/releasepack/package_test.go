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

func TestBuildCreatesReleasePackageWithSetupPayload(t *testing.T) {
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
	if manifest.ProductName != "Ariadne" || manifest.Version != "0.1.0-test" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.ZipPath == "" || manifest.PackageDir == "" || len(manifest.Files) != 2 {
		t.Fatalf("manifest should describe package paths and files: %#v", manifest)
	}
	if manifest.SetupPath == "" || manifest.SetupFile == nil || manifest.SetupFile.Path != "AriadneSetup-0.1.0-test-windows-x64.exe" {
		t.Fatalf("manifest should describe setup exe: %#v", manifest)
	}
	if _, err := os.Stat(manifest.SetupPath); err != nil {
		t.Fatalf("setup exe missing: %v", err)
	}
	assertContains(t, strings.Join(manifest.RollbackNotes, "\n"), "Ariadne.previous")
	readme := readFile(t, filepath.Join(manifest.PackageDir, "README.txt"))
	assertContains(t, readme, "AriadneSetup-0.1.0-test-windows-x64.exe")
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
		packageRoot + "/app/ariadne.exe",
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

func TestSetupInstallerRequiresAdministratorAndEmbedsPayload(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "ariadne.exe")
	iconPath := filepath.Join(base, "assets", "logo.ico")
	outputDir := filepath.Join(base, "dist", "release")
	writeFile(t, exePath, "fake exe")
	writeFile(t, iconPath, "fake icon")

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
	config := setupWinresConfig(Options{ProductName: "Ariadne", Version: "0.1.0-setup-smoke"}, "setup-logo.png", "AriadneSetup.exe")
	assertContains(t, config, `"execution-level": "as invoker"`)
	assertContains(t, strings.Join(result.Manifest.VerificationNotes, "\n"), "administrator permission only when installing the search service")
	assertContains(t, strings.Join(result.Manifest.VerificationNotes, "\n"), "AriadneFileSearch")
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
	exePath := filepath.Join(base, "bin", "ariadne.exe")
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
	if err != nil {
		t.Fatalf("build should not require Everything runtime DLL: %v", err)
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
