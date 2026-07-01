package setupstub

import (
	"archive/zip"
	"bytes"
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

func TestParseCommandOptionsAcceptsInstallerChoices(t *testing.T) {
	command, err := parseCommandOptions([]string{
		"-InstallDir", `C:\Apps\Ariadne`,
		"--no-start-menu-shortcut",
		"--no-desktop-shortcut",
		"--autostart",
		"--launch-after-install",
		"--no-file-search-service",
	})
	if err != nil {
		t.Fatalf("parse command options: %v", err)
	}
	if command.InstallDir != `C:\Apps\Ariadne` {
		t.Fatalf("install dir = %q", command.InstallDir)
	}
	if command.CreateStartMenuShortcut {
		t.Fatal("start menu shortcut should be disabled")
	}
	if command.CreateDesktopShortcut {
		t.Fatal("desktop shortcut should be disabled")
	}
	if !command.AutoStart {
		t.Fatal("autostart should be enabled")
	}
	if !command.LaunchAfterInstall {
		t.Fatal("launch-after-install should be enabled")
	}
	if command.InstallFileSearchService {
		t.Fatal("file search service should be disabled")
	}
}

func TestParseCommandOptionsAcceptsFileSearchServiceCommand(t *testing.T) {
	command, err := parseCommandOptions([]string{
		"--file-search-service-command", "install",
		"--file-search-service-exe", `C:\Apps\Ariadne\ariadne.exe`,
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

func TestExtractPayloadFindsPackageRootWithoutScripts(t *testing.T) {
	payload := zipPayload(t, map[string]string{
		"ariadne-dev-windows-x64/app/ariadne.exe": "fake exe",
		"ariadne-dev-windows-x64/app/logo.ico":    "fake icon",
		"ariadne-dev-windows-x64/manifest.json":   "{}",
	})
	root, err := ExtractPayload(payload, t.TempDir())
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
	_, err := ExtractPayload(payload, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsafe zip entry") {
		t.Fatalf("expected unsafe zip entry error, got %v", err)
	}
}

func TestRunInstallsAndUninstallsPayloadWithoutScripts(t *testing.T) {
	base := t.TempDir()
	installDir := filepath.Join(base, "install")
	payload := zipPayload(t, map[string]string{
		"ariadne-dev-windows-x64/app/ariadne.exe": "fake exe",
		"ariadne-dev-windows-x64/app/logo.ico":    "fake icon",
		"ariadne-dev-windows-x64/manifest.json":   "{}",
	})

	result, err := Run(payload, Options{
		ProductName: "Ariadne",
		Version:     "0.1.0-test",
		Args:        []string{"-InstallDir", installDir, "-NoShortcuts", "-SkipProcessStop", "-SkipServiceChanges"},
	})
	if err != nil {
		t.Fatalf("run installer: %v", err)
	}
	if result.Action != ActionInstall {
		t.Fatalf("action = %s, want %s", result.Action, ActionInstall)
	}
	for _, path := range []string{
		filepath.Join(installDir, "ariadne.exe"),
		filepath.Join(installDir, "logo.ico"),
		filepath.Join(installDir, "AriadneSetup.exe"),
		filepath.Join(installDir, "install_receipt.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed file %s: %v", path, err)
		}
	}
	receipt := readFile(t, filepath.Join(installDir, "install_receipt.json"))
	if strings.Contains(strings.ToLower(receipt), "x-tools") {
		t.Fatalf("receipt should not mention x-tools: %s", receipt)
	}

	result, err = Run(nil, Options{
		ProductName: "Ariadne",
		Version:     "0.1.0-test",
		Args:        []string{"--uninstall", "--synchronous", "-InstallDir", installDir, "-NoShortcuts", "-SkipProcessStop", "-SkipServiceChanges"},
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
