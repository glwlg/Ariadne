package setupstub

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	ActionInstall            = "install"
	ActionUninstall          = "uninstall"
	ActionUninstallScheduled = "uninstall_scheduled"
	ActionCancelled          = "cancelled"
)

type Options struct {
	ProductName string
	Version     string
	Args        []string
	KeepTemp    bool
}

type Result struct {
	Action     string
	InstallDir string
}

type ParsedArgs struct {
	Quiet       bool
	InstallArgs []string
}

type commandOptions struct {
	Uninstall                bool
	Synchronous              bool
	RemoveUserData           bool
	NoShortcuts              bool
	CreateStartMenuShortcut  bool
	CreateDesktopShortcut    bool
	AutoStart                bool
	LaunchAfterInstall       bool
	InstallFileSearchService bool
	SkipProcessStop          bool
	SkipServiceChanges       bool
	ElevatedInstallRetry     bool
	InstallDir               string
	StartMenuDir             string
	DesktopDir               string
	FileSearchServiceCommand string
	FileSearchServiceExePath string
}

type installReceipt struct {
	ProductName   string   `json:"productName"`
	Version       string   `json:"version"`
	InstalledAt   string   `json:"installedAt"`
	InstallDir    string   `json:"installDir"`
	ExePath       string   `json:"exePath"`
	IconPath      string   `json:"iconPath"`
	InstallerPath string   `json:"installerPath"`
	Shortcuts     []string `json:"shortcuts"`
}

func ParseArgs(args []string) ParsedArgs {
	parsed := ParsedArgs{}
	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--quiet", "-quiet", "/quiet", "--silent", "-silent", "/silent":
			parsed.Quiet = true
		default:
			parsed.InstallArgs = append(parsed.InstallArgs, arg)
		}
	}
	return parsed
}

func Run(payload []byte, options Options) (Result, error) {
	if options.ProductName == "" {
		options.ProductName = "Ariadne"
	}
	if options.Version == "" {
		options.Version = "dev"
	}

	command, err := parseCommandOptions(options.Args)
	if err != nil {
		return Result{}, err
	}
	if command.Uninstall {
		return uninstall(options, command)
	}
	if command.FileSearchServiceCommand != "" {
		return runFileSearchServiceCommand(options.ProductName, command.FileSearchServiceCommand, command.FileSearchServiceExePath)
	}
	if len(payload) == 0 {
		return Result{}, errors.New("installer payload is empty")
	}

	tempDir, err := os.MkdirTemp("", "ariadne-setup-*")
	if err != nil {
		return Result{}, fmt.Errorf("create setup temp dir: %w", err)
	}
	if !options.KeepTemp {
		defer os.RemoveAll(tempDir)
	}

	packageRoot, err := ExtractPayload(payload, tempDir)
	if err != nil {
		return Result{}, err
	}
	return install(packageRoot, options, command)
}

func parseCommandOptions(args []string) (commandOptions, error) {
	command := commandOptions{CreateStartMenuShortcut: true, CreateDesktopShortcut: true}
	var ignored bool
	var noStartMenuShortcut bool
	var noDesktopShortcut bool
	var noFileSearchService bool
	flags := flag.NewFlagSet("ariadne-setup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	for _, name := range []string{"uninstall", "Uninstall"} {
		flags.BoolVar(&command.Uninstall, name, false, "")
	}
	for _, name := range []string{"synchronous", "Synchronous"} {
		flags.BoolVar(&command.Synchronous, name, false, "")
	}
	for _, name := range []string{"remove-user-data", "RemoveUserData"} {
		flags.BoolVar(&command.RemoveUserData, name, false, "")
	}
	for _, name := range []string{"no-shortcuts", "NoShortcuts"} {
		flags.BoolVar(&command.NoShortcuts, name, false, "")
	}
	for _, name := range []string{"create-start-menu-shortcut", "CreateStartMenuShortcut"} {
		flags.BoolVar(&command.CreateStartMenuShortcut, name, true, "")
	}
	for _, name := range []string{"no-start-menu-shortcut", "NoStartMenuShortcut"} {
		flags.BoolVar(&noStartMenuShortcut, name, false, "")
	}
	for _, name := range []string{"create-desktop-shortcut", "CreateDesktopShortcut"} {
		flags.BoolVar(&command.CreateDesktopShortcut, name, true, "")
	}
	for _, name := range []string{"no-desktop-shortcut", "NoDesktopShortcut"} {
		flags.BoolVar(&noDesktopShortcut, name, false, "")
	}
	for _, name := range []string{"autostart", "AutoStart"} {
		flags.BoolVar(&command.AutoStart, name, false, "")
	}
	for _, name := range []string{"launch-after-install", "LaunchAfterInstall"} {
		flags.BoolVar(&command.LaunchAfterInstall, name, false, "")
	}
	for _, name := range []string{"install-file-search-service", "InstallFileSearchService"} {
		flags.BoolVar(&command.InstallFileSearchService, name, false, "")
	}
	for _, name := range []string{"no-file-search-service", "NoFileSearchService"} {
		flags.BoolVar(&noFileSearchService, name, false, "")
	}
	for _, name := range []string{"skip-process-stop", "SkipProcessStop"} {
		flags.BoolVar(&command.SkipProcessStop, name, false, "")
	}
	for _, name := range []string{"skip-service-changes", "SkipServiceChanges"} {
		flags.BoolVar(&command.SkipServiceChanges, name, false, "")
	}
	for _, name := range []string{"elevated-install-retry", "ElevatedInstallRetry"} {
		flags.BoolVar(&command.ElevatedInstallRetry, name, false, "")
	}
	for _, name := range []string{"install-dir", "InstallDir"} {
		flags.StringVar(&command.InstallDir, name, "", "")
	}
	for _, name := range []string{"start-menu-dir", "StartMenuDir"} {
		flags.StringVar(&command.StartMenuDir, name, "", "")
	}
	for _, name := range []string{"desktop-dir", "DesktopDir"} {
		flags.StringVar(&command.DesktopDir, name, "", "")
	}
	for _, name := range []string{"file-search-service-command", "FileSearchServiceCommand"} {
		flags.StringVar(&command.FileSearchServiceCommand, name, "", "")
	}
	for _, name := range []string{"file-search-service-exe", "FileSearchServiceExe"} {
		flags.StringVar(&command.FileSearchServiceExePath, name, "", "")
	}
	for _, name := range []string{"Force", "force", "SkipLegacyCheck", "skip-legacy-check"} {
		flags.BoolVar(&ignored, name, false, "")
	}
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, err
	}
	if flags.NArg() > 0 {
		return commandOptions{}, fmt.Errorf("unexpected installer arguments: %s", strings.Join(flags.Args(), " "))
	}
	if noStartMenuShortcut {
		command.CreateStartMenuShortcut = false
	}
	if noDesktopShortcut {
		command.CreateDesktopShortcut = false
	}
	if noFileSearchService {
		command.InstallFileSearchService = false
	}
	return command, nil
}

func install(packageRoot string, options Options, command commandOptions) (Result, error) {
	appSource := filepath.Join(packageRoot, "app")
	if info, err := os.Stat(filepath.Join(appSource, strings.ToLower(options.ProductName)+".exe")); err != nil || info.IsDir() {
		return Result{}, fmt.Errorf("installer payload is missing app/%s.exe", strings.ToLower(options.ProductName))
	}

	installDir := firstNonEmpty(command.InstallDir, defaultInstallDir(options.ProductName))
	if !command.SkipServiceChanges {
		if err := removeFileSearchService(options.ProductName, filepath.Join(installDir, strings.ToLower(options.ProductName)+".exe")); err != nil {
			return Result{}, err
		}
	}
	if !command.SkipProcessStop {
		stopProcess(strings.ToLower(options.ProductName) + ".exe")
	}
	if err := rotateExistingInstall(installDir, options.ProductName); err != nil {
		if shouldRetryInstallElevated(command, err) {
			return retryInstallElevated(options, command, installDir)
		}
		return Result{}, err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := copyDir(appSource, installDir); err != nil {
		return Result{}, err
	}

	installerPath := filepath.Join(installDir, options.ProductName+"Setup.exe")
	if err := copyCurrentExecutable(installerPath); err != nil {
		return Result{}, err
	}

	exePath := filepath.Join(installDir, strings.ToLower(options.ProductName)+".exe")
	iconPath := filepath.Join(installDir, "logo.ico")
	shortcuts := []string{}
	if !command.NoShortcuts {
		if command.CreateStartMenuShortcut {
			startMenuDir := firstNonEmpty(command.StartMenuDir, defaultStartMenuDir())
			if startMenuDir != "" {
				startShortcut := filepath.Join(startMenuDir, options.ProductName+".lnk")
				if err := createShortcut(exePath, startShortcut, iconPath, ""); err != nil {
					return Result{}, err
				}
				shortcuts = append(shortcuts, startShortcut)

				uninstallShortcut := filepath.Join(startMenuDir, "Uninstall "+options.ProductName+".lnk")
				if err := createShortcut(installerPath, uninstallShortcut, iconPath, "--uninstall"); err != nil {
					return Result{}, err
				}
				shortcuts = append(shortcuts, uninstallShortcut)
			}
		}
		if command.CreateDesktopShortcut {
			desktopDir := firstNonEmpty(command.DesktopDir, defaultDesktopDir())
			if desktopDir != "" {
				desktopShortcut := filepath.Join(desktopDir, options.ProductName+".lnk")
				if err := createShortcut(exePath, desktopShortcut, iconPath, ""); err != nil {
					return Result{}, err
				}
				shortcuts = append(shortcuts, desktopShortcut)
			}
		}
		refreshShell()
	}

	receipt := installReceipt{
		ProductName:   options.ProductName,
		Version:       options.Version,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339Nano),
		InstallDir:    installDir,
		ExePath:       exePath,
		IconPath:      iconPath,
		InstallerPath: installerPath,
		Shortcuts:     shortcuts,
	}
	if err := writeReceipt(filepath.Join(installDir, "install_receipt.json"), receipt); err != nil {
		return Result{}, err
	}
	if command.AutoStart {
		if err := setAutostart(options.ProductName, exePath); err != nil {
			return Result{}, err
		}
	} else {
		cleanupAutostart(options.ProductName)
	}
	if command.InstallFileSearchService && !command.SkipServiceChanges {
		if err := installFileSearchService(options.ProductName, exePath); err != nil {
			return Result{}, err
		}
	}
	if command.LaunchAfterInstall {
		_ = launchInstalledApp(exePath)
	}
	return Result{Action: ActionInstall, InstallDir: installDir}, nil
}

func uninstall(options Options, command commandOptions) (Result, error) {
	installDir := firstNonEmpty(command.InstallDir, detectInstallDir(options.ProductName))
	if installDir == "" {
		installDir = defaultInstallDir(options.ProductName)
	}
	if executableInside(installDir) && !command.Synchronous {
		if err := scheduleUninstallFromTemp(options, command, installDir); err != nil {
			return Result{}, err
		}
		return Result{Action: ActionUninstallScheduled, InstallDir: installDir}, nil
	}

	receipt := readReceipt(filepath.Join(installDir, "install_receipt.json"))
	if !command.SkipServiceChanges {
		if err := removeFileSearchService(options.ProductName, firstNonEmpty(receipt.ExePath, filepath.Join(installDir, strings.ToLower(options.ProductName)+".exe"))); err != nil {
			return Result{}, err
		}
	}
	if !command.SkipProcessStop {
		stopProcess(strings.ToLower(options.ProductName) + ".exe")
	}
	removeShortcuts(options.ProductName, receipt, command)
	cleanupAutostart(options.ProductName)
	if command.RemoveUserData {
		removeUserData(options.ProductName)
	}
	if err := os.RemoveAll(installDir); err != nil {
		return Result{}, err
	}
	refreshShell()
	return Result{Action: ActionUninstall, InstallDir: installDir}, nil
}

func ExtractPayload(payload []byte, targetDir string) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return "", fmt.Errorf("open installer payload: %w", err)
	}

	cleanTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return "", err
	}

	var appExe string
	for _, item := range reader.File {
		entryName, err := cleanZipEntryName(item.Name)
		if err != nil {
			return "", err
		}
		if item.FileInfo().IsDir() {
			continue
		}

		targetPath := filepath.Join(cleanTargetDir, entryName)
		if !sameOrInside(targetPath, cleanTargetDir) {
			return "", fmt.Errorf("refusing to extract zip entry outside target: %s", item.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", err
		}
		if err := extractZipFile(item, targetPath); err != nil {
			return "", err
		}

		parts := strings.Split(filepath.ToSlash(entryName), "/")
		if len(parts) >= 3 && strings.EqualFold(parts[len(parts)-2], "app") && strings.EqualFold(parts[len(parts)-1], "ariadne.exe") {
			appExe = targetPath
		}
	}

	if appExe == "" {
		return "", errors.New("installer payload is missing app/ariadne.exe")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(appExe), "..")), nil
}

func cleanZipEntryName(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if path.IsAbs(normalized) {
		return "", fmt.Errorf("refusing absolute zip entry: %s", name)
	}
	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("refusing unsafe zip entry: %s", name)
	}

	native := filepath.Clean(filepath.FromSlash(clean))
	if filepath.IsAbs(native) || filepath.VolumeName(native) != "" || native == ".." || strings.HasPrefix(native, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing unsafe zip entry: %s", name)
	}
	return native, nil
}

func extractZipFile(item *zip.File, targetPath string) error {
	source, err := item.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	mode := item.Mode()
	if mode == 0 {
		mode = 0o644
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func rotateExistingInstall(installDir string, productName string) error {
	info, err := os.Stat(installDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("install path exists and is not a directory: %s", installDir)
	}
	parent := filepath.Dir(installDir)
	stamp := time.Now().Format("20060102-150405")
	for index := 0; index < 20; index++ {
		name := productName + ".previous-" + stamp
		if index > 0 {
			name = fmt.Sprintf("%s.previous-%s-%02d", productName, stamp, index)
		}
		target := filepath.Join(parent, name)
		if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
			return os.Rename(installDir, target)
		}
	}
	return fmt.Errorf("could not choose previous install directory for %s", installDir)
}

func shouldRetryInstallElevated(command commandOptions, err error) bool {
	if command.ElevatedInstallRetry || err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "access is denied") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "being used by another process") ||
		strings.Contains(message, "另一个程序正在使用") ||
		strings.Contains(message, "拒绝访问")
}

func retryInstallElevated(options Options, command commandOptions, installDir string) (Result, error) {
	elevated := command
	elevated.ElevatedInstallRetry = true
	elevated.LaunchAfterInstall = false
	if err := runElevatedInstallerInstall(elevated); err != nil {
		return Result{}, fmt.Errorf("安装程序需要关闭正在运行的 %s: %w", options.ProductName, err)
	}
	if command.LaunchAfterInstall {
		exePath := filepath.Join(installDir, strings.ToLower(options.ProductName)+".exe")
		_ = launchInstalledApp(exePath)
	}
	return Result{Action: ActionInstall, InstallDir: installDir}, nil
}

func copyDir(source string, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFile(path, targetPath)
	})
}

func copyCurrentExecutable(target string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return copyFile(exe, target)
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func writeReceipt(path string, receipt installReceipt) error {
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func readReceipt(path string) installReceipt {
	raw, err := os.ReadFile(path)
	if err != nil {
		return installReceipt{}
	}
	var receipt installReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return installReceipt{}
	}
	return receipt
}

func removeShortcuts(productName string, receipt installReceipt, command commandOptions) {
	seen := map[string]bool{}
	for _, shortcut := range receipt.Shortcuts {
		removeShortcut(shortcut, seen)
	}
	startMenuDir := firstNonEmpty(command.StartMenuDir, defaultStartMenuDir())
	if startMenuDir != "" {
		removeShortcut(filepath.Join(startMenuDir, productName+".lnk"), seen)
		removeShortcut(filepath.Join(startMenuDir, "Uninstall "+productName+".lnk"), seen)
	}
	desktopDir := firstNonEmpty(command.DesktopDir, defaultDesktopDir())
	if desktopDir != "" {
		removeShortcut(filepath.Join(desktopDir, productName+".lnk"), seen)
	}
}

func removeShortcut(path string, seen map[string]bool) {
	if path == "" {
		return
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		clean = path
	}
	key := strings.ToLower(clean)
	if seen[key] {
		return
	}
	seen[key] = true
	_ = os.Remove(path)
}

func scheduleUninstallFromTemp(options Options, command commandOptions, installDir string) error {
	tempExe := filepath.Join(os.TempDir(), fmt.Sprintf("%sUninstall-%d.exe", options.ProductName, time.Now().UnixNano()))
	if err := copyCurrentExecutable(tempExe); err != nil {
		return err
	}
	args := []string{"--uninstall", "--synchronous", "--quiet", "-InstallDir", installDir}
	if command.RemoveUserData {
		args = append(args, "--remove-user-data")
	}
	if command.SkipProcessStop {
		args = append(args, "--skip-process-stop")
	}
	if command.StartMenuDir != "" {
		args = append(args, "-StartMenuDir", command.StartMenuDir)
	}
	if command.DesktopDir != "" {
		args = append(args, "-DesktopDir", command.DesktopDir)
	}
	cmd := exec.Command(tempExe, args...)
	configureCommand(cmd)
	return cmd.Start()
}

func executableInside(root string) bool {
	exe, err := os.Executable()
	if err != nil || root == "" {
		return false
	}
	return sameOrInside(exe, root)
}

func detectInstallDir(productName string) string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, statErr := os.Stat(filepath.Join(dir, strings.ToLower(productName)+".exe")); statErr == nil {
			return dir
		}
	}
	return defaultInstallDir(productName)
}

func defaultInstallDir(productName string) string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		return filepath.Join(os.TempDir(), productName)
	}
	return filepath.Join(base, "Programs", productName)
}

func defaultStartMenuDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
}

func defaultDesktopDir() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		return ""
	}
	return filepath.Join(profile, "Desktop")
}

func removeUserData(productName string) {
	if appData := os.Getenv("APPDATA"); appData != "" {
		_ = os.RemoveAll(filepath.Join(appData, productName))
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		_ = os.RemoveAll(filepath.Join(localAppData, productName))
	}
}

func stopProcess(imageName string) {
	cmd := exec.Command("taskkill.exe", "/IM", imageName, "/F")
	configureCommand(cmd)
	_ = cmd.Run()
	time.Sleep(300 * time.Millisecond)
}

func launchInstalledApp(exePath string) error {
	cmd := exec.Command(exePath)
	cmd.Dir = filepath.Dir(exePath)
	return cmd.Start()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sameOrInside(path string, root string) bool {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if strings.EqualFold(cleanPath, cleanRoot) {
		return true
	}
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
