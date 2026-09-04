package appupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	githubprovider "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type fakeUpdater struct {
	current     string
	state       updater.State
	release     *updater.Release
	checkErr    error
	downloaded  string
	downloadErr error
	downloads   int
}

func (f *fakeUpdater) Check(context.Context) (*updater.Release, error) {
	if f.checkErr != nil {
		return nil, f.checkErr
	}
	if f.release == nil {
		f.state = updater.StateUpToDate
		return nil, nil
	}
	f.state = updater.StateAvailable
	return f.release, nil
}

func (f *fakeUpdater) DownloadAndInstall(context.Context) error {
	f.downloads++
	if f.downloadErr != nil {
		return f.downloadErr
	}
	f.state = updater.StateReady
	return nil
}

func (f *fakeUpdater) DownloadedPath() string { return f.downloaded }
func (f *fakeUpdater) State() updater.State   { return f.state }
func (f *fakeUpdater) CurrentVersion() string { return f.current }

type fakeUpdaterHost struct{}

func (fakeUpdaterHost) Emit(string, ...any) bool                              { return true }
func (fakeUpdaterHost) OnEvent(string, func(any)) func()                      { return func() {} }
func (fakeUpdaterHost) OpenWindow(updater.WindowOptions) updater.WindowHandle { return nil }
func (fakeUpdaterHost) Quit()                                                 {}

func TestCheckForUpdatesExposesInstallerRelease(t *testing.T) {
	client := &fakeUpdater{
		current: "1.0.0",
		state:   updater.StateIdle,
		release: &updater.Release{
			Version:      "1.1.0",
			Name:         "Ariadne 1.1.0",
			Notes:        "修复与改进",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact: updater.Artifact{
				Filename: "AriadneSetup-1.1.0-windows-x64.exe",
				Filetype: "exe",
			},
		},
	}
	service := NewService("1.0.0", client, nil)

	result := service.CheckForUpdates()

	if !result.OK || !result.Status.UpdateAvailable || result.Status.AvailableVersion != "1.1.0" {
		t.Fatalf("expected available installer update, got %#v", result)
	}
	if !result.Status.CanInstall || result.Status.ReleaseNotes != "修复与改进" {
		t.Fatalf("expected installable release details, got %#v", result.Status)
	}
}

func TestCheckRejectsInstallerWithoutSHA256Verification(t *testing.T) {
	client := &fakeUpdater{
		current: "1.0.0",
		state:   updater.StateIdle,
		release: &updater.Release{
			Version:  "1.1.0",
			Artifact: updater.Artifact{Filename: "AriadneSetup-1.1.0-windows-x64.exe", Filetype: "exe"},
		},
	}

	result := NewService("1.0.0", client, nil).CheckForUpdates()

	if result.OK || result.Status.UpdateAvailable {
		t.Fatalf("unverified installer must be rejected, got %#v", result)
	}
}

func TestRejectedCheckClearsPreviouslyAvailableRelease(t *testing.T) {
	client := &fakeUpdater{
		current: "1.0.0",
		state:   updater.StateIdle,
		release: &updater.Release{
			Version:      "1.1.0",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact:     updater.Artifact{Filename: "AriadneSetup-1.1.0-windows-x64.exe", Filetype: "exe"},
		},
	}
	service := NewService("1.0.0", client, nil)
	if result := service.CheckForUpdates(); !result.OK || !result.Status.UpdateAvailable {
		t.Fatalf("initial verified check failed: %#v", result)
	}
	client.release = &updater.Release{
		Version:  "1.2.0",
		Artifact: updater.Artifact{Filename: "AriadneSetup-1.2.0-windows-x64.exe", Filetype: "exe"},
	}
	client.state = updater.StateIdle

	result := service.CheckForUpdates()

	if result.OK || result.Status.UpdateAvailable || result.Status.CanInstall {
		t.Fatalf("rejected check retained a stale release: %#v", result)
	}
}

func TestDownloadAndLaunchInstallerUsesVerifiedStagedSetup(t *testing.T) {
	setupPath := filepath.Join(t.TempDir(), "AriadneSetup-1.1.0-windows-x64.exe")
	if err := os.WriteFile(setupPath, []byte("setup"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeUpdater{
		current:    "1.0.0",
		state:      updater.StateIdle,
		downloaded: setupPath,
		release: &updater.Release{
			Version:      "1.1.0",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact:     updater.Artifact{Filename: filepath.Base(setupPath), Filetype: "exe"},
		},
	}
	launched := ""
	service := NewService("1.0.0", client, func(path string) error {
		launched = path
		return nil
	})
	if result := service.CheckForUpdates(); !result.OK {
		t.Fatalf("check update: %#v", result)
	}

	result := service.DownloadAndLaunchInstaller()

	if !result.OK || launched != setupPath {
		t.Fatalf("expected staged setup to launch, got result=%#v launched=%q", result, launched)
	}
	if !result.Status.InstallerLaunched || result.Status.CanInstall {
		t.Fatalf("expected launched status to disable duplicate installs, got %#v", result.Status)
	}
}

func TestSetupAssetMatcherSelectsOnlyWindowsInstaller(t *testing.T) {
	assets := []githubprovider.ReleaseAsset{
		{Name: "ariadne-1.1.0-windows-x64.zip"},
		{Name: "SHA256SUMS"},
		{Name: "AriadneSetup-1.1.0-linux-amd64.exe"},
		{Name: "AriadneSetup-1.1.0-windows-x64.exe"},
	}

	index := setupAssetMatcher(updater.CheckRequest{Platform: "windows", Arch: "amd64"}, assets)

	if index != 3 {
		t.Fatalf("expected Windows setup asset index 3, got %d", index)
	}
	if got := setupAssetMatcher(updater.CheckRequest{Platform: "linux", Arch: "amd64"}, assets); got != -1 {
		t.Fatalf("expected unsupported platform to have no installer, got %d", got)
	}
}

func TestUpdateVersionRequiresSemanticVersion(t *testing.T) {
	for _, version := range []string{"", "dev", "0.0.0-dev", "20260904-color-fix", "not-a-version"} {
		if updateVersionEnabled(version) {
			t.Fatalf("expected %q to disable update checks", version)
		}
	}
	for _, version := range []string{"1.0.0", "v1.2.3", "0.1.0-beta.1"} {
		if !updateVersionEnabled(version) {
			t.Fatalf("expected %q to enable update checks", version)
		}
	}
}

func TestDownloadRejectsUnexpectedStagedExecutable(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "untrusted.exe")
	if err := os.WriteFile(badPath, []byte("not an installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeUpdater{
		current:    "1.0.0",
		state:      updater.StateIdle,
		downloaded: badPath,
		release: &updater.Release{
			Version:      "1.1.0",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact:     updater.Artifact{Filename: "AriadneSetup-1.1.0-windows-x64.exe", Filetype: "exe"},
		},
	}
	launched := false
	service := NewService("1.0.0", client, func(string) error {
		launched = true
		return nil
	})
	if result := service.CheckForUpdates(); !result.OK {
		t.Fatalf("check update: %#v", result)
	}

	result := service.DownloadAndLaunchInstaller()

	if result.OK || launched {
		t.Fatalf("unexpected executable must not launch: result=%#v launched=%t", result, launched)
	}
}

func TestDownloadRejectsInstallerThatDoesNotMatchCheckedRelease(t *testing.T) {
	stagedPath := filepath.Join(t.TempDir(), "AriadneSetup-1.2.0-windows-x64.exe")
	if err := os.WriteFile(stagedPath, []byte("setup"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeUpdater{
		current:    "1.0.0",
		state:      updater.StateIdle,
		downloaded: stagedPath,
		release: &updater.Release{
			Version:      "1.1.0",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact:     updater.Artifact{Filename: "AriadneSetup-1.1.0-windows-x64.exe", Filetype: "exe"},
		},
	}
	launched := false
	service := NewService("1.0.0", client, func(string) error {
		launched = true
		return nil
	})
	if result := service.CheckForUpdates(); !result.OK {
		t.Fatalf("check update: %#v", result)
	}

	result := service.DownloadAndLaunchInstaller()

	if result.OK || launched {
		t.Fatalf("mismatched installer must not launch: result=%#v launched=%t", result, launched)
	}
}

func TestConfigureKeepsDevelopmentBuildOffline(t *testing.T) {
	service, err := Configure(nil, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if status := service.Status(); status.Enabled || status.CanCheck {
		t.Fatalf("development build must not contact update provider, got %#v", status)
	}
}

func TestConfigureInitializesWailsUpdaterForSemanticVersion(t *testing.T) {
	client := updater.New(fakeUpdaterHost{})
	service, err := Configure(client, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	status := service.Status()
	if !status.Enabled || !status.CanCheck || status.State != string(updater.StateIdle) {
		t.Fatalf("expected configured Wails updater, got %#v", status)
	}
}

func TestConcurrentUpdateOperationIsRejected(t *testing.T) {
	client := &fakeUpdater{current: "1.0.0", state: updater.StateIdle}
	service := NewService("1.0.0", client, nil)
	service.operationMu.Lock()
	defer service.operationMu.Unlock()

	result := service.CheckForUpdates()

	if result.OK || result.Status.LastError == "" {
		t.Fatalf("concurrent updater operation should be rejected: %#v", result)
	}
}

func TestInstallStopsWhenPreUpdateCheckpointFails(t *testing.T) {
	client := &fakeUpdater{
		current: "1.0.0",
		state:   updater.StateIdle,
		release: &updater.Release{
			Version:      "1.1.0",
			Verification: &updater.Verification{DigestAlgo: "sha256", Digest: make([]byte, 32)},
			Artifact:     updater.Artifact{Filename: "AriadneSetup-1.1.0-windows-x64.exe", Filetype: "exe"},
		},
	}
	service := NewServiceWithPreparation("1.0.0", client, func(string) error {
		return nil
	}, func() error {
		return os.ErrPermission
	})
	if result := service.CheckForUpdates(); !result.OK {
		t.Fatalf("check update: %#v", result)
	}

	result := service.DownloadAndLaunchInstaller()

	if result.OK || client.downloads != 0 {
		t.Fatalf("checkpoint failure must stop before download, got result=%#v downloads=%d", result, client.downloads)
	}
}
