//go:build windows

package filesearch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func InstallWindowsService(productName string, exePath string, serviceArgs ...string) error {
	productName = strings.TrimSpace(productName)
	if productName == "" {
		productName = "Ariadne"
	}
	if strings.TrimSpace(exePath) == "" {
		return fmt.Errorf("缺少 Ariadne 程序路径")
	}
	if err := RemoveWindowsService(); err != nil {
		return err
	}
	serviceExePath, err := installProtectedServiceExecutable(exePath)
	if err != nil {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接 Windows 服务管理器失败: %w", err)
	}
	defer manager.Disconnect()

	args := append([]string{"filesearch-service"}, serviceArgs...)
	service, err := manager.CreateService(WindowsServiceName, serviceExePath, mgr.Config{
		DisplayName:      productName + " 后台服务",
		Description:      productName + " 提供文件索引与进程网络统计。",
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
		ErrorControl:     mgr.ErrorNormal,
	}, args...)
	if err != nil && errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		if waitErr := waitWindowsServiceDeleted(manager, WindowsServiceName, 10*time.Second); waitErr != nil {
			return waitErr
		}
		service, err = manager.CreateService(WindowsServiceName, serviceExePath, mgr.Config{
			DisplayName:      productName + " 后台服务",
			Description:      productName + " 提供文件索引与进程网络统计。",
			StartType:        mgr.StartAutomatic,
			DelayedAutoStart: true,
			ErrorControl:     mgr.ErrorNormal,
		}, args...)
	}
	if err != nil {
		return fmt.Errorf("安装搜索服务失败: %w", err)
	}
	defer service.Close()
	if err := service.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("启动搜索服务失败: %w", err)
	}
	return nil
}

func RemoveWindowsService() error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接 Windows 服务管理器失败: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(WindowsServiceName)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return err
		}
		return removeProtectedServiceExecutable()
	}
	_ = stopWindowsService(service)
	deleteErr := service.Delete()
	closeErr := service.Close()
	if deleteErr != nil && !errors.Is(deleteErr, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return fmt.Errorf("删除搜索服务失败: %w", deleteErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭搜索服务句柄失败: %w", closeErr)
	}
	if err := waitWindowsServiceDeleted(manager, WindowsServiceName, 10*time.Second); err != nil {
		return fmt.Errorf("删除搜索服务失败: %w", err)
	}
	return removeProtectedServiceExecutable()
}

func installProtectedServiceExecutable(source string) (string, error) {
	target := protectedServiceExecutablePath()
	if target == "" {
		return "", errors.New("无法确定后台服务安装目录")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("创建后台服务目录失败: %w", err)
	}
	input, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("读取后台服务程序失败: %w", err)
	}
	defer input.Close()
	temporary := target + ".new"
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("写入后台服务程序失败: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return "", closeErr
	}
	_ = os.Remove(target)
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("更新后台服务程序失败: %w", err)
	}
	return target, nil
}

func removeProtectedServiceExecutable() error {
	directory := filepath.Dir(protectedServiceExecutablePath())
	if directory == "." || directory == "" {
		return nil
	}
	if err := os.RemoveAll(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除后台服务程序失败: %w", err)
	}
	return nil
}

func protectedServiceExecutablePath() string {
	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if programFiles == "" {
		return ""
	}
	return filepath.Join(programFiles, "Ariadne", "Service", "Ariadne.Service.exe")
}

func stopWindowsService(service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}
	if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err = service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("停止搜索服务超时")
}

func waitWindowsServiceDeleted(manager *mgr.Mgr, serviceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return nil
		}
		_ = service.Close()
		if time.Now().After(deadline) {
			return fmt.Errorf("等待搜索服务删除超时")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
