//go:build windows

package setupstub

import (
	"strings"
	"testing"
)

func TestInteractiveSelectionArgsEnableFileSearchService(t *testing.T) {
	args := interactiveInstallSelection{
		InstallDir:               `C:\Apps\Ariadne`,
		CreateStartMenuShortcut:  true,
		CreateDesktopShortcut:    true,
		InstallFileSearchService: true,
	}.args()

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --install-file-search-service ") {
		t.Fatalf("checked file search service should be passed explicitly: %#v", args)
	}
	if strings.Contains(joined, " --no-file-search-service ") {
		t.Fatalf("checked file search service should not pass disable flag: %#v", args)
	}
}

func TestElevatedInstallArgsPreserveFileSearchServiceChoice(t *testing.T) {
	args := elevatedInstallArgs(commandOptions{
		InstallDir:               `C:\Apps\Ariadne`,
		InstallFileSearchService: true,
	})

	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, " --install-file-search-service ") {
		t.Fatalf("elevated retry should preserve file search service choice: %#v", args)
	}
	if strings.Contains(joined, " --no-file-search-service ") {
		t.Fatalf("elevated retry should not disable selected file search service: %#v", args)
	}
}
