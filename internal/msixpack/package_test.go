package msixpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildCreatesUnsignedMSIXLayout(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "ariadne.exe")
	logoPath := filepath.Join(base, "assets", "logo.png")
	outputDir := filepath.Join(base, "dist", "msix")
	writeFile(t, exePath, "fake exe")
	writeFile(t, logoPath, "fake png")

	result, err := Build(Options{
		Version:   "1.2",
		ExePath:   exePath,
		LogoPath:  logoPath,
		OutputDir: outputDir,
		CreatedAt: time.Unix(1710000000, 0),
	})
	if err != nil {
		t.Fatalf("build MSIX layout: %v", err)
	}

	manifest := result.Manifest
	if manifest.ProductName != "Ariadne" || manifest.PackageName != "Ariadne.CommandLauncher" || manifest.Publisher != "CN=Ariadne" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if manifest.Version != "1.2.0.0" || manifest.Platform != "windows" || manifest.Arch != runtime.GOARCH {
		t.Fatalf("unexpected manifest platform/version: %#v", manifest)
	}
	if manifest.Packed || manifest.MsixPath != "" || manifest.MsixFile != nil {
		t.Fatalf("unsigned layout should not claim a packed .msix exists: %#v", manifest)
	}
	if manifest.CandidateMsixPath == "" || !strings.HasSuffix(strings.ToLower(manifest.CandidateMsixPath), ".msix") {
		t.Fatalf("manifest should expose candidate .msix output path: %#v", manifest)
	}
	for _, path := range []string{
		filepath.Join(manifest.LayoutDir, "Ariadne.exe"),
		filepath.Join(manifest.LayoutDir, "Assets", "Square44x44Logo.png"),
		filepath.Join(manifest.LayoutDir, "Assets", "Square150x150Logo.png"),
		filepath.Join(manifest.LayoutDir, "Assets", "StoreLogo.png"),
		filepath.Join(manifest.LayoutDir, "AppxManifest.xml"),
		filepath.Join(manifest.LayoutDir, "README-msix.txt"),
		filepath.Join(manifest.LayoutDir, "msix-manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated MSIX layout file %s: %v", path, err)
		}
	}
	if len(manifest.Files) != 6 {
		t.Fatalf("manifest should hash the app payload, logos, AppxManifest, and README; got %#v", manifest.Files)
	}
	for _, file := range manifest.Files {
		if file.Path == "" || file.Bytes <= 0 || file.SHA256 == "" {
			t.Fatalf("manifest file should include relative path, bytes, and hash: %#v", file)
		}
	}

	appxManifest := readFile(t, manifest.AppxManifestPath)
	for _, want := range []string{
		`Identity Name="Ariadne.CommandLauncher"`,
		`Publisher="CN=Ariadne"`,
		`Version="1.2.0.0"`,
		`Executable="Ariadne.exe"`,
		`EntryPoint="Windows.FullTrustApplication"`,
		`runFullTrust`,
		`Square44x44Logo="Assets\Square44x44Logo.png"`,
		`Square150x150Logo="Assets\Square150x150Logo.png"`,
	} {
		assertContains(t, appxManifest, want)
	}
	if strings.Contains(strings.ToLower(appxManifest), "x-tools") {
		t.Fatalf("MSIX manifest should not expose legacy x-tools identity:\n%s", appxManifest)
	}

	var manifestFile Manifest
	readJSON(t, filepath.Join(manifest.LayoutDir, "msix-manifest.json"), &manifestFile)
	if manifestFile.Packed || manifestFile.MsixPath != "" || manifestFile.CandidateMsixPath == "" {
		t.Fatalf("written manifest should match unsigned layout status: %#v", manifestFile)
	}
	assertContains(t, strings.Join(manifestFile.SigningNotes, "\n"), "signtool.exe")
	assertContains(t, readFile(t, filepath.Join(manifest.LayoutDir, "README-msix.txt")), "does not include work memory exports")
}

func TestBuildRejectsMissingExecutable(t *testing.T) {
	base := t.TempDir()
	logoPath := filepath.Join(base, "assets", "logo.png")
	writeFile(t, logoPath, "fake png")

	_, err := Build(Options{
		ExePath:   filepath.Join(base, "missing.exe"),
		LogoPath:  logoPath,
		OutputDir: filepath.Join(base, "out"),
	})
	if err == nil || !strings.Contains(err.Error(), "executable not found") {
		t.Fatalf("expected missing executable error, got %v", err)
	}
}

func TestBuildRejectsPackWhenMakeAppxIsUnavailable(t *testing.T) {
	base := t.TempDir()
	exePath := filepath.Join(base, "bin", "ariadne.exe")
	logoPath := filepath.Join(base, "assets", "logo.png")
	writeFile(t, exePath, "fake exe")
	writeFile(t, logoPath, "fake png")

	_, err := Build(Options{
		ExePath:      exePath,
		LogoPath:     logoPath,
		OutputDir:    filepath.Join(base, "out"),
		Pack:         true,
		MakeAppxPath: filepath.Join(base, "missing-makeappx.exe"),
	})
	if err == nil || !strings.Contains(err.Error(), "makeappx not found") {
		t.Fatalf("expected missing makeappx error, got %v", err)
	}
}

func TestTaskfileExposesMSIXPackagingTasks(t *testing.T) {
	taskfile := readFile(t, filepath.Join("..", "..", "Taskfile.yml"))
	for _, want := range []string{
		"windows:msix:",
		"cmd/msixpack",
		"windows:msix-pack:",
		"-pack",
	} {
		assertContains(t, taskfile, want)
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
		t.Fatalf("expected text to contain %q\n%s", want, text)
	}
}
