//go:build windows

package setupstub

import (
	"ariadne/internal/elevation"
	"ariadne/internal/filesearch"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installFileSearchService(productName string, exePath string) error {
	if strings.TrimSpace(exePath) == "" {
		return fmt.Errorf("缺少 Ariadne 程序路径")
	}
	if err := filesearch.InstallWindowsService(productName, exePath); err == nil {
		return nil
	}
	return runElevatedFileSearchServiceCommand("install", exePath)
}

func removeFileSearchService(productName string, exePath string) error {
	if err := filesearch.RemoveWindowsService(); err == nil {
		return nil
	}
	return runElevatedFileSearchServiceCommand("remove", "")
}

func runFileSearchServiceCommand(productName string, command string, exePath string) (Result, error) {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "install":
		if strings.TrimSpace(exePath) == "" {
			return Result{}, fmt.Errorf("缺少 Ariadne 程序路径")
		}
		if err := filesearch.InstallWindowsService(productName, exePath); err != nil {
			return Result{}, err
		}
		return Result{Action: ActionInstall, InstallDir: filepath.Dir(exePath)}, nil
	case "remove":
		if err := filesearch.RemoveWindowsService(); err != nil {
			return Result{}, err
		}
		return Result{Action: ActionUninstall}, nil
	default:
		return Result{}, fmt.Errorf("未知搜索服务命令: %s", command)
	}
}

func runElevatedFileSearchServiceCommand(command string, exePath string) error {
	setupPath, err := os.Executable()
	if err != nil || strings.TrimSpace(setupPath) == "" {
		if err == nil {
			err = fmt.Errorf("缺少安装程序路径")
		}
		return err
	}
	args := []string{"--quiet", "--file-search-service-command", command}
	if strings.TrimSpace(exePath) != "" {
		args = append(args, "--file-search-service-exe", exePath)
	}
	if err := elevation.RunasWait(setupPath, args); err != nil {
		return fmt.Errorf("搜索服务配置未完成: %w", err)
	}
	return nil
}

func runElevatedInstallerInstall(command commandOptions) error {
	setupPath, err := os.Executable()
	if err != nil || strings.TrimSpace(setupPath) == "" {
		if err == nil {
			err = fmt.Errorf("缺少安装程序路径")
		}
		return err
	}
	return elevation.RunasWait(setupPath, elevatedInstallArgs(command))
}

func elevatedInstallArgs(command commandOptions) []string {
	args := []string{"--quiet", "--elevated-install-retry"}
	if command.InstallDir != "" {
		args = append(args, "-InstallDir", command.InstallDir)
	}
	if command.NoShortcuts {
		args = append(args, "-NoShortcuts")
	}
	if !command.CreateStartMenuShortcut {
		args = append(args, "--no-start-menu-shortcut")
	}
	if !command.CreateDesktopShortcut {
		args = append(args, "--no-desktop-shortcut")
	}
	if command.AutoStart {
		args = append(args, "--autostart")
	}
	if command.InstallFileSearchService {
		args = append(args, "--install-file-search-service")
	} else {
		args = append(args, "--no-file-search-service")
	}
	if command.SkipProcessStop {
		args = append(args, "--skip-process-stop")
	}
	if command.SkipServiceChanges {
		args = append(args, "--skip-service-changes")
	}
	if command.StartMenuDir != "" {
		args = append(args, "-StartMenuDir", command.StartMenuDir)
	}
	if command.DesktopDir != "" {
		args = append(args, "-DesktopDir", command.DesktopDir)
	}
	return args
}
