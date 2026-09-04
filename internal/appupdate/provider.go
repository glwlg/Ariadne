package appupdate

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/updater"
	githubprovider "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const updateRepository = "glwlg/Ariadne"

func Configure(client *updater.Updater, currentVersion string, preparations ...func() error) (*Service, error) {
	newService := func(updaterClient *updater.Updater) *Service {
		service := New(currentVersion, updaterClient)
		if len(preparations) > 0 {
			service.prepare = preparations[0]
		}
		return service
	}
	if !updateVersionEnabled(currentVersion) {
		return newService(nil), nil
	}
	if client == nil {
		return newService(nil), fmt.Errorf("Wails updater is unavailable")
	}
	provider, err := NewGitHubProvider(currentVersion)
	if err != nil {
		return newService(nil), err
	}
	if err := client.Init(updater.Config{
		CurrentVersion: strings.TrimPrefix(strings.TrimSpace(currentVersion), "v"),
		Providers:      []updater.Provider{provider},
		Window:         updater.WindowNone,
	}); err != nil {
		return newService(nil), err
	}
	return newService(client), nil
}

func NewGitHubProvider(currentVersion string) (updater.Provider, error) {
	return githubprovider.New(githubprovider.Config{
		Repository:    updateRepository,
		Prerelease:    strings.Contains(strings.TrimPrefix(strings.TrimSpace(currentVersion), "v"), "-"),
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher:  setupAssetMatcher,
	})
}

func setupAssetMatcher(request updater.CheckRequest, assets []githubprovider.ReleaseAsset) int {
	if !strings.EqualFold(strings.TrimSpace(request.Platform), "windows") {
		return -1
	}
	arch := strings.ToLower(strings.TrimSpace(request.Arch))
	if arch != "amd64" && arch != "x64" && arch != "x86_64" {
		return -1
	}
	for index, asset := range assets {
		if isSetupArtifact(asset.Name) {
			return index
		}
	}
	return -1
}
