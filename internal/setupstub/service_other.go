//go:build !windows

package setupstub

import "fmt"

func installFileSearchService(productName string, exePath string) error {
	return nil
}

func removeFileSearchService(productName string, exePath string) error {
	return nil
}

func runFileSearchServiceCommand(productName string, command string, exePath string) (Result, error) {
	return Result{}, fmt.Errorf("搜索服务仅支持 Windows")
}

func runElevatedInstallerInstall(command commandOptions) error {
	return fmt.Errorf("安装提权仅支持 Windows")
}
