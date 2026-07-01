//go:build windows

package filesearch

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func InstallWindowsService(productName string, exePath string) error {
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

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接 Windows 服务管理器失败: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.CreateService(WindowsServiceName, exePath, mgr.Config{
		DisplayName:      productName + " 搜索服务",
		Description:      productName + " 使用 NTFS USN/MFT 维护本机文件搜索索引。",
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
		ErrorControl:     mgr.ErrorNormal,
	}, "filesearch-service")
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
		return nil
	}
	defer service.Close()
	_ = stopWindowsService(service)
	if err := service.Delete(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return fmt.Errorf("删除搜索服务失败: %w", err)
	}
	return nil
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
