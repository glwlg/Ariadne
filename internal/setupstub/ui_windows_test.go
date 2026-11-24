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
		InstallDir:               `C:\Apps\Fancun`,
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
		InstallDir:            `C:\Apps\Fancun`,
		CreateDesktopShortcut: true,
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
		InstallDir:               `C:\Apps\Fancun`,
		InstallFileSearchService: true,
		FileSearchSettingsConfig: `C:\Users\luwei\AppData\Roaming\Fancun\config.json`,
		LegacyRoamingSource:      `C:\Users\luwei\AppData\Roaming\Ariadne`,
	})

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --install-file-search-service ") {
		t.Fatalf("elevated retry should preserve file search service choice: %#v", args)
	}
	if strings.Contains(joined, " --no-file-search-service ") {
		t.Fatalf("elevated retry should not disable selected file search service: %#v", args)
	}
	if !strings.Contains(joined, ` --settings-config C:\Users\luwei\AppData\Roaming\Fancun\config.json `) {
		t.Fatalf("elevated retry should preserve settings config path: %#v", args)
	}
	if !strings.Contains(joined, ` --legacy-roaming-source C:\Users\luwei\AppData\Roaming\Ariadne `) {
		t.Fatalf("elevated retry should preserve selected legacy profile: %#v", args)
	}
	argsWithBases := elevatedInstallArgs(commandOptions{
		InstallDir:         `C:\Apps\Fancun`,
		RoamingBase:        `C:\Users\luwei\AppData\Roaming`,
		LocalBase:          `C:\Users\luwei\AppData\Local`,
		PicturesBase:       `C:\Users\luwei\Pictures`,
		LaunchAfterInstall: false,
	})
	joinedWithBases := " " + strings.Join(argsWithBases, " ") + " "
	for _, want := range []string{
		` --roaming-base C:\Users\luwei\AppData\Roaming `,
		` --local-base C:\Users\luwei\AppData\Local `,
		` --pictures-base C:\Users\luwei\Pictures `,
	} {
		if !strings.Contains(joinedWithBases, want) {
			t.Fatalf("elevated args missing %q: %#v", want, argsWithBases)
		}
	}
	if strings.Contains(joinedWithBases, " --launch-after-install ") {
		t.Fatalf("elevated helper args must not include --launch-after-install: %#v", argsWithBases)
	}
}

func TestUpdateSelectionPreservesRecordedInstallOptions(t *testing.T) {
	installDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(installDir, "fancun.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeReceipt(filepath.Join(installDir, "install_receipt.json"), installReceipt{
		ProductName: "Fancun",
		InstallDir:  installDir,
		InstallOptions: &receiptInstallOptions{
			CreateStartMenuShortcut:  false,
			CreateDesktopShortcut:    false,
			InstallFileSearchService: false,
			AutoStart:                false,
		},
	}); err != nil {
		t.Fatal(err)
	}

	selection := initialInteractiveSelection(Options{ProductName: "Fancun", UpdateDir: installDir})

	if selection.InstallDir != installDir || selection.CreateDesktopShortcut || selection.InstallFileSearchService || selection.AutoStart {
		t.Fatalf("update selection did not preserve recorded options: %#v", selection)
	}
	if _, err := parseCommandOptions(selection.args()); err != nil {
		t.Fatalf("an old disabled Start Menu choice must not disable it on upgrade: %v", err)
	}
	if !selection.LaunchAfterInstall {
		t.Fatal("interactive update should launch the refreshed application")
	}
}
