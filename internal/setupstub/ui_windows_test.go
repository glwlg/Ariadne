//go:build windows

package setupstub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractiveSelectionArgsEnableFileSearchService(t *testing.T) {
	args := interactiveInstallSelection{
		InstallDir:               `C:\Apps\Ariadne`,
		CreateStartMenuShortcut:  true,
		CreateDesktopShortcut:    true,
		InstallFileSearchService: true,
		AutoStart:                true,
	}.args()

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --install-file-search-service ") {
		t.Fatalf("checked file search service should be passed explicitly: %#v", args)
	}
	if strings.Contains(joined, " --no-file-search-service ") {
		t.Fatalf("checked file search service should not pass disable flag: %#v", args)
	}
	if !strings.Contains(joined, " --autostart ") {
		t.Fatalf("checked autostart should be passed explicitly: %#v", args)
	}
	if strings.Contains(joined, " --no-autostart ") {
		t.Fatalf("checked autostart should not pass disable flag: %#v", args)
	}
}

func TestInteractiveSelectionArgsDisableDefaultAutostart(t *testing.T) {
	args := interactiveInstallSelection{
		InstallDir:              `C:\Apps\Ariadne`,
		CreateStartMenuShortcut: true,
		CreateDesktopShortcut:   true,
	}.args()

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --no-autostart ") {
		t.Fatalf("unchecked autostart should pass disable flag: %#v", args)
	}
	if strings.Contains(joined, " --autostart ") {
		t.Fatalf("unchecked autostart should not pass enable flag: %#v", args)
	}
}

func TestElevatedInstallArgsPreserveFileSearchServiceChoice(t *testing.T) {
	args := elevatedInstallArgs(commandOptions{
		InstallDir:               `C:\Apps\Ariadne`,
		InstallFileSearchService: true,
		FileSearchSettingsConfig: `C:\Users\luwei\AppData\Roaming\Ariadne\config.json`,
	})

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --install-file-search-service ") {
		t.Fatalf("elevated retry should preserve file search service choice: %#v", args)
	}
	if strings.Contains(joined, " --no-file-search-service ") {
		t.Fatalf("elevated retry should not disable selected file search service: %#v", args)
	}
	if !strings.Contains(joined, ` --settings-config C:\Users\luwei\AppData\Roaming\Ariadne\config.json `) {
		t.Fatalf("elevated retry should preserve settings config path: %#v", args)
	}
}

func TestUpdateSelectionPreservesRecordedInstallOptions(t *testing.T) {
	installDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(installDir, "ariadne.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeReceipt(filepath.Join(installDir, "install_receipt.json"), installReceipt{
		ProductName: "Ariadne",
		InstallDir:  installDir,
		InstallOptions: &receiptInstallOptions{
			CreateStartMenuShortcut:  true,
			CreateDesktopShortcut:    false,
			InstallFileSearchService: false,
			AutoStart:                false,
		},
	}); err != nil {
		t.Fatal(err)
	}

	selection := initialInteractiveSelection(Options{ProductName: "Ariadne", UpdateDir: installDir})

	if selection.InstallDir != installDir || !selection.CreateStartMenuShortcut || selection.CreateDesktopShortcut || selection.InstallFileSearchService || selection.AutoStart {
		t.Fatalf("update selection did not preserve recorded options: %#v", selection)
	}
	if !selection.LaunchAfterInstall {
		t.Fatal("interactive update should launch the refreshed application")
	}
}
