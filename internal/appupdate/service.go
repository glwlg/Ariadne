package appupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

const (
	checkTimeout    = 45 * time.Second
	downloadTimeout = 30 * time.Minute
)

type updaterClient interface {
	Check(context.Context) (*updater.Release, error)
	DownloadAndInstall(context.Context) error
	DownloadedPath() string
	State() updater.State
	CurrentVersion() string
}

type installerLauncher func(path string) error

type Status struct {
	CurrentVersion    string `json:"currentVersion"`
	State             string `json:"state"`
	Enabled           bool   `json:"enabled"`
	CanCheck          bool   `json:"canCheck"`
	CanInstall        bool   `json:"canInstall"`
	UpdateAvailable   bool   `json:"updateAvailable"`
	AvailableVersion  string `json:"availableVersion,omitempty"`
	ReleaseName       string `json:"releaseName,omitempty"`
	ReleaseNotes      string `json:"releaseNotes,omitempty"`
	ArtifactName      string `json:"artifactName,omitempty"`
	DownloadedPath    string `json:"downloadedPath,omitempty"`
	InstallerLaunched bool   `json:"installerLaunched"`
	Message           string `json:"message,omitempty"`
	LastError         string `json:"lastError,omitempty"`
}

type Result struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Status  Status `json:"status"`
}

type Service struct {
	mu                sync.RWMutex
	operationMu       sync.Mutex
	currentVersion    string
	updater           updaterClient
	launch            installerLauncher
	prepare           func() error
	available         *updater.Release
	installerLaunched bool
	message           string
	lastError         string
}

func New(currentVersion string, client *updater.Updater) *Service {
	if client == nil {
		return NewService(currentVersion, nil, launchInstaller)
	}
	return NewService(currentVersion, client, launchInstaller)
}

func NewService(currentVersion string, client updaterClient, launch installerLauncher) *Service {
	return NewServiceWithPreparation(currentVersion, client, launch, nil)
}

func NewServiceWithPreparation(currentVersion string, client updaterClient, launch installerLauncher, prepare func() error) *Service {
	return &Service{
		currentVersion: strings.TrimSpace(currentVersion),
		updater:        client,
		launch:         launch,
		prepare:        prepare,
	}
}

func (s *Service) Status() Status {
	s.mu.RLock()
	currentVersion := s.currentVersion
	client := s.updater
	available := s.available
	installerLaunched := s.installerLaunched
	message := s.message
	lastError := s.lastError
	s.mu.RUnlock()

	state := updater.StateUnconfigured
	if client != nil {
		state = client.State()
		if currentVersion == "" {
			currentVersion = client.CurrentVersion()
		}
	}
	enabled := client != nil && updateVersionEnabled(currentVersion)
	status := Status{
		CurrentVersion:    currentVersion,
		State:             string(state),
		Enabled:           enabled,
		CanCheck:          enabled && !busyState(state),
		InstallerLaunched: installerLaunched,
		Message:           message,
		LastError:         lastError,
	}
	if available != nil {
		status.UpdateAvailable = true
		status.CanInstall = enabled && state == updater.StateAvailable
		status.AvailableVersion = available.Version
		status.ReleaseName = available.Name
		status.ReleaseNotes = available.Notes
		status.ArtifactName = available.Artifact.Filename
	}
	if client != nil {
		status.DownloadedPath = client.DownloadedPath()
	}
	return status
}

func (s *Service) CheckForUpdates() Result {
	if !s.Status().Enabled {
		return s.fail("当前构建未配置可更新的版本号")
	}
	if !s.operationMu.TryLock() {
		return s.fail("已有更新操作正在进行")
	}
	defer s.operationMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	release, err := s.updater.Check(ctx)
	if err != nil {
		return s.rejectCheck("检查更新失败: " + err.Error())
	}
	if release == nil {
		s.mu.Lock()
		s.available = nil
		s.installerLaunched = false
		s.message = "当前已是最新版本"
		s.lastError = ""
		s.mu.Unlock()
		return Result{OK: true, Message: "当前已是最新版本", Status: s.Status()}
	}
	if !isSetupArtifact(release.Artifact.Filename) {
		return s.rejectCheck(fmt.Sprintf("更新版本 %s 没有匹配的 Ariadne 安装器", release.Version))
	}
	if !hasSHA256Verification(release) {
		return s.rejectCheck("更新安装程序缺少 SHA-256 校验信息")
	}

	s.mu.Lock()
	s.available = release
	s.installerLaunched = false
	s.message = "发现新版本 " + release.Version
	s.lastError = ""
	s.mu.Unlock()
	return Result{OK: true, Message: "发现新版本 " + release.Version, Status: s.Status()}
}

func (s *Service) DownloadAndLaunchInstaller() Result {
	status := s.Status()
	if !status.Enabled || !status.UpdateAvailable {
		return s.fail("请先检查并选择可用更新")
	}
	if status.InstallerLaunched {
		return s.fail("更新安装器已经启动")
	}
	if s.launch == nil {
		return s.fail("当前平台无法启动更新安装器")
	}
	if !s.operationMu.TryLock() {
		return s.fail("已有更新操作正在进行")
	}
	defer s.operationMu.Unlock()
	if s.prepare != nil {
		if err := s.prepare(); err != nil {
			return s.fail("创建更新前回滚检查点失败: " + err.Error())
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	if err := s.updater.DownloadAndInstall(ctx); err != nil {
		return s.fail("下载或校验更新失败: " + err.Error())
	}
	setupPath, err := validateStagedInstaller(s.updater.DownloadedPath())
	if err != nil {
		return s.fail("拒绝启动更新安装器: " + err.Error())
	}
	if !strings.EqualFold(filepath.Base(setupPath), filepath.Base(status.ArtifactName)) {
		return s.fail("拒绝启动更新安装器: 下载文件与已检查版本不匹配")
	}
	if err := s.launch(setupPath); err != nil {
		return s.fail("启动更新安装器失败: " + err.Error())
	}

	s.mu.Lock()
	s.installerLaunched = true
	s.message = "更新安装器已启动，请按向导完成安装"
	s.lastError = ""
	s.mu.Unlock()
	return Result{OK: true, Message: "更新安装器已启动，请按向导完成安装", Status: s.Status()}
}

func (s *Service) fail(message string) Result {
	s.mu.Lock()
	s.message = message
	s.lastError = message
	s.mu.Unlock()
	return Result{OK: false, Message: message, Status: s.Status()}
}

func (s *Service) rejectCheck(message string) Result {
	s.mu.Lock()
	s.available = nil
	s.installerLaunched = false
	s.mu.Unlock()
	return s.fail(message)
}

func updateVersionEnabled(version string) bool {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" || strings.EqualFold(version, "dev") || version == "0.0.0-dev" {
		return false
	}
	_, err := semver.StrictNewVersion(version)
	return err == nil
}

func busyState(state updater.State) bool {
	switch state {
	case updater.StateChecking, updater.StateDownloading, updater.StateVerifying, updater.StateInstalling:
		return true
	default:
		return false
	}
}

func isSetupArtifact(name string) bool {
	name = strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	return strings.HasPrefix(name, "ariadnesetup-") && strings.HasSuffix(name, "-windows-x64.exe")
}

func hasSHA256Verification(release *updater.Release) bool {
	if release == nil || release.Verification == nil {
		return false
	}
	return strings.EqualFold(release.Verification.DigestAlgo, "sha256") && len(release.Verification.Digest) == 32
}

func validateStagedInstaller(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("下载路径为空")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !isSetupArtifact(absolute) {
		return "", fmt.Errorf("文件名不是 Ariadne Windows 安装器")
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Size() <= 0 {
		return "", fmt.Errorf("安装器文件无效")
	}
	return absolute, nil
}
