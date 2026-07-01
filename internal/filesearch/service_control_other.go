//go:build !windows

package filesearch

import "fmt"

func InstallWindowsService(productName string, exePath string) error {
	return fmt.Errorf("Ariadne 搜索服务仅支持 Windows")
}

func RemoveWindowsService() error {
	return fmt.Errorf("Ariadne 搜索服务仅支持 Windows")
}
